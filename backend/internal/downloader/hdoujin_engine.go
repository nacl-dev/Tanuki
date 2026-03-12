package downloader

import (
	"archive/zip"
	"context"
	"fmt"
	htmlstd "html"
	"io"
	"net/http"
	urlpkg "net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/antchfx/htmlquery"
	lua "github.com/yuin/gopher-lua"
	"golang.org/x/net/html"

	"github.com/nacl-dev/tanuki/internal/models"
	"go.uber.org/zap"
)

type HDoujinLuaEngine struct {
	modulesDir    string
	client        *http.Client
	log           *zap.Logger
	progress      func(id string, downloaded, total int64, files, totalFiles int)
	modulesByHost map[string][]*hdoujinLuaModule
	enabled       bool
}

type hdoujinLuaModule struct {
	Path                  string
	FileName              string
	Name                  string
	Language              string
	Type                  string
	Adult                 bool
	Strict                bool
	Domains               []string
	HasGetInfo            bool
	HasGetPages           bool
	HasGetChapters        bool
	HasBeforeDownloadPage bool
	Body                  string
}

type hdoujinChapter struct {
	URL   string
	Title string
}

type hdoujinRuntime struct {
	L           *lua.LState
	module      *hdoujinLuaModule
	client      *http.Client
	log         *zap.Logger
	currentURL  string
	currentBody string
	currentDoc  *html.Node
	info        map[string]lua.LValue
	pages       []string
	chapters    []hdoujinChapter
	pagesTbl    *lua.LTable
	chaptersTbl *lua.LTable
	matchedHost string
}

type hdoujinDOMNode struct {
	runtime *hdoujinRuntime
	node    *html.Node
}

var hdoujinAttrSuffixRe = regexp.MustCompile(`^(.*?)/@([A-Za-z_:][A-Za-z0-9_:\-\.]*)$`)

// NewHDoujinLuaEngine creates a compatibility engine for local HDoujinDownloader Lua modules.
func NewHDoujinLuaEngine(modulesDir, cookiesPath string, log *zap.Logger) *HDoujinLuaEngine {
	client, err := newHTTPClientWithCookies(cookiesPath)
	if err != nil {
		if log != nil {
			log.Warn("hdoujin: cookies unavailable", zap.String("path", cookiesPath), zap.Error(err))
		}
		client = &http.Client{}
	}

	engine := &HDoujinLuaEngine{
		modulesDir:    strings.TrimSpace(modulesDir),
		client:        client,
		log:           log,
		modulesByHost: map[string][]*hdoujinLuaModule{},
	}
	engine.loadModules()
	return engine
}

func (e *HDoujinLuaEngine) SetProgressUpdater(fn func(id string, downloaded, total int64, files, totalFiles int)) {
	e.progress = fn
}

func (e *HDoujinLuaEngine) CanHandle(rawURL string) bool {
	if !e.enabled {
		return false
	}
	return len(e.matchingModules(rawURL)) > 0
}

func (e *HDoujinLuaEngine) FetchMetadata(rawURL string) (*SourceMetadata, error) {
	module, err := e.pickExecutableModule(rawURL)
	if err != nil {
		return nil, err
	}

	rt, err := e.newRuntime(module, rawURL)
	if err != nil {
		return nil, err
	}
	defer rt.Close()

	if err := rt.loadURL(context.Background(), rawURL, nil); err != nil {
		return nil, err
	}
	if err := rt.callIfPresent("GetInfo"); err != nil {
		return nil, newUnsupportedURLError("hdoujin", "module GetInfo failed: "+err.Error())
	}
	if err := rt.callIfPresent("GetPages"); err != nil {
		return nil, newUnsupportedURLError("hdoujin", "module GetPages failed: "+err.Error())
	}
	if len(rt.pages) == 0 {
		if err := rt.loadURL(context.Background(), rawURL, nil); err != nil {
			return nil, err
		}
		if err := rt.callIfPresent("GetChapters"); err != nil {
			return nil, newUnsupportedURLError("hdoujin", "module GetChapters failed: "+err.Error())
		}
	}

	title := rt.infoString("Title")
	if title == "" {
		title = sanitizeArchiveName(pathBaseWithoutExt(rawURL))
	}

	return &SourceMetadata{
		Title:      title,
		Tags:       rt.importTags(),
		TotalFiles: maxInt(len(rt.pages), len(rt.chapters)),
	}, nil
}

