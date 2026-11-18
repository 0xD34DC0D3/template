package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/0xD34DC0D3/voodoo/voodooauth"
)

func TestRenderErrorIsCleanly500(t *testing.T) {
	// Fails midway through execution, after some output was produced.
	pages := fstest.MapFS{"p.html": {Data: []byte(`{{define "p"}}partial {{.Data.Missing.Field}}{{end}}`)}}
	rec := httptest.NewRecorder()
	// A public page sets its cache policy before rendering; the 500 must
	// not inherit it.
	rec.Header().Set("Cache-Control", "public, max-age=60")
	New("app", "https://site.example", pages, voodooauth.New("http://voodoo.invalid")).Render(rec, httptest.NewRequest(http.MethodGet, "/", nil), View{Page: "p", Data: struct{ Missing *struct{ Field int } }{}})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "partial") {
		t.Errorf("partial output leaked: %q", rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	if rec.Header().Get("Vary") != "HX-Request" {
		t.Errorf("Vary = %q", rec.Header().Get("Vary"))
	}
}
