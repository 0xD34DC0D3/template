package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/0xD34DC0D3/template/internal/jobs"
)

// Middleware is plain net/http; chain applies them outermost first.
type Middleware func(http.Handler) http.Handler

func chain(h http.Handler, mw ...Middleware) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}

// maxBody caps request bodies. Forms here are tiny; a hard cap stops a client
// from making the server buffer something huge. Handlers that read the body
// see *http.MaxBytesError and answer 413. Inngest's calls carry a run's
// whole state, so its route gets jobs.MaxBody instead.
const maxBody = 64 << 10

func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := int64(maxBody)
		if r.URL.Path == jobs.Path {
			limit = jobs.MaxBody
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		next.ServeHTTP(w, r)
	})
}

// skipJobs runs mw everywhere except jobs.Path. Inngest sends no voodoo
// credential, so asking voodoo about it would be wasted work.
func skipJobs(mw Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		wrapped := mw(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == jobs.Path {
				next.ServeHTTP(w, r)
				return
			}
			wrapped.ServeHTTP(w, r)
		})
	}
}

// crossOrigin rejects cross-origin browser requests on unsafe methods (CSRF)
// using the stdlib's Sec-Fetch-Site / Origin check. Requests carrying neither
// header (curl, servers, API-key clients) pass: they hold no ambient cookie a
// third-party page could borrow.
//
// jobs.Path is exempt outright. Inngest's calls carry neither header today,
// but they are authenticated by signature, not cookie, so a browser header a
// proxy or future SDK adds must not turn them into 403s.
var crossOrigin = func() Middleware {
	c := http.NewCrossOriginProtection()
	c.AddInsecureBypassPattern(jobs.Path)
	return c.Handler
}()

// contentSecurityPolicy allows only our own assets: no inline script or style,
// no framing. script-src is explicit so the intent survives a loosened
// default-src. htmx needs nothing beyond 'self' (see the htmx-config meta tag
// in web/layout.html).
const contentSecurityPolicy = "default-src 'self'; script-src 'self'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'"

// securityHeaders sets the browser hardening headers on every response.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// recoverer turns a handler panic into a 500 (written by page) instead of a
// dropped connection. http.ErrAbortHandler is net/http's deliberate abort
// and is re-panicked.
func recoverer(page http.HandlerFunc) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				v := recover()
				if v == nil {
					return
				}
				if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(v)
				}
				slog.Error("panic", "panic", v)
				page(w, r)
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// statusWriter records the status for the request log.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the real writer.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

type routeKey struct{}

// recordRoute wraps the mux and hands the matched pattern back to
// requestLogger. ServeMux sets r.Pattern on the request it is given, but any
// middleware in between that calls r.WithContext (the auth middleware does)
// passes the mux a copy, so the logger's own r never sees it. Deferred so a
// panicking handler is still logged with its route.
func recordRoute(mux http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p, ok := r.Context().Value(routeKey{}).(*string); ok {
				*p = r.Pattern
			}
		}()
		mux.ServeHTTP(w, r)
	})
}

// requestLogger writes one line per request: method, route pattern, status,
// duration. It logs the route pattern ("GET /static/"), never the URL: query
// strings and paths can carry tokens. It must wrap recoverer so a recovered
// panic is logged as the 500 it became.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		matched := new(string) // filled by recordRoute
		next.ServeHTTP(sw, r.WithContext(context.WithValue(r.Context(), routeKey{}, matched)))
		// Empty means no route matched (or the request never reached the
		// mux), and the raw path must not be logged.
		route := *matched
		if route == "" {
			route = "unmatched"
		}
		if sw.status == 0 {
			sw.status = http.StatusOK
		}
		attrs := []any{
			"method", r.Method,
			"route", route,
			"status", sw.status,
			"dur_ms", time.Since(start).Milliseconds(),
		}
		if sw.status >= http.StatusInternalServerError {
			slog.Error("request", attrs...)
		} else {
			slog.Info("request", attrs...)
		}
	})
}
