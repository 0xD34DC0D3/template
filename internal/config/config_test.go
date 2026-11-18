package config

import (
	"os"
	"testing"
)

// The bind host is the only thing keeping this process off the network, so the
// default has to be loopback and the opt-out has to be deliberate.
func TestLoadBindHost(t *testing.T) {
	cases := []struct {
		name string
		set  bool
		env  string
		want string
	}{
		{"unset defaults to loopback", false, "", "127.0.0.1"},
		{"explicit override is honoured", true, "0.0.0.0", "0.0.0.0"},
		{"IPv6 loopback passes through unchanged", true, "::1", "::1"},
		// envOr would fold this into the default; Load special-cases it so the
		// "all interfaces" escape hatch stays reachable.
		{"empty means all interfaces", true, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("APPNAME_DB_PATH", "/tmp/appname-test.db")
			// t.Setenv first either way so its cleanup restores whatever the
			// developer's shell had; the unset case then clears it, so an
			// exported APPNAME_BIND can't quietly turn the default green.
			t.Setenv("APPNAME_BIND", tc.env)
			if !tc.set {
				os.Unsetenv("APPNAME_BIND")
			}
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.BindHost != tc.want {
				t.Fatalf("BindHost = %q, want %q", cfg.BindHost, tc.want)
			}
		})
	}
}

func TestLoadDefaultsAndErrors(t *testing.T) {
	t.Setenv("APPNAME_DB_PATH", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() without APPNAME_DB_PATH succeeded")
	}

	t.Setenv("APPNAME_DB_PATH", "/tmp/appname-test.db")
	t.Setenv("APPNAME_PORT", "")
	t.Setenv("APPNAME_PROD", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 8081 || !cfg.Prod {
		t.Fatalf("defaults: Port = %d, Prod = %v; want 8081, true", cfg.Port, cfg.Prod)
	}

	t.Setenv("APPNAME_PORT", "abc")
	if _, err := Load(); err == nil {
		t.Fatal("Load() with a non-numeric APPNAME_PORT succeeded")
	}
	t.Setenv("APPNAME_PORT", "")
	t.Setenv("APPNAME_PROD", "maybe")
	if _, err := Load(); err == nil {
		t.Fatal("Load() with a non-boolean APPNAME_PROD succeeded")
	}
	t.Setenv("APPNAME_PROD", "")
	t.Setenv("APPNAME_LOG_LEVEL", "loud")
	if _, err := Load(); err == nil {
		t.Fatal("Load() with an unknown APPNAME_LOG_LEVEL succeeded")
	}
}

func TestCheckServe(t *testing.T) {
	const (
		devVoodoo  = "http://localhost:8080"
		prodVoodoo = "https://auth.example.com"
	)
	cases := []struct {
		name   string
		prod   bool
		base   string
		voodoo string
		ok     bool
	}{
		{"dev needs no base url", false, "", devVoodoo, true},
		{"dev allows http", false, "http://localhost:8081", devVoodoo, true},
		{"prod requires base url", true, "", prodVoodoo, false},
		{"prod rejects http", true, "http://appname.example.com", prodVoodoo, false},
		{"prod rejects missing host", true, "https://", prodVoodoo, false},
		{"prod accepts https", true, "https://appname.example.com", prodVoodoo, true},
		{"dev requires voodoo url", false, "", "", false},
		{"dev rejects non-http voodoo url", false, "", "localhost:8080", false},
		{"prod requires voodoo url", true, "https://appname.example.com", "", false},
		{"prod rejects http voodoo url", true, "https://appname.example.com", "http://auth.example.com", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{Prod: tc.prod, BaseURL: tc.base, VoodooURL: tc.voodoo, SiteURL: "https://site.example.com",
				InngestSigningKey: "signkey-prod-x", InngestEventKey: "ek"}
			if err := c.CheckServe(); (err == nil) != tc.ok {
				t.Fatalf("CheckServe() error = %v, want ok=%v", err, tc.ok)
			}
		})
	}
}

