package downloader

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	urlpkg "net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/antchfx/htmlquery"
	lua "github.com/yuin/gopher-lua"
	"golang.org/x/net/html"

	"github.com/nacl-dev/tanuki/internal/models"
)

type hdoujinJSONNode struct {
	runtime *hdoujinRuntime
	value   any
}

type hdoujinJSONPathStep struct {
	Key      string
	AnyIndex bool
	Index    int
	HasIndex bool
}

func (rt *hdoujinRuntime) newModuleSettingsTable() *lua.LTable {
	tbl := rt.L.NewTable()
	rt.L.SetField(tbl, "AddCheck", rt.L.NewFunction(func(L *lua.LState) int {
		key := strings.TrimSpace(luaStringArgMaybeSelf(L, tbl, 1))
		defaultValue := luaValueArgMaybeSelf(L, tbl, 2)
		if key != "" {
			tbl.RawSetString(key, lua.LBool(luaValueAsBool(defaultValue)))
		}
		return 0
	}))
	rt.L.SetField(tbl, "AddText", rt.L.NewFunction(func(L *lua.LState) int {
		key := strings.TrimSpace(luaStringArgMaybeSelf(L, tbl, 1))
		if key != "" {
			tbl.RawSetString(key, lua.LString(luaValueArgMaybeSelf(L, tbl, 2).String()))
		}
		return 0
	}))
	return tbl
}

func (rt *hdoujinRuntime) newHTTPTable(headersTbl, postDataTbl *lua.LTable) *lua.LTable {
	httpTbl := rt.L.NewTable()
	cookiesTbl := rt.L.NewTable()
	rt.L.SetField(cookiesTbl, "Contains", rt.L.NewFunction(func(L *lua.LState) int {
		name := strings.TrimSpace(luaStringArgMaybeSelf(L, cookiesTbl, 1))
		if name == "" || rt.client == nil || rt.client.Jar == nil {
			L.Push(lua.LBool(false))
			return 1
		}
		u, err := urlpkg.Parse(rt.currentURL)
		if err != nil {
			L.Push(lua.LBool(false))
			return 1
		}
		found := false
		for _, cookie := range rt.client.Jar.Cookies(u) {
			if cookie != nil && cookie.Name == name {
				found = true
				break
			}
		}
		L.Push(lua.LBool(found))
		return 1
	}))

	rt.L.SetField(httpTbl, "Headers", headersTbl)
	rt.L.SetField(httpTbl, "Cookies", cookiesTbl)
	rt.L.SetField(httpTbl, "PostData", postDataTbl)
	rt.L.SetField(httpTbl, "Get", rt.L.NewFunction(func(L *lua.LState) int {
		response, err := rt.doHTTPRequest(context.Background(), http.MethodGet, luaStringArgMaybeSelf(L, httpTbl, 1), headersTbl, nil, lua.LNil)
		if err != nil {
			L.RaiseError("%s", err.Error())
			return 0
		}
		L.Push(lua.LString(response.Body))
		return 1
	}))
	rt.L.SetField(httpTbl, "GetResponse", rt.L.NewFunction(func(L *lua.LState) int {
		response, err := rt.doHTTPRequest(context.Background(), http.MethodGet, luaStringArgMaybeSelf(L, httpTbl, 1), headersTbl, nil, lua.LNil)
		if err != nil {
			L.RaiseError("%s", err.Error())
			return 0
		}
		L.Push(rt.newHTTPResponseValue(response))
		return 1
	}))
	rt.L.SetField(httpTbl, "Post", rt.L.NewFunction(func(L *lua.LState) int {
		response, err := rt.doHTTPRequest(context.Background(), http.MethodPost, luaStringArgMaybeSelf(L, httpTbl, 1), headersTbl, postDataTbl, luaValueArgMaybeSelf(L, httpTbl, 2))
		if err != nil {
			L.RaiseError("%s", err.Error())
			return 0
		}
		postDataTbl.ForEach(func(key, _ lua.LValue) {
			if key.Type() == lua.LTString {
				postDataTbl.RawSetString(key.String(), lua.LNil)
			}
		})
		L.Push(lua.LString(response.Body))
		return 1
	}))
	rt.L.SetField(httpTbl, "PostResponse", rt.L.NewFunction(func(L *lua.LState) int {
		response, err := rt.doHTTPRequest(context.Background(), http.MethodPost, luaStringArgMaybeSelf(L, httpTbl, 1), headersTbl, postDataTbl, luaValueArgMaybeSelf(L, httpTbl, 2))
		if err != nil {
			L.RaiseError("%s", err.Error())
			return 0
		}
		postDataTbl.ForEach(func(key, _ lua.LValue) {
			if key.Type() == lua.LTString {
				postDataTbl.RawSetString(key.String(), lua.LNil)
			}
		})
		L.Push(rt.newHTTPResponseValue(response))
		return 1
	}))
	return httpTbl
}

