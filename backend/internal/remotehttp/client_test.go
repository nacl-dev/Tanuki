package remotehttp

import "testing"

func TestValidateURLAllowsPublicHTTPSHosts(t *testing.T) {
	if err := ValidateURL("https://example.com/image.jpg"); err != nil {
		t.Fatalf("expected public https URL to be allowed: %v", err)
	}
}

func TestValidateURLRejectsLocalhost(t *testing.T) {
	if err := ValidateURL("http://localhost:8080/internal"); err == nil {
		t.Fatal("expected localhost to be rejected")
	}
}

func TestValidateURLRejectsPrivateIP(t *testing.T) {
	if err := ValidateURL("http://192.168.0.10/file.jpg"); err == nil {
		t.Fatal("expected private IP to be rejected")
	}
}

func TestValidateURLRejectsUnsupportedSchemes(t *testing.T) {
	if err := ValidateURL("file:///tmp/test.jpg"); err == nil {
		t.Fatal("expected file URL to be rejected")
	}
}
