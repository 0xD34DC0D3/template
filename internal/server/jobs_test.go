package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/0xD34DC0D3/voodoo/voodooauth"
	"github.com/inngest/inngestgo"

	"github.com/0xD34DC0D3/template/internal/jobs"
)

// fakeJobs stands in for the Inngest client: Serve reports what reached the
// handler, Send records event names. Unused methods panic via the nil embed.
type fakeJobs struct {
	inngestgo.Client
	sent    []string
	sendErr error
}

func (f *fakeJobs) Send(_ context.Context, evt any) (string, error) {
	if f.sendErr != nil {
		return "", f.sendErr
	}
	f.sent = append(f.sent, evt.(inngestgo.GenericEvent[jobs.PingData]).Name)
	return "id", nil
}

// Serve reads the whole body, as the SDK does, and says how much arrived and
// whether voodoo auth ran (it put a user on the context).
func (f *fakeJobs) Serve() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, err := io.Copy(io.Discard, r.Body)
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			http.Error(w, "too large", http.StatusRequestEntityTooLarge)
			return
		}
		fmt.Fprintf(w, "read=%d user=%v", n, voodooauth.UserFrom(r.Context()) != nil)
	})
}

func TestJobsRouteThroughMiddleware(t *testing.T) {
	h := newTestHandlerWith(t, &fakeJobs{})
	var logBuf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	post := func(size int) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, jobs.Path, bytes.NewReader(make([]byte, size)))
		// Browser CSRF signals and a session cookie must not matter here.
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		req.Header.Set("Origin", "https://evil.example")
		return do(h, withCookie(req, "admin"))
	}

	// Well over the 64 KB cap every other route has.
	big := 1 << 20
	rec := post(big)
	if want := fmt.Sprintf("read=%d user=false", big); rec.Code != http.StatusOK || rec.Body.String() != want {
		t.Fatalf("got %d %q, want 200 %q", rec.Code, rec.Body.String(), want)
	}
	if rec := post(jobs.MaxBody + 1); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("over jobs.MaxBody: %d, want 413", rec.Code)
	}
	if !strings.Contains(logBuf.String(), `route=/api/inngest`) {
		t.Errorf("route missing from log: %s", logBuf.String())
	}
}

// The real SDK handler answers through the stack: in dev mode GET is its
// unauthenticated inspection, which lists the registered functions.
func TestJobsRouteRealSDK(t *testing.T) {
	h := newTestHandler(t)
	rec := do(h, httptest.NewRequest(http.MethodGet, jobs.Path, nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"function_count":2`) {
		t.Fatalf("GET %s = %d %s", jobs.Path, rec.Code, rec.Body.String())
	}
}

func TestPingButton(t *testing.T) {
	post := func(h http.Handler, cookie string, hx bool) *httptest.ResponseRecorder {
		req := withCookie(httptest.NewRequest(http.MethodPost, "/example/ping", nil), cookie)
		if hx {
			req.Header.Set("HX-Request", "true")
		}
		return do(h, req)
	}

	fj := &fakeJobs{}
	h := newTestHandlerWith(t, fj)
	if rec := post(h, "admin", false); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Errorf("no-JS: %d Location=%q, want 303 /", rec.Code, rec.Header().Get("Location"))
	}
	rec := post(h, "admin", true)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `hx-get="/"`) || strings.Contains(rec.Body.String(), "<html") {
		t.Errorf("htmx: %d %s", rec.Code, rec.Body.String())
	}
	if len(fj.sent) != 2 || fj.sent[0] != jobs.PingEvent {
		t.Errorf("sent = %v, want two %s", fj.sent, jobs.PingEvent)
	}
	if rec := post(h, "user", false); rec.Code != http.StatusForbidden {
		t.Errorf("non-admin: %d, want 403", rec.Code)
	}
	if len(fj.sent) != 2 {
		t.Errorf("non-admin sent an event: %v", fj.sent)
	}

	page := func(cookie string) string {
		return do(h, withCookie(httptest.NewRequest(http.MethodGet, "/", nil), cookie)).Body.String()
	}
	if !strings.Contains(page("admin"), `action="/example/ping"`) || strings.Contains(page("user"), `action="/example/ping"`) {
		t.Error("ping button should show for admins only")
	}

	failing := newTestHandlerWith(t, &fakeJobs{sendErr: errors.New("down")})
	if rec := post(failing, "admin", false); rec.Code != http.StatusBadGateway {
		t.Errorf("send failure: %d, want 502", rec.Code)
	}
}