func (rt *hdoujinRuntime) newHTTPResponseValue(response hdoujinHTTPResponse) *lua.LTable {
	tbl := rt.L.NewTable()
	rt.L.SetField(tbl, "StatusCode", lua.LNumber(response.StatusCode))
	rt.L.SetField(tbl, "Body", lua.LString(response.Body))
	cookies := make([]lua.LValue, 0, len(response.Cookies))
	for _, cookie := range response.Cookies {
		if cookie == nil {
			continue
		}
		entry := rt.L.NewTable()
		rt.L.SetField(entry, "Name", lua.LString(cookie.Name))
		rt.L.SetField(entry, "Value", lua.LString(cookie.Value))
		rt.L.SetField(entry, "Domain", lua.LString(cookie.Domain))
		rt.L.SetField(entry, "Path", lua.LString(cookie.Path))
		cookies = append(cookies, entry)
	}
	rt.L.SetField(tbl, "Cookies", rt.newCollection(cookies))
	return tbl
}

func (rt *hdoujinRuntime) newChapterInfoFactory() *lua.LTable {
	tbl := rt.L.NewTable()
	rt.L.SetField(tbl, "New", rt.L.NewFunction(func(L *lua.LState) int {
		L.Push(rt.L.NewTable())
		return 1
	}))
	return tbl
}

func (rt *hdoujinRuntime) newPageInfoFactory() *lua.LTable {
	tbl := rt.L.NewTable()
	rt.L.SetField(tbl, "New", rt.L.NewFunction(func(L *lua.LState) int {
		entry := rt.L.NewTable()
		if value := strings.TrimSpace(luaStringArgMaybeSelf(L, tbl, 1)); value != "" {
			rt.L.SetField(entry, "Url", lua.LString(value))
		}
		L.Push(entry)
		return 1
	}))
	return tbl
}

func (rt *hdoujinRuntime) newListFactory() *lua.LTable {
	tbl := rt.L.NewTable()
	rt.L.SetField(tbl, "New", rt.L.NewFunction(func(L *lua.LState) int {
		items := make([]lua.LValue, 0, 8)
		list := rt.newCollection(items)
		rt.L.SetField(list, "Add", rt.L.NewFunction(func(L *lua.LState) int {
			value := luaValueArgMaybeSelf(L, list, 1)
			list.Append(value)
			return 0
		}))
		returnValue := list
		L.Push(returnValue)
		return 1
	}))
	return tbl
}

func (rt *hdoujinRuntime) newDictFactory() *lua.LTable {
	tbl := rt.L.NewTable()
	rt.L.SetField(tbl, "New", rt.L.NewFunction(func(L *lua.LState) int {
		entry := rt.L.NewTable()
		rt.L.SetField(entry, "ContainsKey", rt.L.NewFunction(func(L *lua.LState) int {
			key := luaValueArgMaybeSelf(L, entry, 1)
			L.Push(lua.LBool(entry.RawGet(key) != lua.LNil))
			return 1
		}))
		L.Push(entry)
		return 1
	}))
	return tbl
}

func (rt *hdoujinRuntime) newDOMFactory() *lua.LTable {
	tbl := rt.L.NewTable()
	rt.L.SetField(tbl, "New", rt.L.NewFunction(func(L *lua.LState) int {
		fragment := strings.TrimSpace(luaStringArgMaybeSelf(L, tbl, 1))
		node, err := parseHTMLFragment(fragment)
		if err != nil {
			L.RaiseError("Dom.New parse failed: %v", err)
			return 0
		}
		L.Push(rt.newDOMValue(node))
		return 1
	}))
	return tbl
}