func (e *HDoujinLuaEngine) Download(ctx context.Context, job *models.DownloadJob) error {
	module, err := e.pickExecutableModule(job.URL)
	if err != nil {
		return err
	}

	rt, err := e.newRuntime(module, job.URL)
	if err != nil {
		return err
	}
	defer rt.Close()

	if err := rt.loadURL(ctx, job.URL, nil); err != nil {
		return err
	}
	if err := rt.callIfPresent("GetInfo"); err != nil {
		return newUnsupportedURLError("hdoujin", "module GetInfo failed: "+err.Error())
	}
	if err := rt.callIfPresent("GetPages"); err != nil {
		return newUnsupportedURLError("hdoujin", "module GetPages failed: "+err.Error())
	}
	if len(rt.pages) == 0 && rt.module.HasGetChapters {
		if err := rt.loadURL(ctx, job.URL, nil); err != nil {
			return err
		}
		if err := rt.callIfPresent("GetChapters"); err != nil {
			return newUnsupportedURLError("hdoujin", "module GetChapters failed: "+err.Error())
		}
		if len(rt.chapters) > 0 {
			return e.downloadChapterSeries(ctx, job, rt, module)
		}
	}
	if len(rt.pages) == 0 {
		return newUnsupportedURLError("hdoujin", "module did not yield downloadable pages")
	}

	title := rt.infoString("Title")
	if title == "" {
		title = rt.module.Name
	}
	archiveName := sanitizeArchiveName(title)
	if archiveName == "" {
		archiveName = sanitizeArchiveName(pathBaseWithoutExt(job.URL))
	}
	if archiveName == "" {
		archiveName = "hdoujin-download"
	}

	if err := os.MkdirAll(job.TargetDirectory, 0o755); err != nil {
		return fmt.Errorf("hdoujin mkdir: %w", err)
	}

	finalPath := filepath.Join(job.TargetDirectory, archiveName+".cbz")
	tmpPath := finalPath + ".part"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("hdoujin create cbz: %w", err)
	}

	zw := zip.NewWriter(file)
	for idx, rawPage := range rt.pages {
		downloadURL, err := rt.resolveDownloadURL(ctx, rawPage)
		if err != nil {
			_ = zw.Close()
			_ = file.Close()
			return err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
		if err != nil {
			_ = zw.Close()
			_ = file.Close()
			return fmt.Errorf("hdoujin page request %d: %w", idx+1, err)
		}
		req.Header.Set("User-Agent", "Tanuki/1.0")
		req.Header.Set("Referer", sourceBaseURL(downloadURL))

		resp, err := e.client.Do(req)
		if err != nil {
			_ = zw.Close()
			_ = file.Close()
			return fmt.Errorf("hdoujin page fetch %d: %w", idx+1, err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			_ = zw.Close()
			_ = file.Close()
			if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusTooManyRequests {
				return newUnsupportedURLError("hdoujin", blockedSourceDetail(resp.StatusCode))
			}
			return fmt.Errorf("hdoujin page status %d: %d", idx+1, resp.StatusCode)
		}

		entryName, err := archiveEntryName(idx+1, downloadURL)
		if err != nil {
			resp.Body.Close()
			_ = zw.Close()
			_ = file.Close()
			return err
		}
		writer, err := zw.Create(entryName)
		if err != nil {
			resp.Body.Close()
			_ = zw.Close()
			_ = file.Close()
			return fmt.Errorf("hdoujin cbz entry %d: %w", idx+1, err)
		}
		if _, err := io.Copy(writer, resp.Body); err != nil {
			resp.Body.Close()
			_ = zw.Close()
			_ = file.Close()
			return fmt.Errorf("hdoujin cbz write %d: %w", idx+1, err)
		}
		resp.Body.Close()
		if e.progress != nil {
			e.progress(job.ID, 0, 0, idx+1, len(rt.pages))
		}
	}

	if err := zw.Close(); err != nil {
		_ = file.Close()
		return fmt.Errorf("hdoujin close cbz: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("hdoujin close file: %w", err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return fmt.Errorf("hdoujin rename cbz: %w", err)
	}

	if err := writeImportMetadata(finalPath, models.ImportMetadata{
		Title:     title,
		SourceURL: job.URL,
		Tags:      rt.importTags(),
	}); err != nil {
		return fmt.Errorf("hdoujin metadata: %w", err)
	}

	return nil
}

func (e *HDoujinLuaEngine) pickExecutableModule(rawURL string) (*hdoujinLuaModule, error) {
	matches := e.matchingModules(rawURL)
	if len(matches) == 0 {
		return nil, newUnsupportedURLError("hdoujin", "no matching local lua module found")
	}
	for _, module := range matches {
		if module.HasGetPages || module.HasGetInfo {
			return module, nil
		}
	}
	return matches[0], nil
}

func (e *HDoujinLuaEngine) matchingModules(rawURL string) []*hdoujinLuaModule {
	u, err := urlpkg.Parse(rawURL)
	if err != nil {
		return nil
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return nil
	}
	if exact := e.modulesByHost[host]; len(exact) > 0 {
		return exact
	}
	if strings.HasPrefix(host, "www.") {
		return e.modulesByHost[strings.TrimPrefix(host, "www.")]
	}
	if alt := e.modulesByHost["www."+host]; len(alt) > 0 {
		return alt
	}
	return nil
}

func (e *HDoujinLuaEngine) loadModules() {
	if e.modulesDir == "" {
		return
	}
	entries, err := os.ReadDir(e.modulesDir)
	if err != nil {
		if e.log != nil {
			e.log.Warn("hdoujin: modules directory unavailable", zap.String("path", e.modulesDir), zap.Error(err))
		}
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".lua") {
			continue
		}
		path := filepath.Join(e.modulesDir, entry.Name())
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		module := e.loadModule(path, string(body))
		if module == nil || len(module.Domains) == 0 {
			continue
		}
		for _, domain := range module.Domains {
			e.modulesByHost[domain] = append(e.modulesByHost[domain], module)
		}
	}

	for host, modules := range e.modulesByHost {
		sort.SliceStable(modules, func(i, j int) bool {
			if modules[i].Strict == modules[j].Strict {
				return modules[i].FileName < modules[j].FileName
			}
			return modules[i].Strict && !modules[j].Strict
		})
		e.modulesByHost[host] = modules
	}
	e.enabled = len(e.modulesByHost) > 0
}

func (e *HDoujinLuaEngine) loadModule(path, body string) *hdoujinLuaModule {
	module := &hdoujinLuaModule{
		Path:                  path,
		FileName:              filepath.Base(path),
		Body:                  body,
		HasBeforeDownloadPage: strings.Contains(body, "function BeforeDownloadPage"),
	}

	audit := AuditHDoujinModule(module.FileName, body)
	module.HasGetInfo = audit.HasGetInfo
	module.HasGetPages = audit.HasGetPages
	module.HasGetChapters = audit.HasGetChapters

	L := lua.NewState(lua.Options{SkipOpenLibs: false})
	defer L.Close()

	domains := make([]string, 0, 4)
	domainsTbl := L.NewTable()
	L.SetField(domainsTbl, "Add", L.NewFunction(func(L *lua.LState) int {
		value := strings.ToLower(strings.TrimSpace(luaStringArgMaybeSelf(L, domainsTbl, 1)))
		if value == "" || slices.Contains(domains, value) {
			return 0
		}
		domains = append(domains, value)
		return 0
	}))
	L.SetField(domainsTbl, "First", L.NewFunction(func(L *lua.LState) int {
		if len(domains) == 0 {
			L.Push(lua.LString(""))
		} else {
			L.Push(lua.LString(domains[0]))
		}
		return 1
	}))

	moduleTbl := L.NewTable()
	L.SetField(moduleTbl, "Domains", domainsTbl)
	L.SetGlobal("module", moduleTbl)

	globalTbl := L.NewTable()
	L.SetField(globalTbl, "SetCookie", L.NewFunction(func(L *lua.LState) int { return 0 }))
	L.SetField(globalTbl, "SetCookies", L.NewFunction(func(L *lua.LState) int { return 0 }))
	L.SetGlobal("global", globalTbl)
	L.SetGlobal("DoEncryptedString", L.NewFunction(func(L *lua.LState) int { return 0 }))

	if err := L.DoString(body); err == nil {
		if register := L.GetGlobal("Register"); register.Type() == lua.LTFunction {
			_ = L.CallByParam(lua.P{Fn: register, NRet: 0, Protect: true})
		}
	}

	module.Name = getLuaStringField(moduleTbl, "Name")
	module.Language = getLuaStringField(moduleTbl, "Language")
	module.Type = getLuaStringField(moduleTbl, "Type")
	module.Adult = lua.LVAsBool(L.GetField(moduleTbl, "Adult"))
	module.Strict = lua.LVAsBool(L.GetField(moduleTbl, "Strict"))
	module.Domains = compactStrings(domains)

	if len(module.Domains) == 0 {
		module.Domains = compactStrings(audit.Domains)
	}
	if module.Name == "" {
		module.Name = audit.DisplayName
	}
	if module.Language == "" {
		module.Language = audit.Language
	}

	return module
}

func (e *HDoujinLuaEngine) newRuntime(module *hdoujinLuaModule, rawURL string) (*hdoujinRuntime, error) {
	u, err := urlpkg.Parse(rawURL)
	if err != nil {
		return nil, newUnsupportedURLError("hdoujin", "invalid url")
	}

	rt := &hdoujinRuntime{
		L:           lua.NewState(lua.Options{SkipOpenLibs: false}),
		module:      module,
		client:      e.client,
		log:         e.log,
		currentURL:  rawURL,
		info:        map[string]lua.LValue{},
		matchedHost: strings.ToLower(u.Hostname()),
	}
	rt.installBaseRuntime()
	if err := rt.L.DoString(module.Body); err != nil {
		rt.Close()
		return nil, newUnsupportedURLError("hdoujin", "module load failed: "+err.Error())
	}
	if err := rt.callIfPresent("Register"); err != nil {
		rt.Close()
		return nil, newUnsupportedURLError("hdoujin", "module Register failed: "+err.Error())
	}
	return rt, nil
}

func (rt *hdoujinRuntime) Close() {
	if rt != nil && rt.L != nil {
		rt.L.Close()
	}
}

func (rt *hdoujinRuntime) installBaseRuntime() {
	L := rt.L
	rt.extendStringLibrary()

	moduleTbl := L.NewTable()
	L.SetField(moduleTbl, "Name", lua.LString(rt.module.Name))
	L.SetField(moduleTbl, "Language", lua.LString(rt.module.Language))
	L.SetField(moduleTbl, "Type", lua.LString(rt.module.Type))
	L.SetField(moduleTbl, "Adult", lua.LBool(rt.module.Adult))
	L.SetField(moduleTbl, "Strict", lua.LBool(rt.module.Strict))
	L.SetField(moduleTbl, "Domain", lua.LString(rt.matchedHost))
	L.SetField(moduleTbl, "Settings", rt.newModuleSettingsTable())

	domains := append([]string(nil), rt.module.Domains...)
	domainsTbl := L.NewTable()
	L.SetField(domainsTbl, "Add", L.NewFunction(func(L *lua.LState) int {
		value := strings.ToLower(strings.TrimSpace(luaStringArgMaybeSelf(L, domainsTbl, 1)))
		if value == "" || slices.Contains(domains, value) {
			return 0
		}
		domains = append(domains, value)
		return 0
	}))
	L.SetField(domainsTbl, "First", L.NewFunction(func(L *lua.LState) int {
		if len(domains) == 0 {
			L.Push(lua.LString(""))
		} else {
			L.Push(lua.LString(domains[0]))
		}
		return 1
	}))
	L.SetField(moduleTbl, "Domains", domainsTbl)
	L.SetGlobal("module", moduleTbl)

	infoTbl := L.NewTable()
	infoMeta := L.NewTable()
	L.SetField(infoMeta, "__newindex", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(2)
		value := L.CheckAny(3)
		rt.info[key] = value
		L.RawSet(L.CheckTable(1), lua.LString(key), value)
		return 0
	}))
	L.SetMetatable(infoTbl, infoMeta)
	L.SetGlobal("info", infoTbl)

	pagesTbl := L.NewTable()
	L.SetField(pagesTbl, "Add", L.NewFunction(func(L *lua.LState) int {
		rt.pages = append(rt.pages, rt.collectPageValues(luaValueArgMaybeSelf(L, pagesTbl, 1))...)
		rt.syncPagesTable()
		return 0
	}))
	L.SetField(pagesTbl, "AddRange", L.NewFunction(func(L *lua.LState) int {
		rt.pages = append(rt.pages, rt.collectPageValues(luaValueArgMaybeSelf(L, pagesTbl, 1))...)
		rt.syncPagesTable()
		return 0
	}))
	L.SetField(pagesTbl, "Count", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(len(rt.pages)))
		return 1
	}))
	L.SetField(pagesTbl, "Reverse", L.NewFunction(func(L *lua.LState) int {
		reverseStrings(rt.pages)
		rt.syncPagesTable()
		return 0
	}))
	L.SetField(pagesTbl, "Sort", L.NewFunction(func(L *lua.LState) int {
		sortStringsNaturally(rt.pages)
		rt.syncPagesTable()
		return 0
	}))
	L.SetField(pagesTbl, "Headers", L.NewTable())
	L.SetField(pagesTbl, "Referer", lua.LString(""))
	rt.pagesTbl = pagesTbl
	rt.syncPagesTable()
	L.SetGlobal("pages", pagesTbl)

	chaptersTbl := L.NewTable()
	L.SetField(chaptersTbl, "Add", L.NewFunction(func(L *lua.LState) int {
		if chapter, ok := rt.chapterFromArgs(L, chaptersTbl); ok {
			rt.chapters = append(rt.chapters, chapter)
			rt.syncChaptersTable()
		}
		return 0
	}))
	L.SetField(chaptersTbl, "AddRange", L.NewFunction(func(L *lua.LState) int {
		rt.chapters = append(rt.chapters, rt.collectChapterValues(luaCollectionValues(luaValueArgMaybeSelf(L, chaptersTbl, 1))...)...)
		rt.syncChaptersTable()
		return 0
	}))
	L.SetField(chaptersTbl, "Count", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(len(rt.chapters)))
		return 1
	}))
	L.SetField(chaptersTbl, "Reverse", L.NewFunction(func(L *lua.LState) int {
		reverseChapters(rt.chapters)
		rt.syncChaptersTable()
		return 0
	}))
	L.SetField(chaptersTbl, "Sort", L.NewFunction(func(L *lua.LState) int {
		sortChaptersNaturally(rt.chapters)
		rt.syncChaptersTable()
		return 0
	}))
	rt.chaptersTbl = chaptersTbl
	rt.syncChaptersTable()
	L.SetGlobal("chapters", chaptersTbl)

	pageTbl := rt.newDOMValue(nil)
	L.SetField(pageTbl, "Url", lua.LString(""))
	L.SetGlobal("page", pageTbl)
	L.SetGlobal("Dom", rt.newDOMFactory())
	L.SetGlobal("Json", rt.newJSONFactory())
	L.SetGlobal("JavaScript", rt.newJavaScriptFactory())
	L.SetGlobal("ChapterInfo", rt.newChapterInfoFactory())
	L.SetGlobal("PageInfo", rt.newPageInfoFactory())
	L.SetGlobal("ChapterList", rt.newListFactory())
	L.SetGlobal("PageList", rt.newListFactory())
	L.SetGlobal("Dict", rt.newDictFactory())

	globalTbl := L.NewTable()
	L.SetField(globalTbl, "SetCookie", L.NewFunction(func(L *lua.LState) int {
		domain := strings.TrimSpace(luaStringArgMaybeSelf(L, globalTbl, 1))
		name := strings.TrimSpace(luaStringArgMaybeSelf(L, globalTbl, 2))
		value := luaStringArgMaybeSelf(L, globalTbl, 3)
		if domain == "" || name == "" || rt.client == nil || rt.client.Jar == nil {
			return 0
		}
		u := &urlpkg.URL{Scheme: "https", Host: domain, Path: "/"}
		rt.client.Jar.SetCookies(u, []*http.Cookie{{Name: name, Value: value, Domain: domain, Path: "/"}})
		return 0
	}))
	L.SetField(globalTbl, "SetCookies", L.NewFunction(func(L *lua.LState) int {
		rt.setCookies(luaValueArgMaybeSelf(L, globalTbl, 1))
		return 0
	}))
	L.SetGlobal("global", globalTbl)

	headersTbl := L.NewTable()
	postDataTbl := L.NewTable()
	L.SetField(postDataTbl, "Add", L.NewFunction(func(L *lua.LState) int {
		key := strings.TrimSpace(luaStringArgMaybeSelf(L, postDataTbl, 1))
		value := luaValueArgMaybeSelf(L, postDataTbl, 2)
		if key != "" {
			postDataTbl.RawSetString(key, lua.LString(strings.TrimSpace(value.String())))
		}
		return 0
	}))
	L.SetGlobal("http", rt.newHTTPTable(headersTbl, postDataTbl))

	L.SetGlobal("url", lua.LString(rt.currentURL))
	L.SetGlobal("dom", rt.newDOMValue(nil))
	L.SetGlobal("doc", lua.LString(rt.currentBody))
	L.SetGlobal("API_VERSION", lua.LNumber(20260312))
	L.SetGlobal("isempty", L.NewFunction(func(L *lua.LState) int {
		value := L.CheckAny(1)
		empty := false
		switch v := value.(type) {
		case lua.LString:
			empty = strings.TrimSpace(v.String()) == ""
		case *lua.LTable:
			empty = v.Len() == 0
		case *lua.LNilType:
			empty = true
		default:
			empty = value.String() == ""
		}
		L.Push(lua.LBool(empty))
		return 1
	}))
	L.SetGlobal("GetParameter", L.NewFunction(func(L *lua.LState) int {
		raw := L.CheckString(1)
		key := L.CheckString(2)
		u, err := urlpkg.Parse(raw)
		if err != nil {
			L.Push(lua.LString(""))
			return 1
		}
		L.Push(lua.LString(u.Query().Get(key)))
		return 1
	}))
	L.SetGlobal("GetRoot", L.NewFunction(func(L *lua.LState) int {
		raw := L.CheckString(1)
		u, err := urlpkg.Parse(raw)
		if err != nil {
			L.Push(lua.LString(""))
			return 1
		}
		L.Push(lua.LString(sourceBaseURL(u.String())))
		return 1
	}))
	L.SetGlobal("GetRooted", L.NewFunction(func(L *lua.LState) int {
		value := luaCollectionValues(L.CheckAny(1))
		root := L.CheckString(2)
		if len(value) == 0 {
			L.Push(lua.LString(""))
			return 1
		}
		L.Push(lua.LString(resolveHDoujinURL(root, strings.TrimSpace(value[0].String()))))
		return 1
	}))
	L.SetGlobal("toboolean", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(luaValueAsBool(L.CheckAny(1))))
		return 1
	}))
	L.SetGlobal("RegexReplace", L.NewFunction(func(L *lua.LState) int {
		value := L.CheckString(1)
		pattern := L.CheckString(2)
		replacement := L.CheckString(3)
		re, err := regexp.Compile(pattern)
		if err != nil {
			L.Push(lua.LString(value))
			return 1
		}
		L.Push(lua.LString(re.ReplaceAllString(value, replacement)))
		return 1
	}))
	L.SetGlobal("DoEncryptedString", L.NewFunction(func(L *lua.LState) int {
		L.RaiseError("encrypted HDoujin modules are not supported in the first compatibility pass")
		return 0
	}))
}

