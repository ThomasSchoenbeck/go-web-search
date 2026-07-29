module serp-harvester

go 1.26

// Resolved by `go mod tidy`:
//   github.com/chromedp/chromedp            browser automation
//   github.com/chromedp/cdproto             CDP types
//   github.com/BurntSushi/toml              config
//   github.com/google/uuid                  UUIDv7 primary keys
//   github.com/temoto/robotstxt             robots.txt parsing
//   golang.org/x/net                        HTML parsing
//   github.com/modelcontextprotocol/go-sdk  MCP server
//   turso.tech/database/tursogo             Turso database/sql driver
//
// The Turso Go bindings moved here from github.com/tursodatabase/turso-go and
// are still marked BETA, so pin the version you test with. Go 1.25+ is required
// by the MCP SDK.

require (
	github.com/BurntSushi/toml v1.6.0
	github.com/chromedp/cdproto v0.0.0-20260719223732-95f6af754cfe
	github.com/chromedp/chromedp v0.16.0
	github.com/google/uuid v1.6.0
	github.com/modelcontextprotocol/go-sdk v1.7.0
	github.com/temoto/robotstxt v1.1.2
	golang.org/x/net v0.57.0
	turso.tech/database/tursogo v0.7.1
)

require (
	github.com/chromedp/sysutil v1.1.0 // indirect
	github.com/ebitengine/purego v0.9.1 // indirect
	github.com/go-json-experiment/json v0.0.0-20260623181947-01eb4420fa68 // indirect
	github.com/gobwas/httphead v0.1.0 // indirect
	github.com/gobwas/pool v0.2.1 // indirect
	github.com/gobwas/ws v1.4.0 // indirect
	github.com/google/jsonschema-go v0.4.3 // indirect
	github.com/segmentio/asm v1.1.3 // indirect
	github.com/segmentio/encoding v0.5.4 // indirect
	github.com/tursodatabase/turso-go-platform-libs v0.7.1 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/oauth2 v0.35.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/time v0.15.0 // indirect
)