func (rt *hdoujinRuntime) newJSONFactory() *lua.LTable {
	tbl := rt.L.NewTable()
	rt.L.SetField(tbl, "New", rt.L.NewFunction(func(L *lua.LState) int {
		arg := luaValueArgMaybeSelf(L, tbl, 1)
		switch typed := arg.(type) {
		case *lua.LTable:
			L.Push(rt.newJSONValue(luaValueToGo(typed)))
			return 1
		default:
			value, err := parseJSONDocument(arg.String())
			if err != nil {
				L.RaiseError("Json.New parse failed: %v", err)
				return 0
			}
			L.Push(rt.newJSONValue(value))
			return 1
		}
	}))
	return tbl
}

func (rt *hdoujinRuntime) newJavaScriptFactory() *lua.LTable {
	tbl := rt.L.NewTable()
	rt.L.SetField(tbl, "New", rt.L.NewFunction(func(L *lua.LState) int {
		L.Push(rt.newJavaScriptValue())
		return 1
	}))
	return tbl
}

func (rt *hdoujinRuntime) newJavaScriptValue() lua.LValue {
	tbl := rt.L.NewTable()
	rt.L.SetField(tbl, "Execute", rt.L.NewFunction(func(L *lua.LState) int {
		script := luaStringArgMaybeSelf(L, tbl, 1)
		value, err := parseJavaScriptAssignedJSON(script)
		if err != nil {
			L.RaiseError("JavaScript.Execute is only supported for simple JSON assignments right now: %v", err)
			return 0
		}
		L.Push(rt.newJSONValue(value))
		return 1
	}))
	return tbl
}

func (rt *hdoujinRuntime) newJSONValue(value any) lua.LValue {
	wrapper := &hdoujinJSONNode{runtime: rt, value: value}
	tbl := rt.L.NewTable()
	ud := rt.L.NewUserData()
	ud.Value = wrapper
	rt.L.RawSet(tbl, lua.LString("__tanuki_json_node"), ud)

	rt.L.SetField(tbl, "SelectValue", rt.L.NewFunction(func(L *lua.LState) int {
		values := wrapper.selectPath(luaStringArgMaybeSelf(L, tbl, 1))
		L.Push(lua.LString(formatJSONScalar(firstJSONValue(values))))
		return 1
	}))
	rt.L.SetField(tbl, "SelectValues", rt.L.NewFunction(func(L *lua.LState) int {
		values := wrapper.selectPath(luaStringArgMaybeSelf(L, tbl, 1))
		out := make([]lua.LValue, 0, len(values))
		for _, value := range flattenJSONScalars(values) {
			out = append(out, lua.LString(formatJSONScalar(value)))
		}
		L.Push(rt.newCollection(out))
		return 1
	}))
	rt.L.SetField(tbl, "SelectNode", rt.L.NewFunction(func(L *lua.LState) int {
		values := wrapper.selectPath(luaStringArgMaybeSelf(L, tbl, 1))
		if len(values) == 0 {
			L.Push(lua.LNil)
		} else {
			L.Push(rt.newJSONValue(values[0]))
		}
		return 1
	}))
	rt.L.SetField(tbl, "SelectNodes", rt.L.NewFunction(func(L *lua.LState) int {
		values := wrapper.selectPath(luaStringArgMaybeSelf(L, tbl, 1))
		out := make([]lua.LValue, 0, len(values))
		for _, value := range values {
			out = append(out, rt.newJSONValue(value))
		}
		L.Push(rt.newCollection(out))
		return 1
	}))
	rt.L.SetField(tbl, "ToJson", rt.L.NewFunction(func(L *lua.LState) int {
		L.Push(tbl)
		return 1
	}))
	return tbl
}

func (node *hdoujinJSONNode) selectPath(path string) []any {
	steps := parseJSONPath(path)
	current := []any{node.value}
	for _, step := range steps {
		next := make([]any, 0, len(current))
		for _, item := range current {
			candidate := item
			if step.Key != "" {
				object, ok := candidate.(map[string]any)
				if !ok {
					continue
				}
				var found bool
				candidate, found = object[step.Key]
				if !found {
					continue
				}
			}

			if step.AnyIndex {
				array, ok := candidate.([]any)
				if !ok {
					continue
				}
				next = append(next, array...)
				continue
			}
			if step.HasIndex {
				array, ok := candidate.([]any)
				if !ok || step.Index < 0 || step.Index >= len(array) {
					continue
				}
				next = append(next, array[step.Index])
				continue
			}

			next = append(next, candidate)
		}
		current = next
	}
	return current
}