// In prod the signing key is the only thing authenticating calls to
// /api/inngest, and dev mode skips that check.
func TestCheckServeInngest(t *testing.T) {
	cases := []struct {
		name            string
		prod            bool
		sign, event, dv string
		ok              bool
	}{
		{"prod with both keys", true, "signkey-prod-x", "ek", "", true},
		{"prod requires signing key", true, "", "ek", "", false},
		{"prod requires event key", true, "signkey-prod-x", "", "", false},
		{"prod refuses dev mode", true, "signkey-prod-x", "ek", "1", false},
		// The SDK treats any non-empty INNGEST_DEV as dev, "0" included.
		{"prod refuses INNGEST_DEV=0", true, "signkey-prod-x", "ek", "0", false},
		{"dev needs no keys", false, "", "", "1", true},
		{"dev without the dev server is allowed", false, "", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{Prod: tc.prod, BaseURL: "https://appname.example.com", VoodooURL: "https://auth.example.com",
				SiteURL: "https://site.example.com", InngestSigningKey: tc.sign, InngestEventKey: tc.event, InngestDev: tc.dv}
			if err := c.CheckServe(); (err == nil) != tc.ok {
				t.Fatalf("CheckServe() error = %v, want ok=%v", err, tc.ok)
			}
		})
	}
}

func TestLoadInngest(t *testing.T) {
	t.Setenv("APPNAME_DB_PATH", "/tmp/appname-test.db")
	t.Setenv("INNGEST_SIGNING_KEY", "signkey-test-x")
	t.Setenv("INNGEST_EVENT_KEY", "ek")
	t.Setenv("INNGEST_DEV", "http://127.0.0.1:8288")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.InngestSigningKey != "signkey-test-x" || cfg.InngestEventKey != "ek" || cfg.InngestDev != "http://127.0.0.1:8288" {
		t.Fatalf("got %+v", cfg)
	}
}

// SiteURL is linked from every page: trailing slashes are trimmed (the
// layout appends "/"), and the localhost default is refused in prod.
func TestLoadSiteURL(t *testing.T) {
	cases := []struct {
		name string
		set  bool
		env  string
		want string
	}{
		{"unset defaults to the local website", false, "", "http://localhost:4321"},
		{"empty means the default", true, "", "http://localhost:4321"},
		{"trailing slash trimmed", true, "https://site.example.com/", "https://site.example.com"},
		{"kept as given", true, "https://site.example.com", "https://site.example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("APPNAME_DB_PATH", "/tmp/appname-test.db")
			t.Setenv("APPNAME_SITE_URL", tc.env)
			if !tc.set {
				os.Unsetenv("APPNAME_SITE_URL")
			}
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.SiteURL != tc.want {
				t.Fatalf("SiteURL = %q, want %q", cfg.SiteURL, tc.want)
			}
		})
	}
}

func TestCheckServeSiteURL(t *testing.T) {
	cases := []struct {
		name string
		prod bool
		site string
		ok   bool
	}{
		{"dev accepts the http default", false, "http://localhost:4321", true},
		{"dev accepts https", false, "https://site.example.com", true},
		{"dev rejects a missing scheme", false, "localhost:4321", false},
		{"dev rejects empty", false, "", false},
		{"prod accepts https", true, "https://site.example.com", true},
		{"prod rejects the http default", true, "http://localhost:4321", false},
		{"prod rejects missing host", true, "https://", false},
		{"prod rejects empty", true, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{Prod: tc.prod, BaseURL: "https://appname.example.com", VoodooURL: "https://auth.example.com",
				SiteURL: tc.site, InngestSigningKey: "signkey-prod-x", InngestEventKey: "ek"}
			if err := c.CheckServe(); (err == nil) != tc.ok {
				t.Fatalf("CheckServe() error = %v, want ok=%v", err, tc.ok)
			}
		})
	}
}