func (rt *hdoujinRuntime) extendStringLibrary() {
	stringTbl, ok := rt.L.GetGlobal("string").(*lua.LTable)
	if !ok {
		return
	}

	rt.L.SetField(stringTbl, "trim", rt.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(strings.TrimSpace(L.CheckString(1))))
		return 1
	}))
	rt.L.SetField(stringTbl, "startswith", rt.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(strings.HasPrefix(L.CheckString(1), L.CheckString(2))))
		return 1
	}))
	rt.L.SetField(stringTbl, "endswith", rt.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(strings.HasSuffix(L.CheckString(1), L.CheckString(2))))
		return 1
	}))
	rt.L.SetField(stringTbl, "contains", rt.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(strings.Contains(L.CheckString(1), L.CheckString(2))))
		return 1
	}))
	rt.L.SetField(stringTbl, "before", rt.L.NewFunction(func(L *lua.LState) int {
		value := L.CheckString(1)
		sep := L.CheckString(2)
		if idx := strings.Index(value, sep); idx >= 0 {
			L.Push(lua.LString(value[:idx]))
		} else {
			L.Push(lua.LString(value))
		}
		return 1
	}))
	rt.L.SetField(stringTbl, "after", rt.L.NewFunction(func(L *lua.LState) int {
		value := L.CheckString(1)
		sep := L.CheckString(2)
		if idx := strings.Index(value, sep); idx >= 0 {
			L.Push(lua.LString(value[idx+len(sep):]))
		} else {
			L.Push(lua.LString(""))
		}
		return 1
	}))
	rt.L.SetField(stringTbl, "regex", rt.L.NewFunction(func(L *lua.LState) int {
		value := L.CheckString(1)
		pattern := L.CheckString(2)
		group := L.OptInt(3, 0)
		re, err := regexp.Compile(pattern)
		if err != nil {
			L.Push(lua.LString(""))
			return 1
		}
		match := re.FindStringSubmatch(value)
		if len(match) == 0 {
			L.Push(lua.LString(""))
			return 1
		}
		if group < 0 || group >= len(match) {
			group = 0
		}
		L.Push(lua.LString(match[group]))
		return 1
	}))
	rt.L.SetField(stringTbl, "beforelast", rt.L.NewFunction(func(L *lua.LState) int {
		value := L.CheckString(1)
		sep := L.CheckString(2)
		if idx := strings.LastIndex(value, sep); idx >= 0 {
			L.Push(lua.LString(value[:idx]))
		} else {
			L.Push(lua.LString(value))
		}
		return 1
	}))
	rt.L.SetField(stringTbl, "between", rt.L.NewFunction(func(L *lua.LState) int {
		value := L.CheckString(1)
		left := L.CheckString(2)
		right := L.CheckString(3)
		start := strings.Index(value, left)
		if start < 0 {
			L.Push(lua.LString(""))
			return 1
		}
		start += len(left)
		end := strings.Index(value[start:], right)
		if end < 0 {
			L.Push(lua.LString(""))
			return 1
		}
		L.Push(lua.LString(value[start : start+end]))
		return 1
	}))
	rt.L.SetField(stringTbl, "replace", rt.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(strings.ReplaceAll(L.CheckString(1), L.CheckString(2), L.CheckString(3))))
		return 1
	}))
	rt.L.SetField(stringTbl, "regexmany", rt.L.NewFunction(func(L *lua.LState) int {
		value := L.CheckString(1)
		pattern := L.CheckString(2)
		group := L.OptInt(3, 0)
		re, err := regexp.Compile(pattern)
		if err != nil {
			L.Push(rt.newCollection(nil))
			return 1
		}
		matches := re.FindAllStringSubmatch(value, -1)
		out := make([]lua.LValue, 0, len(matches))
		for _, match := range matches {
			idx := group
			if idx < 0 || idx >= len(match) {
				idx = 0
			}
			out = append(out, lua.LString(match[idx]))
		}
		L.Push(rt.newCollection(out))
		return 1
	}))
}

