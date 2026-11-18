// Package web holds the shared HTML chrome (layout template, stylesheet,
// fonts, vendored htmx) and the one helper that renders a page.
//
// Vendored: static/htmx.min.js is htmx 2.0.11 from
// https://cdn.jsdelivr.net/npm/htmx.org@2.0.11/dist/htmx.min.js
// sha256 d6fdc75f204e6bdefa99b69bf1e6d4ac69b8a364f77929f45c13476b4000f717
// To upgrade: download the new file, update this comment, run the tests.
package web

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/0xD34DC0D3/voodoo/voodooauth"
)

//go:embed layout.html
var layoutFS embed.FS

//go:embed static
var staticFS embed.FS

// static is rooted so "style.css" is a top-level entry, ready to mount under
// "/static/".
var static = func() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return sub
}()

// version fingerprints the static files. Pages link them as ?v=<version>, so
// a request carrying it can be cached forever: a changed file changes the URL.
var version = func() string {
	h := sha256.New()
	err := fs.WalkDir(static, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(static, p)
		h.Write([]byte(p))
		h.Write(b)
		return err
	})
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(h.Sum(nil))[:10]
}()

// StaticHandler serves the embedded static files; mount it at "/static/".
// Versioned URLs (?v=) are immutable. Unversioned ones (fonts, referenced
// from the CSS) get a short cache so a changed file still reaches readers.
func StaticHandler() http.Handler {
	files := http.StripPrefix("/static/", http.FileServerFS(static))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("v") != "" {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		files.ServeHTTP(w, r)
	})
}

// Renderer renders page templates inside the shared layout.
type Renderer struct {
	t       *template.Template
	app     string
	siteURL string
	auth    *voodooauth.Client
}

// New parses the layout plus every *.html in pages into one template set, so
// feature packages embed only their own templates. Templates are program
// text, so a parse error panics at startup. siteURL is the org website the
// header tile and footer link to (APPNAME_SITE_URL, no trailing slash); auth
// supplies the login and logout links in the header.
func New(app, siteURL string, pages fs.FS, auth *voodooauth.Client) *Renderer {
	t := template.Must(template.ParseFS(layoutFS, "layout.html"))
	return &Renderer{t: template.Must(t.ParseFS(pages, "*.html")), app: app, siteURL: siteURL, auth: auth}
}

// View describes one response. Page is the full-page template, Fragment the
// block that alone replaces part of the page for an htmx request. Status is
// the HTTP status, 200 when zero.
//
// Nav names the header link marked aria-current (see layout.html); NavSub
// says the page sits below that link's page (/things/1 under /things), so
// the link gets aria-current="true" instead of "page".
// Wide opts into main.wide, for table-heavy pages.
type View struct {
	Page     string
	Fragment string
	Title    string
	Data     any
	Status   int
	Nav      string
	NavSub   bool
	Wide     bool
}

// page is what templates see: layout fields plus the handler's Data. User
// is nil when signed out; every page can show who is signed in. Partial is
// true when only the fragment is rendered (an htmx request).
type page struct {
	Title, App, SiteURL, Ver string
	User                     *voodooauth.User
	LoginURL, LogoutURL      string
	Data                     any
	Nav                      string
	NavSub, Wide, Partial    bool
}

// Current is the aria-current value for the header link named nav, or ""
// when that link is not the current page or section.
func (p page) Current(nav string) string {
	switch {
	case p.Nav != nav || nav == "":
		return ""
	case p.NavSub:
		return "true"
	default:
		return "page"
	}
}

// Render writes the full page, or just the fragment when the request has
// HX-Request: true. Vary is always set so a cache never serves one to the
// other. The template runs into a buffer first so a failure is a clean 500
// instead of a half-written page.
func (r *Renderer) Render(w http.ResponseWriter, req *http.Request, v View) {
	w.Header().Add("Vary", "HX-Request")
	name, partial := v.Page, false
	if v.Fragment != "" && req.Header.Get("HX-Request") == "true" {
		name, partial = v.Fragment, true
	}
	var buf bytes.Buffer
	p := page{
		Title: v.Title, App: r.app, SiteURL: r.siteURL, Ver: version, Data: v.Data,
		User:      voodooauth.UserFrom(req.Context()),
		LoginURL:  r.auth.LoginURL(req),
		LogoutURL: r.auth.LogoutURL(),
		Nav:       v.Nav, NavSub: v.NavSub, Wide: v.Wide, Partial: partial,
	}
	if err := r.t.ExecuteTemplate(&buf, name, p); err != nil {
		slog.Error("render", "template", name, "err", err)
		// The handler may have marked the page cacheable; an error must not be.
		w.Header().Set("Cache-Control", "no-store")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if v.Status != 0 {
		w.WriteHeader(v.Status)
	}
	_, _ = buf.WriteTo(w)
}

// Error renders the layout's error page: status, its standard text as the
// heading, and msg below it. It is no-store, replacing whatever the handler
// set: the page names the signed-in user, and only public pages Vary on
// Cookie, so no cache may keep it. htmx requests get the full page too; htmx
// does not swap 4xx or 5xx responses by default anyway.
func (r *Renderer) Error(w http.ResponseWriter, req *http.Request, status int, msg string) {
	w.Header().Set("Cache-Control", "no-store")
	r.Render(w, req, View{Page: "error", Title: http.StatusText(status), Data: msg, Status: status})
}
