package renderer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glincker/stacklit/internal/schema"
)

func renderTestPage(t *testing.T, remote bool) string {
	t.Helper()
	SetMapIcons(remote)
	t.Cleanup(func() { SetMapIcons(true) })
	p := filepath.Join(t.TempDir(), "page.html")
	idx := &schema.Index{Modules: map[string]schema.ModuleInfo{"a": {Files: 1, Lines: 3}}}
	if err := WriteHTML(idx, p); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestMapIconsOffHasNoRemoteURL(t *testing.T) {
	// The SVG namespace string is an identifier, not a request.
	page := strings.ReplaceAll(renderTestPage(t, false), "http://www.w3.org/2000/svg", "")
	for _, host := range []string{"thesvg.org", "jsdelivr.net", "http://", "https://"} {
		if strings.Contains(page, host) {
			t.Errorf("page with icons off contains %q", host)
		}
	}
	if !strings.Contains(page, "Content-Security-Policy") {
		t.Error("page is missing its Content-Security-Policy")
	}
	if !strings.Contains(page, "connect-src 'none'") {
		t.Error("CSP must block network connections")
	}
}

func TestMapIconsRemoteIsLimitedByCSP(t *testing.T) {
	page := renderTestPage(t, true)
	if !strings.Contains(page, "img-src data: https://thesvg.org https://cdn.jsdelivr.net;") {
		t.Error("CSP should allow images only from the two icon hosts")
	}
	if strings.Contains(page, "{{") {
		t.Error("template placeholder left in page")
	}
	if !strings.Contains(page, "default-src 'none'") {
		t.Error("CSP must start from default-src 'none'")
	}
}