func (rt *hdoujinRuntime) newDOMValue(node *html.Node) lua.LValue {
	wrapper := &hdoujinDOMNode{runtime: rt, node: node}
	tbl := rt.L.NewTable()
	meta := rt.L.NewTable()
	ud := rt.L.NewUserData()
	ud.Value = wrapper
	rt.L.RawSet(tbl, lua.LString("__tanuki_dom_node"), ud)

	rt.L.SetField(tbl, "SelectValue", rt.L.NewFunction(func(L *lua.LState) int {
		values := wrapper.selectValues(luaStringArgMaybeSelf(L, tbl, 1))
		if len(values) == 0 {
			L.Push(lua.LString(""))
		} else {
			L.Push(lua.LString(values[0]))
		}
		return 1
	}))
	rt.L.SetField(tbl, "SelectValues", rt.L.NewFunction(func(L *lua.LState) int {
		values := wrapper.selectValues(luaStringArgMaybeSelf(L, tbl, 1))
		out := make([]lua.LValue, 0, len(values))
		for _, value := range values {
			out = append(out, lua.LString(value))
		}
		L.Push(rt.newCollection(out))
		return 1
	}))
	rt.L.SetField(tbl, "SelectElement", rt.L.NewFunction(func(L *lua.LState) int {
		nodes := wrapper.selectNodes(luaStringArgMaybeSelf(L, tbl, 1))
		if len(nodes) == 0 {
			L.Push(lua.LNil)
		} else {
			L.Push(rt.newDOMValue(nodes[0]))
		}
		return 1
	}))
	rt.L.SetField(tbl, "SelectElements", rt.L.NewFunction(func(L *lua.LState) int {
		nodes := wrapper.selectNodes(luaStringArgMaybeSelf(L, tbl, 1))
		out := make([]lua.LValue, 0, len(nodes))
		for _, n := range nodes {
			out = append(out, rt.newDOMValue(n))
		}
		L.Push(rt.newCollection(out))
		return 1
	}))
	rt.L.SetField(tbl, "SelectNode", rt.L.GetField(tbl, "SelectElement"))
	rt.L.SetField(tbl, "SelectNodes", rt.L.GetField(tbl, "SelectElements"))
	rt.L.SetField(tbl, "New", rt.L.NewFunction(func(L *lua.LState) int {
		fragment := strings.TrimSpace(luaStringArgMaybeSelf(L, tbl, 1))
		node, err := parseHTMLFragment(fragment)
		if err != nil {
			L.RaiseError("dom.New parse failed: %v", err)
			return 0
		}
		L.Push(rt.newDOMValue(node))
		return 1
	}))
	rt.L.SetField(tbl, "Title", lua.LString(domDocumentTitle(wrapper.root())))
	rt.L.SetMetatable(tbl, meta)
	return tbl
}

