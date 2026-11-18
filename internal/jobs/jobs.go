// Package jobs creates appname's Inngest client and registers its functions.
// Inngest calls them over HTTP at Path; the client also sends events.
//
// The SDK reads its own env vars (INNGEST_SIGNING_KEY, INNGEST_EVENT_KEY,
// INNGEST_DEV, ...); internal/config reads the same ones only to refuse
// unsafe prod settings, so there is one name for each setting.
package jobs

import (
	"context"
	"log/slog"

	"github.com/inngest/inngestgo"
	"github.com/inngest/inngestgo/step"

	"github.com/0xD34DC0D3/template/internal/db"
)

// Path is where the SDK's handler is mounted. The dev server and Inngest
// Cloud are pointed at it.
const Path = "/api/inngest"

// MaxBody caps a request from Inngest. A call carries the event plus every
// earlier step's output, and Inngest caps a run's whole state at 32 MiB
// (https://www.inngest.com/docs/usage-limits/inngest); the extra 4 MiB is
// headroom for the JSON envelope around it.
const MaxBody = 36 << 20

// Deps is everything the functions use. Passed in, never global, so tests
// can hand them a temp DB.
type Deps struct {
	R *db.Queries // read pool
	W *db.Queries // single writer
}

// New returns the client with every function registered.
func New(d Deps) (inngestgo.Client, error) {
	c, err := inngestgo.NewClient(inngestgo.ClientOpts{
		AppID: "appname",
		// The SDK's own cap (default 100 MB) is kept in step with the
		// server's, so neither silently undercuts the other.
		MaxBodySize: MaxBody,
		Logger:      slog.Default(),
	})
	if err != nil {
		return nil, err
	}
	if err := d.register(c); err != nil {
		return nil, err
	}
	return c, nil
}

// EXAMPLE functions: show an event function writing through the Writer
// inside a step, and a cron reading through the Reader. A new app deletes
// them with the notes table.

// PingEvent is sent by the admin button on the notes page.
const PingEvent = "appname/example.ping"

// PingData is empty: the event itself is the message.
type PingData struct{}

// SendPing sends the EXAMPLE event.
func SendPing(ctx context.Context, c inngestgo.Client) error {
	_, err := c.Send(ctx, inngestgo.GenericEvent[PingData]{Name: PingEvent, Data: PingData{}})
	return err
}

func (d Deps) register(c inngestgo.Client) error {
	if _, err := inngestgo.CreateFunction(c,
		inngestgo.FunctionOpts{ID: "example-ping", Name: "EXAMPLE: ping adds a note"},
		inngestgo.EventTrigger(PingEvent, nil),
		d.ping,
	); err != nil {
		return err
	}
	_, err := inngestgo.CreateFunction(c,
		inngestgo.FunctionOpts{ID: "example-count-notes", Name: "EXAMPLE: log the notes count"},
		inngestgo.CronTrigger("0 * * * *"),
		d.countNotes,
	)
	return err
}

// ping adds one note. The write is in a step so a retry after a later
// failure replays its recorded result instead of adding a second note.
func (d Deps) ping(ctx context.Context, _ inngestgo.Input[PingData]) (any, error) {
	return step.Run(ctx, "add-note", func(ctx context.Context) (int64, error) {
		n, err := d.W.CreateNote(ctx, "EXAMPLE: added by the "+PingEvent+" job")
		return n.ID, err
	})
}

func (d Deps) countNotes(ctx context.Context, _ inngestgo.Input[any]) (any, error) {
	n, err := d.R.CountNotes(ctx)
	if err != nil {
		return nil, err
	}
	slog.Info("example cron: notes", "count", n)
	return n, nil
}
