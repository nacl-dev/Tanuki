package downloader

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type HDoujinModuleAudit struct {
	Module          string   `json:"module"`
	DisplayName     string   `json:"display_name,omitempty"`
	Language        string   `json:"language,omitempty"`
	Domains         []string `json:"domains,omitempty"`
	HasGetInfo      bool     `json:"has_get_info"`
	HasGetChapters  bool     `json:"has_get_chapters"`
	HasGetPages     bool     `json:"has_get_pages"`
	RequiresLogin   bool     `json:"requires_login"`
	UsesJavaScript  bool     `json:"uses_javascript"`
	UsesEncryption  bool     `json:"uses_encryption"`
	SuggestedEngine string   `json:"suggested_engine,omitempty"`
	Coverage        string   `json:"coverage"`
	Priority        string   `json:"priority"`
	Notes           []string `json:"notes,omitempty"`
}

type HDoujinAuditSummary struct {
	TotalModules       int `json:"total_modules"`
	NativeMatches      int `json:"native_matches"`
	GalleryDLFallbacks int `json:"gallery_dl_fallbacks"`
	ReviewCandidates   int `json:"review_candidates"`
	EncryptedModules   int `json:"encrypted_modules"`
	LoginModules       int `json:"login_modules"`
	JavaScriptModules  int `json:"javascript_modules"`
}

type HDoujinAuditReport struct {
	Summary HDoujinAuditSummary  `json:"summary"`
	Modules []HDoujinModuleAudit `json:"modules"`
}

var (
	hdoujinSingleQuotedValueRe = regexp.MustCompile(`(?m)module\.(Name|Language)\s*=\s*'([^']+)'`)
	hdoujinDoubleQuotedValueRe = regexp.MustCompile(`(?m)module\.(Name|Language)\s*=\s*"([^"]+)"`)
	hdoujinSingleDomainRe      = regexp.MustCompile(`(?m)module\.Domains\.Add\('([^']+)'\)`)
	hdoujinDoubleDomainRe      = regexp.MustCompile(`(?m)module\.Domains\.Add\("([^"]+)"\)`)
	hdoujinLoginRe             = regexp.MustCompile(`(?m)function\s+Login\s*\(`)
	hdoujinJSRe                = regexp.MustCompile(`(?m)JavaScript\.New\s*\(`)
	hdoujinEncryptedRe         = regexp.MustCompile(`(?m)DoEncryptedString\s*\(`)
	hdoujinGetInfoRe           = regexp.MustCompile(`(?m)function\s+GetInfo\s*\(`)
	hdoujinGetChaptersRe       = regexp.MustCompile(`(?m)function\s+GetChapters\s*\(`)
	hdoujinGetPagesRe          = regexp.MustCompile(`(?m)function\s+GetPages\s*\(`)
)

func AuditHDoujinModule(moduleFile, body string) HDoujinModuleAudit {
	audit := HDoujinModuleAudit{
		Module:         filepath.Base(moduleFile),
		HasGetInfo:     hdoujinGetInfoRe.MatchString(body),
		HasGetChapters: hdoujinGetChaptersRe.MatchString(body),
		HasGetPages:    hdoujinGetPagesRe.MatchString(body),
		RequiresLogin:  hdoujinLoginRe.MatchString(body),
		UsesJavaScript: hdoujinJSRe.MatchString(body),
		UsesEncryption: hdoujinEncryptedRe.MatchString(body),
	}

	for _, match := range hdoujinSingleQuotedValueRe.FindAllStringSubmatch(body, -1) {
		switch match[1] {
		case "Name":
			audit.DisplayName = strings.TrimSpace(match[2])
		case "Language":
			audit.Language = strings.TrimSpace(match[2])
		}
	}
	for _, match := range hdoujinDoubleQuotedValueRe.FindAllStringSubmatch(body, -1) {
		switch match[1] {
		case "Name":
			if audit.DisplayName == "" {
				audit.DisplayName = strings.TrimSpace(match[2])
			}
		case "Language":
			if audit.Language == "" {
				audit.Language = strings.TrimSpace(match[2])
			}
		}
	}

	domainSeen := map[string]struct{}{}
	for _, match := range hdoujinSingleDomainRe.FindAllStringSubmatch(body, -1) {
		domain := strings.ToLower(strings.TrimSpace(match[1]))
		if domain == "" {
			continue
		}
		if _, ok := domainSeen[domain]; ok {
			continue
		}
		domainSeen[domain] = struct{}{}
		audit.Domains = append(audit.Domains, domain)
	}
	for _, match := range hdoujinDoubleDomainRe.FindAllStringSubmatch(body, -1) {
		domain := strings.ToLower(strings.TrimSpace(match[1]))
		if domain == "" {
			continue
		}
		if _, ok := domainSeen[domain]; ok {
			continue
		}
		domainSeen[domain] = struct{}{}
		audit.Domains = append(audit.Domains, domain)
	}
	sort.Strings(audit.Domains)

	audit.SuggestedEngine, audit.Coverage, audit.Priority, audit.Notes = classifyHDoujinModule(audit)
	return audit
}