func (rt *hdoujinRuntime) newCollection(values []lua.LValue) *lua.LTable {
	tbl := rt.L.NewTable()
	for _, value := range values {
		tbl.Append(value)
	}
	rt.L.SetField(tbl, "Count", rt.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(tbl.Len()))
		return 1
	}))
	rt.L.SetField(tbl, "First", rt.L.NewFunction(func(L *lua.LState) int {
		if tbl.Len() == 0 {
			L.Push(lua.LNil)
		} else {
			L.Push(tbl.RawGetInt(1))
		}
		return 1
	}))
	rt.L.SetField(tbl, "Last", rt.L.NewFunction(func(L *lua.LState) int {
		if tbl.Len() == 0 {
			L.Push(lua.LNil)
		} else {
			L.Push(tbl.RawGetInt(tbl.Len()))
		}
		return 1
	}))

	index := 0
	meta := rt.L.NewTable()
	rt.L.SetField(meta, "__index", rt.L.NewFunction(func(L *lua.LState) int {
		key := L.CheckAny(2)
		switch typed := key.(type) {
		case lua.LNumber:
			index := int(typed)
			if index >= 0 {
				L.Push(tbl.RawGetInt(index + 1))
				return 1
			}
		case lua.LString:
			if value := tbl.RawGetString(typed.String()); value != lua.LNil {
				L.Push(value)
				return 1
			}
		}
		L.Push(lua.LNil)
		return 1
	}))
	rt.L.SetField(meta, "__call", rt.L.NewFunction(func(L *lua.LState) int {
		if index >= tbl.Len() {
			return 0
		}
		index++
		L.Push(tbl.RawGetInt(index))
		return 1
	}))
	rt.L.SetMetatable(tbl, meta)
	return tbl
}