func parseJSONPath(path string) []hdoujinJSONPathStep {
	segments := strings.Split(strings.TrimSpace(path), ".")
	steps := make([]hdoujinJSONPathStep, 0, len(segments))
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}

		step := hdoujinJSONPathStep{Key: segment}
		if open := strings.Index(segment, "["); open >= 0 && strings.HasSuffix(segment, "]") {
			step.Key = strings.TrimSpace(segment[:open])
			indexExpr := strings.TrimSpace(segment[open+1 : len(segment)-1])
			if indexExpr == "*" {
				step.AnyIndex = true
			} else if n, err := strconv.Atoi(indexExpr); err == nil {
				step.HasIndex = true
				step.Index = n
			}
		}
		steps = append(steps, step)
	}
	return steps
}

func parseJSONDocument(raw string) (any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}, nil
	}

	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, err
	}
	return value, nil
}

func parseJavaScriptAssignedJSON(script string) (any, error) {
	script = strings.TrimSpace(script)
	if script == "" {
		return nil, fmt.Errorf("empty script")
	}

	if idx := strings.Index(script, "="); idx >= 0 {
		script = strings.TrimSpace(script[idx+1:])
	}
	script = strings.TrimSuffix(script, ";")
	return parseJSONDocument(script)
}

func parseHTMLFragment(fragment string) (*html.Node, error) {
	doc, err := html.Parse(strings.NewReader("<html><body><div>" + fragment + "</div></body></html>"))
	if err != nil {
		return nil, err
	}
	var root *html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if root != nil {
			return
		}
		if node.Type == html.ElementNode && node.Data == "div" {
			root = node
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if root == nil {
		root = &html.Node{Type: html.ElementNode, Data: "div"}
	}
	return root, nil
}

func firstJSONValue(values []any) any {
	if len(values) == 0 {
		return nil
	}
	return values[0]
}

func flattenJSONScalars(values []any) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		switch typed := value.(type) {
		case []any:
			out = append(out, flattenJSONScalars(typed)...)
		default:
			out = append(out, typed)
		}
	}
	return out
}

func formatJSONScalar(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case float64:
		if typed == math.Trunc(typed) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	default:
		body, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(body)
	}
}

func reverseStrings(values []string) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

func reverseChapters(values []hdoujinChapter) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}

func sortStringsNaturally(values []string) {
	sort.SliceStable(values, func(i, j int) bool {
		return naturalLess(values[i], values[j])
	})
}

func sortChaptersNaturally(values []hdoujinChapter) {
	sort.SliceStable(values, func(i, j int) bool {
		leftIndex, leftOK := inferExplicitWorkIndex(values[i].Title)
		rightIndex, rightOK := inferExplicitWorkIndex(values[j].Title)
		switch {
		case leftOK && rightOK && leftIndex != rightIndex:
			return leftIndex < rightIndex
		case leftOK != rightOK:
			return leftOK
		case !strings.EqualFold(values[i].Title, values[j].Title):
			return naturalLess(values[i].Title, values[j].Title)
		default:
			return naturalLess(values[i].URL, values[j].URL)
		}
	})
}

func naturalLess(left, right string) bool {
	leftParts := splitNaturalParts(strings.ToLower(strings.TrimSpace(left)))
	rightParts := splitNaturalParts(strings.ToLower(strings.TrimSpace(right)))
	for idx := 0; idx < len(leftParts) && idx < len(rightParts); idx++ {
		l := leftParts[idx]
		r := rightParts[idx]
		lNum, lErr := strconv.Atoi(l)
		rNum, rErr := strconv.Atoi(r)
		if lErr == nil && rErr == nil {
			if lNum != rNum {
				return lNum < rNum
			}
			continue
		}
		if l != r {
			return l < r
		}
	}
	return len(leftParts) < len(rightParts)
}

