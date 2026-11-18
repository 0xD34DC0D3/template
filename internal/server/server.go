// Package server builds appname's HTTP handler: routes plus the middleware stack.
package server

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/0xD34DC0D3/voodoo/voodooauth"
	"github.com/inngest/inngestgo"

	"github.com/0xD34DC0D3/template/internal/db"
	"github.com/0xD34DC0D3/template/internal/jobs"
	"github.com/0xD34DC0D3/template/internal/web"
)

const appName = "appname"

//go:embed views/*.html
var viewsFS embed.FS

type server struct {
	web  *web.Renderer
	auth *voodooauth.Client
	r    *db.Queries // read pool
	w    *db.Queries // single writer
	jobs inngestgo.Client
}

// New returns the full handler. Outermost first: log, security headers,
// recover (inside the logger so a panic is logged as a 500), body cap, CSRF,
// auth. Headers sit outside the CSRF check so rejections carry them too; auth
// sits inside it so a rejected cross-origin post costs no call to voodoo.
// Auth only identifies the caller; routes enforce with auth.Require*.
//
// jobs.Path is server-to-server (Inngest, authenticated by its signing key):
// it gets a larger body cap and skips CSRF and voodoo auth, but still runs
// through the log, headers and recover like every other route.
//
// siteURL is the org website (APPNAME_SITE_URL) the layout links to.
func New(d *db.DB, auth *voodooauth.Client, jc inngestgo.Client, siteURL string) http.Handler {
	views, err := fs.Sub(viewsFS, "views")
	if err != nil {
		panic(err) // embedded at build time; cannot fail at runtime
	}
	s := &server{web: web.New(appName, siteURL, views, auth), auth: auth, r: db.New(d.Reader), w: db.New(d.Writer), jobs: jc}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("GET /static/", web.StaticHandler())
	// No method: the SDK answers GET (inspect), POST (call) and PUT (sync).
	mux.Handle(jobs.Path, jc.Serve())
	s.exampleRoutes(mux)
	// Everything no other pattern claims gets the HTML 404 page instead of
	// the mux's text/plain one. It has no method, so a known path asked
	// with the wrong method is a 404 here too, not a 405: a method in the
	// pattern ("GET /") would conflict with jobs.Path, which has none.
	mux.HandleFunc("/", s.notFound)

	return chain(recordRoute(mux), requestLogger, securityHeaders, recoverer(s.panicPage), limitBody, crossOrigin, skipJobs(auth.Middleware))
}

// Error responses go through the layout. web.Renderer.Error makes them
// no-store, replacing any Cache-Control the handler set, so none is stored.

func (s *server) notFound(w http.ResponseWriter, r *http.Request) {
	s.web.Error(w, r, http.StatusNotFound, "There is nothing at this address.")
}

func (s *server) badRequest(w http.ResponseWriter, r *http.Request, msg string) {
	s.web.Error(w, r, http.StatusBadRequest, msg)
}

func (s *server) internalError(w http.ResponseWriter, r *http.Request, what string, err error) {
	slog.Error(what, "err", err)
	s.web.Error(w, r, http.StatusInternalServerError, "Something went wrong on our side. Try again in a moment.")
}

// panicPage answers a recovered panic. recoverer runs outside the auth
// middleware, so the page shows the signed-out header.
func (s *server) panicPage(w http.ResponseWriter, r *http.Request) {
	s.web.Error(w, r, http.StatusInternalServerError, "Something went wrong on our side. Try again in a moment.")
}