func (node *hdoujinDOMNode) root() *html.Node {
	if node.node != nil {
		return node.node
	}
	return node.runtime.currentDoc
}

func (rt *hdoujinRuntime) collectPageValues(value lua.LValue) []string {
	items := luaCollectionValues(value)
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text := strings.TrimSpace(rt.pageValueString(item)); text != "" {
			out = append(out, text)
		}
	}
	return compactStrings(out)
}

func (rt *hdoujinRuntime) pageValueString(value lua.LValue) string {
	switch v := value.(type) {
	case lua.LString:
		return v.String()
	case *lua.LTable:
		if node := domNodeFromValue(v); node != nil {
			candidates := []string{
				firstString(node.selectValues("./@href")),
				firstString(node.selectValues("./@src")),
				firstString(node.selectValues(".//@href")),
				firstString(node.selectValues(".//@src")),
				firstString(node.selectValues("./@data-src")),
				firstString(node.selectValues(".//@data-src")),
				firstString(node.selectValues("./@data-original")),
				firstString(node.selectValues(".//@data-original")),
			}
			for _, candidate := range candidates {
				if strings.TrimSpace(candidate) != "" {
					return candidate
				}
			}
			return firstString(node.selectValues("."))
		}
		if url := strings.TrimSpace(firstNonEmpty(v.RawGetString("Url").String(), v.RawGetString("URL").String())); url != "" {
			return url
		}
	}
	return strings.TrimSpace(value.String())
}

func (rt *hdoujinRuntime) collectChapterValues(values ...lua.LValue) []hdoujinChapter {
	if len(values) == 1 {
		values = luaCollectionValues(values[0])
	}
	out := make([]hdoujinChapter, 0, len(values))
	for _, value := range values {
		switch v := value.(type) {
		case lua.LString:
			if url := strings.TrimSpace(v.String()); url != "" {
				out = append(out, hdoujinChapter{URL: url})
			}
		case *lua.LTable:
			if node := domNodeFromValue(v); node != nil {
				url := firstNonEmpty(
					firstString(node.selectValues("./@href")),
					firstString(node.selectValues(".//@href")),
					firstString(node.selectValues("./@src")),
					firstString(node.selectValues(".//@src")),
				)
				title := firstNonEmpty(
					firstString(node.selectValues("./@title")),
					firstString(node.selectValues(".//@title")),
					firstString(node.selectValues(".")),
				)
				if strings.TrimSpace(url) != "" {
					out = append(out, hdoujinChapter{URL: url, Title: title})
				}
				continue
			}
			url := strings.TrimSpace(firstNonEmpty(v.RawGetString("Url").String(), v.RawGetString("URL").String()))
			title := strings.TrimSpace(firstNonEmpty(v.RawGetString("Title").String(), v.RawGetString("Name").String()))
			if url != "" {
				out = append(out, hdoujinChapter{URL: url, Title: title})
			}
		}
	}
	return out
}