func splitNaturalParts(value string) []string {
	if value == "" {
		return nil
	}
	parts := make([]string, 0, 8)
	start := 0
	lastDigit := unicode.IsDigit(rune(value[0]))
	for idx, r := range value {
		if idx == 0 {
			continue
		}
		isDigit := unicode.IsDigit(r)
		if isDigit == lastDigit {
			continue
		}
		parts = append(parts, value[start:idx])
		start = idx
		lastDigit = isDigit
	}
	parts = append(parts, value[start:])
	return parts
}

func (e *HDoujinLuaEngine) downloadChapterSeries(ctx context.Context, job *models.DownloadJob, root *hdoujinRuntime, module *hdoujinLuaModule) error {
	if len(root.chapters) == 0 {
		return newUnsupportedURLError("hdoujin", "module did not yield any chapters")
	}

	seriesTitle := strings.TrimSpace(root.infoString("Title"))
	seriesTags := root.importTags()
	for idx, chapter := range root.chapters {
		chapterURL := resolveHDoujinURL(root.currentURL, chapter.URL)
		if chapterURL == "" {
			return newUnsupportedURLError("hdoujin", "module chapter url could not be resolved")
		}
		if err := e.downloadSingleChapter(ctx, job, module, seriesTitle, seriesTags, chapterURL, chapter); err != nil {
			return err
		}
		if e.progress != nil {
			e.progress(job.ID, 0, 0, idx+1, len(root.chapters))
		}
	}
	return nil
}

func (e *HDoujinLuaEngine) downloadSingleChapter(ctx context.Context, job *models.DownloadJob, module *hdoujinLuaModule, seriesTitle string, inheritedTags []string, chapterURL string, chapter hdoujinChapter) error {
	rt, err := e.newRuntime(module, chapterURL)
	if err != nil {
		return err
	}
	defer rt.Close()

	if err := rt.loadURL(ctx, chapterURL, nil); err != nil {
		return err
	}
	if err := rt.callIfPresent("GetInfo"); err != nil {
		return newUnsupportedURLError("hdoujin", "chapter GetInfo failed: "+err.Error())
	}
	if err := rt.callIfPresent("GetPages"); err != nil {
		return newUnsupportedURLError("hdoujin", "chapter GetPages failed: "+err.Error())
	}
	if len(rt.pages) == 0 {
		return newUnsupportedURLError("hdoujin", "chapter module did not yield downloadable pages")
	}

	title := strings.TrimSpace(rt.infoString("Title"))
	if title == "" {
		switch {
		case seriesTitle != "" && strings.TrimSpace(chapter.Title) != "":
			title = strings.TrimSpace(seriesTitle + " - " + chapter.Title)
		case strings.TrimSpace(chapter.Title) != "":
			title = strings.TrimSpace(chapter.Title)
		default:
			title = sanitizeArchiveName(pathBaseWithoutExt(chapterURL))
		}
	}

	archiveName := sanitizeArchiveName(title)
	if archiveName == "" {
		archiveName = "hdoujin-chapter"
	}
	if err := os.MkdirAll(job.TargetDirectory, 0o755); err != nil {
		return fmt.Errorf("hdoujin mkdir: %w", err)
	}
	finalPath := uniqueOrganizedPath(job.TargetDirectory, archiveName+".cbz")
	if err := e.writeHDoujinArchive(ctx, rt, finalPath); err != nil {
		return err
	}

	tags := mergeImportTags(inheritedTags, rt.importTags())
	metadata := models.ImportMetadata{
		Title:     title,
		WorkTitle: cleanWorkTitleCandidate(seriesTitle),
		SourceURL: chapterURL,
		Tags:      tags,
	}
	if workIndex, ok := inferExplicitWorkIndex(chapter.Title); ok {
		metadata.WorkIndex = workIndex
	} else if workIndex, ok := inferExplicitWorkIndex(title); ok {
		metadata.WorkIndex = workIndex
	}
	if err := writeImportMetadata(finalPath, metadata); err != nil {
		return fmt.Errorf("hdoujin chapter metadata: %w", err)
	}
	return nil
}

