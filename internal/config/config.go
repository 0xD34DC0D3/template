package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DBPath string
	Port   int

	// BindHost is the interface the HTTP listener attaches to. Defaults to
	// loopback: the reverse proxy is the only intended client, and appname
	// terminates no TLS of its own, so nothing should reach it directly. Set
	// APPNAME_BIND to an empty string to listen on all interfaces.
	BindHost string

	// BaseURL is appname's external origin. Besides links, it is the origin of the
	// next= URL voodoo sends users back to after login; when set, the Host
	// header is never used for that.
	BaseURL  string
	LogLevel slog.Level

	// SiteURL is the org website the header tile and footer link to, with no
	// trailing slash. The org design guide says hosts come from env, never
	// code; the default is the website's local dev server.
	SiteURL string

	// VoodooURL is the voodoo server that owns sign-in and answers /me.
	VoodooURL string

	// Prod marks a real deployment. It turns on the stricter checks in
	// CheckServe, and later dev-only conveniences must refuse to run under it.
	// It defaults to on so a server missing the variable fails safe; local
	// development opts out in .env.
	Prod bool

	// Inngest settings use the SDK's own names, not APPNAME_, because the SDK
	// reads them from the environment itself; these copies exist only for
	// CheckServe and doctor. InngestDev follows the SDK: any non-empty
	// value (even "0") means dev mode, and a URL names the dev server.
	InngestSigningKey string
	InngestEventKey   string
	InngestDev        string
}

func Load() (*Config, error) {
	dbPath := os.Getenv("APPNAME_DB_PATH")
	if dbPath == "" {
		return nil, errors.New("APPNAME_DB_PATH is required")
	}
	port, err := envInt("APPNAME_PORT", 8081)
	if err != nil {
		return nil, err
	}
	prod, err := envBool("APPNAME_PROD", true)
	if err != nil {
		return nil, err
	}

	// envOr treats "" as unset, but APPNAME_BIND="" is the explicit "all
	// interfaces" escape hatch, so it is special-cased.
	bindHost := envOr("APPNAME_BIND", "127.0.0.1")
	if v, ok := os.LookupEnv("APPNAME_BIND"); ok && v == "" {
		bindHost = ""
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(envOr("APPNAME_LOG_LEVEL", "info"))); err != nil {
		return nil, fmt.Errorf("APPNAME_LOG_LEVEL: %w", err)
	}

	return &Config{
		DBPath:    dbPath,
		Port:      port,
		BindHost:  bindHost,
		BaseURL:   os.Getenv("APPNAME_BASE_URL"),
		SiteURL:   strings.TrimRight(envOr("APPNAME_SITE_URL", "http://localhost:4321"), "/"),
		LogLevel:  level,
		Prod:      prod,
		VoodooURL: os.Getenv("APPNAME_VOODOO_URL"),

		InngestSigningKey: os.Getenv("INNGEST_SIGNING_KEY"),
		InngestEventKey:   os.Getenv("INNGEST_EVENT_KEY"),
		InngestDev:        os.Getenv("INNGEST_DEV"),
	}, nil
}

// CheckServe validates settings only the HTTP server needs. It is separate from
// Load so `migrate` keeps working with just APPNAME_DB_PATH.
//
// In prod BaseURL is the origin of every link appname emits and the login
// redirect target, and VoodooURL receives users' session cookies and API
// keys; without https those would travel in the clear. Without BaseURL in
// prod, next= would come from the client-controlled Host header. SiteURL
// is linked from every page, so prod wants https there too (and so refuses
// the localhost default).
//
// In prod the signing key is what stops anyone else from calling
// /api/inngest, and dev mode turns that check off, so dev is refused and
// both keys are required (without the event key nothing can be sent).
func (c *Config) CheckServe() error {
	if !validURL(c.VoodooURL, c.Prod) {
		if c.Prod {
			return errors.New("APPNAME_VOODOO_URL must be an https URL when APPNAME_PROD is on")
		}
		return errors.New("APPNAME_VOODOO_URL is required and must be an http(s) URL")
	}
	if c.Prod && !validURL(c.BaseURL, true) {
		return errors.New("APPNAME_BASE_URL must be an https URL when APPNAME_PROD is on")
	}
	if !validURL(c.SiteURL, c.Prod) {
		if c.Prod {
			return errors.New("APPNAME_SITE_URL must be an https URL when APPNAME_PROD is on")
		}
		return errors.New("APPNAME_SITE_URL must be an http(s) URL")
	}
	if c.Prod {
		switch {
		case c.InngestDev != "":
			return errors.New("INNGEST_DEV must be unset when APPNAME_PROD is on")
		case c.InngestSigningKey == "":
			return errors.New("INNGEST_SIGNING_KEY is required when APPNAME_PROD is on")
		case c.InngestEventKey == "":
			return errors.New("INNGEST_EVENT_KEY is required when APPNAME_PROD is on")
		}
	}
	return nil
}

func validURL(s string, httpsOnly bool) bool {
	u, err := url.Parse(s)
	if s == "" || err != nil || u.Host == "" {
		return false
	}
	return u.Scheme == "https" || (!httpsOnly && u.Scheme == "http")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return n, nil
}

func envBool(key string, def bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s: %w", key, err)
	}
	return b, nil
}
