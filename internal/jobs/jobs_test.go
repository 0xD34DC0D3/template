package jobs

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/inngest/inngestgo"
	"github.com/pressly/goose/v3"

	"github.com/0xD34DC0D3/template/internal/db"
	"github.com/0xD34DC0D3/template/migrations"
)

// Outside an Inngest request step.Run just calls its function, so the
// functions run directly against a temp DB.
func TestExampleFunctions(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	p, err := goose.NewProvider(goose.DialectSQLite3, d.Writer, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := p.Up(ctx); err != nil {
		t.Fatal(err)
	}
	deps := Deps{R: db.New(d.Reader), W: db.New(d.Writer)}

	if _, err := deps.ping(ctx, inngestgo.Input[PingData]{}); err != nil {
		t.Fatal(err)
	}
	n, err := deps.countNotes(ctx, inngestgo.Input[any]{})
	if err != nil || n != int64(1) {
		t.Fatalf("countNotes = %v, %v; want 1", n, err)
	}
}

// Registration validates function configs (IDs, cron syntax) up front.
func TestNew(t *testing.T) {
	t.Setenv("INNGEST_DEV", "1")
	if _, err := New(Deps{}); err != nil {
		t.Fatal(err)
	}
}