func AuditHDoujinModulesDir(modulesDir string) (HDoujinAuditReport, error) {
	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		return HDoujinAuditReport{}, err
	}

	report := HDoujinAuditReport{
		Modules: make([]HDoujinModuleAudit, 0, len(entries)),
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".lua") {
			continue
		}
		path := filepath.Join(modulesDir, entry.Name())
		body, err := os.ReadFile(path)
		if err != nil {
			return HDoujinAuditReport{}, err
		}
		report.Modules = append(report.Modules, AuditHDoujinModule(entry.Name(), string(body)))
	}

	sort.Slice(report.Modules, func(i, j int) bool {
		if report.Modules[i].Priority == report.Modules[j].Priority {
			return report.Modules[i].Module < report.Modules[j].Module
		}
		return report.Modules[i].Priority < report.Modules[j].Priority
	})

	for _, module := range report.Modules {
		report.Summary.TotalModules++
		switch module.Coverage {
		case "native-engine":
			report.Summary.NativeMatches++
		case "gallery-dl-fallback":
			report.Summary.GalleryDLFallbacks++
		case "manual-review":
			report.Summary.ReviewCandidates++
		}
		if module.UsesEncryption {
			report.Summary.EncryptedModules++
		}
		if module.RequiresLogin {
			report.Summary.LoginModules++
		}
		if module.UsesJavaScript {
			report.Summary.JavaScriptModules++
		}
	}

	return report, nil
}

func (r HDoujinAuditReport) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

func classifyHDoujinModule(audit HDoujinModuleAudit) (engine, coverage, priority string, notes []string) {
	if audit.UsesEncryption {
		notes = append(notes, "module body is encrypted/obfuscated")
	}
	if audit.RequiresLogin {
		notes = append(notes, "site requires login flow")
	}
	if audit.UsesJavaScript {
		notes = append(notes, "module relies on embedded JavaScript execution")
	}

	for _, domain := range audit.Domains {
		switch domain {
		case "danbooru.donmai.us":
			return "*downloader.DanbooruEngine", "native-engine", "native-now", notes
		case "gelbooru.com", "safebooru.org":
			return "*downloader.BooruEngine", "native-engine", "native-now", notes
		case "fakku.net", "www.fakku.net", "nhentai.net", "hitomi.la":
			return "*downloader.GalleryDLEngine", "gallery-dl-fallback", "fallback-first", notes
		}
	}

	lowerModule := strings.ToLower(strings.TrimSuffix(audit.Module, filepath.Ext(audit.Module)))
	switch lowerModule {
	case "fakku", "nhentai":
		return "*downloader.GalleryDLEngine", "gallery-dl-fallback", "fallback-first", notes
	case "hitomi":
		if audit.UsesEncryption {
			notes = append(notes, "prefer fixture/reference use instead of runtime reuse")
		}
		return "*downloader.GalleryDLEngine", "gallery-dl-fallback", "fallback-first", notes
	case "danbooru":
		return "*downloader.DanbooruEngine", "native-engine", "native-now", notes
	case "gelbooru":
		return "*downloader.BooruEngine", "native-engine", "native-now", notes
	}

	if audit.UsesEncryption || audit.RequiresLogin || audit.UsesJavaScript {
		return "", "manual-review", "manual-review", notes
	}
	if len(audit.Domains) == 0 {
		notes = append(notes, "no domains discovered from static parse")
		return "", "manual-review", "manual-review", notes
	}

	return "", "manual-review", "manual-review", notes
}

func CoverageSummary(report HDoujinAuditReport) []string {
	summary := []string{
		"total_modules=" + itoa(report.Summary.TotalModules),
		"native_matches=" + itoa(report.Summary.NativeMatches),
		"gallery_dl_fallbacks=" + itoa(report.Summary.GalleryDLFallbacks),
		"review_candidates=" + itoa(report.Summary.ReviewCandidates),
		"encrypted_modules=" + itoa(report.Summary.EncryptedModules),
	}
	return summary
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