func (rt *hdoujinRuntime) chapterFromArgs(L *lua.LState, self lua.LValue) (hdoujinChapter, bool) {
	first := luaValueArgMaybeSelf(L, self, 1)
	second := luaValueArgMaybeSelf(L, self, 2)
	if table, ok := first.(*lua.LTable); ok {
		if node := domNodeFromValue(table); node != nil {
			chapters := rt.collectChapterValues(first)
			if len(chapters) > 0 {
				return chapters[0], true
			}
			return hdoujinChapter{}, false
		}
		if chapters := rt.collectChapterValues(first); len(chapters) > 0 {
			return chapters[0], true
		}
	}
	url := strings.TrimSpace(rt.pageValueString(first))
	if url == "" {
		return hdoujinChapter{}, false
	}
	title := strings.TrimSpace(second.String())
	if _, ok := second.(*lua.LNilType); ok {
		title = ""
	}
	return hdoujinChapter{URL: url, Title: title}, true
}

func (node *hdoujinDOMNode) selectNodes(expr string) []*html.Node {
	root := node.root()
	if root == nil {
		return nil
	}
	nodes, err := htmlquery.QueryAll(root, strings.TrimSpace(expr))
	if err != nil {
		return nil
	}
	return nodes
}

func (node *hdoujinDOMNode) selectValues(expr string) []string {
	root := node.root()
	if root == nil {
		return nil
	}

	trimmedExpr := strings.TrimSpace(expr)
	if strings.HasPrefix(trimmedExpr, "@") {
		attr := strings.TrimPrefix(trimmedExpr, "@")
		if value := strings.TrimSpace(htmlstd.UnescapeString(htmlquery.SelectAttr(root, attr))); value != "" {
			return []string{value}
		}
		return nil
	}
	if match := hdoujinAttrSuffixRe.FindStringSubmatch(trimmedExpr); len(match) == 3 {
		baseExpr := strings.TrimSpace(match[1])
		attr := match[2]
		if baseExpr == "" {
			baseExpr = "."
		}
		nodes, err := htmlquery.QueryAll(root, baseExpr)
		if err != nil {
			return nil
		}
		out := make([]string, 0, len(nodes))
		for _, n := range nodes {
			if value := strings.TrimSpace(htmlstd.UnescapeString(htmlquery.SelectAttr(n, attr))); value != "" {
				out = append(out, value)
			}
		}
		return out
	}

	nodes, err := htmlquery.QueryAll(root, trimmedExpr)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if text := strings.TrimSpace(htmlstd.UnescapeString(htmlquery.InnerText(n))); text != "" {
			out = append(out, text)
		}
	}
	return out
}

func (rt *hdoujinRuntime) loadURL(ctx context.Context, rawURL string, headers *lua.LTable) error {
	target, err := urlpkg.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("hdoujin parse url: %w", err)
	}
	if base, parseErr := urlpkg.Parse(rt.currentURL); parseErr == nil {
		target = base.ResolveReference(target)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return fmt.Errorf("hdoujin request: %w", err)
	}
	req.Header.Set("User-Agent", "Tanuki/1.0")
	req.Header.Set("Referer", sourceBaseURL(target.String()))
	if headers != nil {
		headers.ForEach(func(key, value lua.LValue) {
			req.Header.Set(key.String(), value.String())
		})
	}

	resp, err := rt.client.Do(req)
	if err != nil {
		return fmt.Errorf("hdoujin fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusTooManyRequests {
			return newUnsupportedURLError("hdoujin", blockedSourceDetail(resp.StatusCode))
		}
		return fmt.Errorf("hdoujin status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("hdoujin read body: %w", err)
	}

	doc, err := htmlquery.Parse(strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("hdoujin parse html: %w", err)
	}

	rt.currentURL = target.String()
	rt.currentBody = string(body)
	rt.currentDoc = doc
	rt.L.SetGlobal("url", lua.LString(rt.currentURL))
	rt.L.SetGlobal("doc", lua.LString(rt.currentBody))
	rt.L.SetGlobal("dom", rt.newDOMValue(nil))
	if pageTbl, ok := rt.L.GetGlobal("page").(*lua.LTable); ok {
		rt.L.SetField(pageTbl, "Url", lua.LString(rt.currentURL))
	}
	return nil
}

func (rt *hdoujinRuntime) callIfPresent(name string) error {
	fn := rt.L.GetGlobal(name)
	if fn.Type() != lua.LTFunction {
		return nil
	}
	return rt.L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true})
}

func (rt *hdoujinRuntime) resolveDownloadURL(ctx context.Context, rawPage string) (string, error) {
	pageURL := resolveImageURL(mustURL(rt.currentURL), rawPage)
	if pageURL == "" {
		pageURL = rawPage
	}
	if !rt.module.HasBeforeDownloadPage || looksLikeDirectMediaURL(pageURL) {
		return pageURL, nil
	}

	if err := rt.loadURL(ctx, pageURL, nil); err != nil {
		return "", err
	}

	pageTbl, ok := rt.L.GetGlobal("page").(*lua.LTable)
	if ok {
		rt.L.SetField(pageTbl, "Url", lua.LString(pageURL))
	}
	if err := rt.callIfPresent("BeforeDownloadPage"); err != nil {
		return "", newUnsupportedURLError("hdoujin", "module BeforeDownloadPage failed: "+err.Error())
	}
	if ok {
		if updated := strings.TrimSpace(rt.L.GetField(pageTbl, "Url").String()); updated != "" {
			return resolveImageURL(mustURL(rt.currentURL), updated), nil
		}
	}
	return pageURL, nil
}