func (e *HDoujinLuaEngine) writeHDoujinArchive(ctx context.Context, rt *hdoujinRuntime, finalPath string) error {
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
			_ = os.Remove(tmpPath)
			return err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
		if err != nil {
			_ = zw.Close()
			_ = file.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("hdoujin page request %d: %w", idx+1, err)
		}
		req.Header.Set("User-Agent", "Tanuki/1.0")
		req.Header.Set("Referer", sourceBaseURL(downloadURL))

		resp, err := e.client.Do(req)
		if err != nil {
			_ = zw.Close()
			_ = file.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("hdoujin page fetch %d: %w", idx+1, err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			_ = zw.Close()
			_ = file.Close()
			_ = os.Remove(tmpPath)
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
			_ = os.Remove(tmpPath)
			return err
		}
		writer, err := zw.Create(entryName)
		if err != nil {
			resp.Body.Close()
			_ = zw.Close()
			_ = file.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("hdoujin cbz entry %d: %w", idx+1, err)
		}
		if _, err := io.Copy(writer, resp.Body); err != nil {
			resp.Body.Close()
			_ = zw.Close()
			_ = file.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("hdoujin cbz write %d: %w", idx+1, err)
		}
		resp.Body.Close()
	}

	if err := zw.Close(); err != nil {
		_ = file.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("hdoujin close cbz: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("hdoujin close file: %w", err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("hdoujin rename cbz: %w", err)
	}
	return nil
}

func resolveHDoujinURL(baseRaw, targetRaw string) string {
	targetRaw = strings.TrimSpace(targetRaw)
	if targetRaw == "" {
		return ""
	}
	target, err := urlpkg.Parse(targetRaw)
	if err != nil {
		return ""
	}
	base, err := urlpkg.Parse(strings.TrimSpace(baseRaw))
	if err == nil {
		target = base.ResolveReference(target)
	}
	return target.String()
}

type hdoujinHTTPResponse struct {
	StatusCode int
	Body       string
	Cookies    []*http.Cookie
}

func (rt *hdoujinRuntime) doHTTPRequest(ctx context.Context, method, rawURL string, headers, postData *lua.LTable, explicitBody lua.LValue) (hdoujinHTTPResponse, error) {
	target, err := urlpkg.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return hdoujinHTTPResponse{}, fmt.Errorf("hdoujin parse url: %w", err)
	}
	if base, parseErr := urlpkg.Parse(rt.currentURL); parseErr == nil {
		target = base.ResolveReference(target)
	}

	bodyReader, contentType, err := rt.httpRequestBody(postData, explicitBody)
	if err != nil {
		return hdoujinHTTPResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, method, target.String(), bodyReader)
	if err != nil {
		return hdoujinHTTPResponse{}, fmt.Errorf("hdoujin request: %w", err)
	}
	req.Header.Set("User-Agent", "Tanuki/1.0")
	req.Header.Set("Referer", sourceBaseURL(target.String()))
	if contentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", contentType)
	}
	if headers != nil {
		headers.ForEach(func(key, value lua.LValue) {
			if key.Type() == lua.LTString {
				req.Header.Set(key.String(), value.String())
			}
		})
	}

	resp, err := rt.client.Do(req)
	if err != nil {
		return hdoujinHTTPResponse{}, fmt.Errorf("hdoujin fetch: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return hdoujinHTTPResponse{}, fmt.Errorf("hdoujin read body: %w", err)
	}

	doc, parseErr := htmlquery.Parse(strings.NewReader(string(body)))
	if parseErr == nil {
		rt.currentDoc = doc
	}
	rt.currentURL = target.String()
	rt.currentBody = string(body)
	rt.L.SetGlobal("url", lua.LString(rt.currentURL))
	rt.L.SetGlobal("doc", lua.LString(rt.currentBody))
	rt.L.SetGlobal("dom", rt.newDOMValue(nil))
	if pageTbl, ok := rt.L.GetGlobal("page").(*lua.LTable); ok {
		rt.L.SetField(pageTbl, "Url", lua.LString(rt.currentURL))
	}

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusTooManyRequests {
		return hdoujinHTTPResponse{}, newUnsupportedURLError("hdoujin", blockedSourceDetail(resp.StatusCode))
	}

	return hdoujinHTTPResponse{
		StatusCode: resp.StatusCode,
		Body:       string(body),
		Cookies:    resp.Cookies(),
	}, nil
}

