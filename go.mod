module github.com/0xD34DC0D3/template

go 1.27.4

require (
	github.com/0xD34DC0D3/voodoo v0.0.0
	github.com/inngest/inngestgo v0.16.5
	github.com/joho/godotenv v1.5.1
	github.com/pressly/goose/v3 v3.30.1
	modernc.org/sqlite v1.62.0
)

require (
	github.com/coder/websocket v1.8.15 // indirect
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/fatih/structs v1.1.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gosimple/slug v1.12.0 // indirect
	github.com/gosimple/unidecode v1.0.1 // indirect
	github.com/gowebpki/jcs v1.0.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/inngest/inngest v1.19.3 // indirect
	github.com/lmittmann/tint v1.1.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/mfridman/interpolate v0.2.0 // indirect
	github.com/ncruces/go-strftime v1.1.0 // indirect
	github.com/oklog/ulid/v2 v2.1.1 // indirect
	github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/sethvargo/go-retry v0.5.0 // indirect
	github.com/xhit/go-str2duration/v2 v2.1.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sync v0.25.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
	golang.org/x/text v0.44.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261110000943-09e21b237f2c // indirect
	google.golang.org/grpc v1.85.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
	modernc.org/libc v1.81.2 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.13.0 // indirect
)

// Temporary: voodooauth is not yet published at a tagged version, so builds
// need a voodoo checkout beside appname. Known issue in docs/design.md.
replace github.com/0xD34DC0D3/voodoo => ../voodoo
