package downloader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuditHDoujinModuleExtractsDomainsAndFlags(t *testing.T) {
	t.Parallel()

	body := `
function Register()
  module.Name = 'FAKKU!'
  module.Language = 'English'
  module.Domains.Add('fakku.net')
  module.Domains.Add("www.fakku.net")
end

function GetInfo()
end
`

	audit := AuditHDoujinModule("Fakku.lua", body)
	if audit.DisplayName != "FAKKU!" {
		t.Fatalf("unexpected display name: %q", audit.DisplayName)
	}
	if len(audit.Domains) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(audit.Domains))
	}
	if audit.SuggestedEngine != "*downloader.GalleryDLEngine" || audit.Coverage != "gallery-dl-fallback" {
		t.Fatalf("unexpected coverage: %+v", audit)
	}
	if audit.RequiresLogin || audit.UsesJavaScript || audit.UsesEncryption {
		t.Fatalf("unexpected runtime flags: %+v", audit)
	}
}

func TestAuditHDoujinModuleFlagsEncryptedRuntime(t *testing.T) {
	t.Parallel()

	body := `
DoEncryptedString("abc")
function Login()
end
`

	audit := AuditHDoujinModule("Hitomi.lua", body)
	if !audit.UsesEncryption || !audit.RequiresLogin {
		t.Fatalf("expected encrypted login module, got %+v", audit)
	}
	if audit.Coverage != "gallery-dl-fallback" {
		t.Fatalf("expected gallery-dl fallback for hitomi by module name, got %+v", audit)
	}
}

func TestAuditHDoujinModulesDirSummarizesModules(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Gelbooru.lua"), []byte(`
function Register()
  module.Domains.Add('gelbooru.com')
end
function GetInfo()
end
function GetPages()
end
`), 0o644); err != nil {
		t.Fatalf("write gelbooru fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Hitomi.lua"), []byte(`
DoEncryptedString("abc")
function Login()
end
`), 0o644); err != nil {
		t.Fatalf("write hitomi fixture: %v", err)
	}

	report, err := AuditHDoujinModulesDir(dir)
	if err != nil {
		t.Fatalf("audit modules dir: %v", err)
	}
	if report.Summary.TotalModules != 2 {
		t.Fatalf("expected 2 modules, got %d", report.Summary.TotalModules)
	}
	if report.Summary.NativeMatches != 1 || report.Summary.GalleryDLFallbacks != 1 {
		t.Fatalf("unexpected summary: %+v", report.Summary)
	}
}