func (rt *hdoujinRuntime) httpRequestBody(postData *lua.LTable, explicitBody lua.LValue) (io.Reader, string, error) {
	switch typed := explicitBody.(type) {
	case *lua.LNilType:
	case lua.LString:
		return strings.NewReader(typed.String()), "application/json", nil
	case *lua.LTable:
		body, err := json.Marshal(luaValueToGo(typed))
		if err != nil {
			return nil, "", fmt.Errorf("hdoujin marshal post body: %w", err)
		}
		return bytes.NewReader(body), "application/json", nil
	default:
		if strings.TrimSpace(explicitBody.String()) != "" {
			return strings.NewReader(explicitBody.String()), "application/json", nil
		}
	}

	if postData == nil {
		return nil, "", nil
	}
	values := urlpkg.Values{}
	postData.ForEach(func(key, value lua.LValue) {
		if key.Type() != lua.LTString {
			return
		}
		switch value.Type() {
		case lua.LTFunction:
			return
		case lua.LTNil:
			return
		default:
			values.Set(key.String(), value.String())
		}
	})
	if len(values) == 0 {
		return nil, "", nil
	}
	return strings.NewReader(values.Encode()), "application/x-www-form-urlencoded", nil
}

func (rt *hdoujinRuntime) syncPagesTable() {
	if rt.pagesTbl == nil {
		return
	}
	clearTableArray(rt.pagesTbl)
	for idx, page := range rt.pages {
		rt.pagesTbl.RawSetInt(idx+1, lua.LString(page))
	}
}

func (rt *hdoujinRuntime) syncChaptersTable() {
	if rt.chaptersTbl == nil {
		return
	}
	clearTableArray(rt.chaptersTbl)
	for idx, chapter := range rt.chapters {
		entry := rt.L.NewTable()
		rt.L.SetField(entry, "Url", lua.LString(chapter.URL))
		rt.L.SetField(entry, "Title", lua.LString(chapter.Title))
		rt.chaptersTbl.RawSetInt(idx+1, entry)
	}
}

func clearTableArray(tbl *lua.LTable) {
	for idx := tbl.Len(); idx >= 1; idx-- {
		tbl.RawSetInt(idx, lua.LNil)
	}
}

func (rt *hdoujinRuntime) setCookies(value lua.LValue) {
	if rt.client == nil || rt.client.Jar == nil {
		return
	}
	u, err := urlpkg.Parse(rt.currentURL)
	if err != nil {
		return
	}
	cookies := make([]*http.Cookie, 0, 4)
	for _, item := range luaCollectionValues(value) {
		table, ok := item.(*lua.LTable)
		if !ok {
			continue
		}
		name := strings.TrimSpace(firstNonEmpty(table.RawGetString("Name").String(), table.RawGetString("name").String()))
		if name == "" {
			continue
		}
		cookies = append(cookies, &http.Cookie{
			Name:   name,
			Value:  firstNonEmpty(table.RawGetString("Value").String(), table.RawGetString("value").String()),
			Domain: firstNonEmpty(table.RawGetString("Domain").String(), u.Hostname()),
			Path:   firstNonEmpty(table.RawGetString("Path").String(), "/"),
		})
	}
	if len(cookies) > 0 {
		rt.client.Jar.SetCookies(u, cookies)
	}
}

func luaValueToGo(value lua.LValue) any {
	switch typed := value.(type) {
	case *lua.LNilType:
		return nil
	case lua.LBool:
		return bool(typed)
	case lua.LNumber:
		return float64(typed)
	case lua.LString:
		return typed.String()
	case *lua.LFunction:
		return nil
	case *lua.LTable:
		maxIndex := typed.Len()
		if maxIndex > 0 {
			array := make([]any, 0, maxIndex)
			sequential := true
			for idx := 1; idx <= maxIndex; idx++ {
				item := typed.RawGetInt(idx)
				if item == lua.LNil {
					sequential = false
					break
				}
				if _, isFunc := item.(*lua.LFunction); isFunc {
					continue
				}
				array = append(array, luaValueToGo(item))
			}
			if sequential {
				return array
			}
		}
		object := map[string]any{}
		typed.ForEach(func(key, value lua.LValue) {
			if key.Type() == lua.LTString {
				if _, isFunc := value.(*lua.LFunction); isFunc {
					return
				}
				object[key.String()] = luaValueToGo(value)
			}
		})
		return object
	default:
		return typed.String()
	}
}
