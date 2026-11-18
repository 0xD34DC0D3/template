package server

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/0xD34DC0D3/voodoo/voodooauth"

	"github.com/0xD34DC0D3/template/internal/jobs"
	"github.com/0xD34DC0D3/template/internal/web"
)

// EXAMPLE pages: the template's demonstration of the render helper, Reader,
// Writer, an htmx form that also works without JS, error pages, and auth
// (anyone may read, only a signed-in user may add, one admin-only page, and
// an admin-only button that sends an Inngest event whose job adds a note).
// A new app deletes this file, views/notes.html, views/admin.html, the
// example nav links in web/layout.html and the notes table (with a new
// migration once production has applied 00001).

const maxNoteRunes = 500

func (s *server) exampleRoutes(mux *http.ServeMux) {
	// {$} so only "/" matches; the catch-all "/" in New answers the rest.
	mux.HandleFunc("GET /{$}", s.notesPage)
	mux.Handle("POST /notes", s.auth.RequireUser(http.HandlerFunc(s.addNote)))
	// Admins are voodoo's (is_admin); there is no local role table.
	mux.Handle("GET /admin", s.auth.RequireAdmin(http.HandlerFunc(s.adminPage)))
	mux.Handle("POST /example/ping", s.auth.RequireAdmin(http.HandlerFunc(s.sendPing)))
}

// sendPing sends the example event. The note appears only once the job has
// run, so htmx gets a fragment that re-fetches the list shortly after.
func (s *server) sendPing(w http.ResponseWriter, r *http.Request) {
	if err := jobs.SendPing(r.Context(), s.jobs); err != nil {
		slog.Error("send ping", "err", err)
		s.web.Error(w, r, http.StatusBadGateway, "Could not queue the job. Try again in a moment.")
		return
	}
	if r.Header.Get("HX-Request") != "true" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.web.Render(w, r, web.View{Page: "ping-sent", Fragment: "ping-sent"})
}

func (s *server) adminPage(w http.ResponseWriter, r *http.Request) {
	s.web.Render(w, r, web.View{Page: "admin", Title: "Admin", Nav: "admin", Data: voodooauth.UserFrom(r.Context())})
}

type noteView struct {
	Body    string
	Created string
}

// notesData is what notes.html sees: the list, plus the rejected input and
// its error when an htmx add failed validation.
type notesData struct {
	Notes []noteView
	Body  string
	Err   string
}

func (s *server) view(r *http.Request, nd notesData) (web.View, error) {
	notes, err := s.r.ListNotes(r.Context(), 50)
	if err != nil {
		return web.View{}, err
	}
	nd.Notes = make([]noteView, len(notes))
	for i, n := range notes {
		nd.Notes[i] = noteView{n.Body, time.Unix(n.CreatedAt, 0).UTC().Format("2006-01-02 15:04")}
	}
	return web.View{Page: "notes", Fragment: "notes-panel", Title: "Notes", Nav: "notes", Data: nd}, nil
}

func (s *server) notesPage(w http.ResponseWriter, r *http.Request) {
	v, err := s.view(r, notesData{})
	if err != nil {
		s.internalError(w, r, "list notes", err)
		return
	}
	s.web.Render(w, r, v)
}

func (s *server) addNote(w http.ResponseWriter, r *http.Request) {
	hx := r.Header.Get("HX-Request") == "true"
	if err := r.ParseForm(); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			s.web.Error(w, r, http.StatusRequestEntityTooLarge, "That was too large to send.")
			return
		}
		s.badRequest(w, r, "The form could not be read.")
		return
	}
	body := strings.TrimSpace(r.PostFormValue("body"))
	if body == "" || utf8.RuneCountInString(body) > maxNoteRunes {
		const msg = "A note must be 1 to 500 characters."
		if !hx {
			s.badRequest(w, r, msg)
			return
		}
		// htmx does not swap 4xx, so a rejected htmx post is a 200 with the
		// error inside the fragment, input kept.
		v, err := s.view(r, notesData{Body: body, Err: msg})
		if err != nil {
			s.internalError(w, r, "list notes", err)
			return
		}
		s.web.Render(w, r, v)
		return
	}
	if _, err := s.w.CreateNote(r.Context(), body); err != nil {
		s.internalError(w, r, "create note", err)
		return
	}
	// Without JS the browser needs post/redirect/get so reload does not repost.
	if !hx {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	v, err := s.view(r, notesData{})
	if err != nil {
		s.internalError(w, r, "list notes", err)
		return
	}
	s.web.Render(w, r, v)
}