func (rt *hdoujinRuntime) infoString(key string) string {
	value, ok := rt.info[key]
	if !ok {
		return ""
	}
	switch v := value.(type) {
	case lua.LString:
		return strings.TrimSpace(v.String())
	case *lua.LTable:
		values := rt.tableStrings(v)
		return strings.Join(values, ", ")
	default:
		return strings.TrimSpace(value.String())
	}
}

func (rt *hdoujinRuntime) infoStrings(key string) []string {
	value, ok := rt.info[key]
	if !ok {
		return nil
	}
	switch v := value.(type) {
	case lua.LString:
		if text := strings.TrimSpace(v.String()); text != "" {
			return []string{text}
		}
	case *lua.LTable:
		return rt.tableStrings(v)
	}
	return nil
}

func (rt *hdoujinRuntime) tableStrings(tbl *lua.LTable) []string {
	out := make([]string, 0, tbl.Len())
	for idx := 1; idx <= tbl.Len(); idx++ {
		value := tbl.RawGetInt(idx)
		text := strings.TrimSpace(value.String())
		if table, ok := value.(*lua.LTable); ok {
			if node := domNodeFromValue(table); node != nil {
				text = strings.TrimSpace(firstNonEmpty(
					firstString(node.selectValues("./@title")),
					firstString(node.selectValues(".//@title")),
					firstString(node.selectValues(".")),
				))
			}
		}
		if text != "" {
			out = append(out, text)
		}
	}
	return compactStrings(out)
}

func (rt *hdoujinRuntime) importTags() []string {
	tags := make([]string, 0, 32)
	for _, artist := range append(rt.infoStrings("Artist"), rt.infoStrings("Circle")...) {
		tags = append(tags, withNamespaces(artist, "artist", "circle")...)
	}
	for _, parody := range rt.infoStrings("Parody") {
		tags = append(tags, withNamespaces(parody, "parody", "series")...)
	}
	for _, character := range rt.infoStrings("Characters") {
		tags = append(tags, withNamespaces(character, "character")...)
	}
	for _, tag := range rt.infoStrings("Tags") {
		tags = append(tags, withNamespaces(tag, "genre")...)
	}
	tags = append(tags, "site:"+strings.ToLower(strings.TrimSuffix(rt.module.FileName, filepath.Ext(rt.module.FileName))))
	return compactStrings(tags)
}

func withNamespaces(value string, namespaces ...string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	out := make([]string, 0, len(namespaces))
	for _, ns := range namespaces {
		out = append(out, strings.ToLower(strings.TrimSpace(ns))+":"+value)
	}
	return out
}

func getLuaStringField(tbl *lua.LTable, field string) string {
	if tbl == nil {
		return ""
	}
	return strings.TrimSpace(tbl.RawGetString(field).String())
}

func luaArgOffsetMaybeSelf(L *lua.LState, self lua.LValue) int {
	if self != nil && L.GetTop() > 0 && L.Get(1) == self {
		return 1
	}
	return 0
}

func luaValueArgMaybeSelf(L *lua.LState, self lua.LValue, ordinal int) lua.LValue {
	idx := luaArgOffsetMaybeSelf(L, self) + ordinal
	if idx <= 0 || idx > L.GetTop() {
		return lua.LNil
	}
	return L.Get(idx)
}

func luaStringArgMaybeSelf(L *lua.LState, self lua.LValue, ordinal int) string {
	idx := luaArgOffsetMaybeSelf(L, self) + ordinal
	return L.CheckString(idx)
}

func luaCollectionValues(value lua.LValue) []lua.LValue {
	switch v := value.(type) {
	case *lua.LTable:
		out := make([]lua.LValue, 0, v.Len())
		for i := 1; i <= v.Len(); i++ {
			out = append(out, v.RawGetInt(i))
		}
		return out
	case *lua.LNilType:
		return nil
	default:
		return []lua.LValue{value}
	}
}

func luaValueAsBool(value lua.LValue) bool {
	switch v := value.(type) {
	case lua.LBool:
		return bool(v)
	case lua.LNumber:
		return float64(v) != 0
	case lua.LString:
		switch strings.ToLower(strings.TrimSpace(v.String())) {
		case "", "0", "false", "no", "off", "nil", "null":
			return false
		default:
			return true
		}
	case *lua.LNilType:
		return false
	default:
		return strings.TrimSpace(value.String()) != ""
	}
}

func domNodeFromValue(value lua.LValue) *hdoujinDOMNode {
	tbl, ok := value.(*lua.LTable)
	if !ok {
		return nil
	}
	ud, ok := tbl.RawGetString("__tanuki_dom_node").(*lua.LUserData)
	if !ok {
		return nil
	}
	node, _ := ud.Value.(*hdoujinDOMNode)
	return node
}

func firstString(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func domDocumentTitle(root *html.Node) string {
	if root == nil {
		return ""
	}
	nodes, err := htmlquery.QueryAll(root, "//title")
	if err != nil || len(nodes) == 0 {
		return ""
	}
	return strings.TrimSpace(htmlstd.UnescapeString(htmlquery.InnerText(nodes[0])))
}

func looksLikeDirectMediaURL(raw string) bool {
	u, err := urlpkg.Parse(raw)
	if err != nil {
		return false
	}
	switch strings.ToLower(filepath.Ext(u.Path)) {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp", ".avif", ".mp4", ".webm", ".mkv", ".mov":
		return true
	default:
		return false
	}
}

func mustURL(raw string) *urlpkg.URL {
	u, err := urlpkg.Parse(raw)
	if err != nil {
		return nil
	}
	return u
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
