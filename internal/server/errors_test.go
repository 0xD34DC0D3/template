package server

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/0xD34DC0D3/voodoo/voodooauth"

	"github.com/0xD34DC0D3/template/internal/web"
)

// Error responses come through the layout as HTML and are never cacheable,
// signed in or not.
func TestErrorPages(t *testing.T) {
	h := newTestHandler(t)
	tests := []struct {
		name, method, path string
		status             int
	}{
		{"unknown path", http.MethodGet, "/nope", http.StatusNotFound},
		{"unknown nested path", http.MethodGet, "/admin/extra", http.StatusNotFound},
		// The catch-all has no method, so a wrong method is a 404 too.
		{"wrong method", http.MethodGet, "/notes", http.StatusNotFound},
	}
	for _, tc := range tests {
		for _, cookie := range []string{"", "user"} {
			t.Run(tc.name+" "+cookie, func(t *testing.T) {
				req := httptest.NewRequest(tc.method, tc.path, nil)
				if cookie != "" {
					withCookie(req, cookie)
				}
				rec := do(h, req)
				checkErrorPage(t, rec, tc.status)
			})
		}
	}
	t.Run("bad note", func(t *testing.T) {
		checkErrorPage(t, do(h, postNote("", nil)), http.StatusBadRequest)
	})
	t.Run("too large", func(t *testing.T) {
		checkErrorPage(t, do(h, postNote(strings.Repeat("a", maxBody+1), nil)), http.StatusRequestEntityTooLarge)
	})
}

func checkErrorPage(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status %d, want %d", rec.Code, want)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	body := rec.Body.String()
	for _, w := range []string{"<html", "<h1>" + http.StatusText(want) + "</h1>", `<nav aria-label="Main">`, `href="/">Back to ` + appName + `</a>`} {
		if !strings.Contains(body, w) {
			t.Errorf("missing %q in %s", w, body)
		}
	}
}

func TestInternalErrorPage(t *testing.T) {
	h, d := newTestServer(t, nil)
	if err := d.Reader.Close(); err != nil {
		t.Fatal(err)
	}
	checkErrorPage(t, do(h, httptest.NewRequest(http.MethodGet, "/", nil)), http.StatusInternalServerError)
}

// A recovered panic gets the layout's 500 page.
func TestPanicPage(t *testing.T) {
	views, err := fs.Sub(viewsFS, "views")
	if err != nil {
		t.Fatal(err)
	}
	s := &server{web: web.New(appName, testSiteURL, views, voodooauth.New("http://voodoo.invalid"))}
	h := chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }),
		requestLogger, securityHeaders, recoverer(s.panicPage))
	checkErrorPage(t, do(h, httptest.NewRequest(http.MethodGet, "/", nil)), http.StatusInternalServerError)
}

// htmx does not swap 4xx, so a rejected htmx post is a 200 fragment that
// carries the error and keeps the input.
func TestHTMXValidationError(t *testing.T) {
	h := newTestHandler(t)
	long := strings.Repeat("x", maxNoteRunes+1)
	rec := do(h, postNote(long, map[string]string{"HX-Request": "true"}))
	body := rec.Body.String()
	if rec.Code != http.StatusOK || strings.Contains(body, "<html") {
		t.Fatalf("got %d, want a 200 fragment: %s", rec.Code, body)
	}
	if !strings.Contains(body, `role="alert"`) || !strings.Contains(body, `value="`+long+`"`) {
		t.Errorf("error or input missing: %s", body)
	}
}

func TestNavCurrent(t *testing.T) {
	h := newTestHandler(t)
	tests := []struct {
		path, cookie string
		want         string
		not          []string
	}{
		{"/", "", `<a href="/" aria-current="page">notes</a>`, []string{`href="/admin"`}},
		{"/admin", "admin", `<a href="/admin" aria-current="page">admin</a>`, []string{`href="/" aria-current`}},
		{"/nope", "", `<nav aria-label="Main">`, []string{"aria-current"}},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.cookie != "" {
				withCookie(req, tc.cookie)
			}
			body := do(h, req).Body.String()
			if !strings.Contains(body, tc.want) {
				t.Errorf("missing %q in %s", tc.want, body)
			}
			for _, n := range tc.not {
				if strings.Contains(body, n) {
					t.Errorf("unexpected %q", n)
				}
			}
		})
	}
}
