package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The serve listener's port is written down in three places that must agree:
// the compiled default here, the shipped config.toml, and the SPA's dev-proxy
// target in web/vite.config.ts. They drifted once (8081 vs 8082), and the
// failure is silent — the dev server proxies /api to a port nothing is on, but
// only for someone running with no config.toml, so it survives every normal
// test run. This pins them together.
func TestServeAddrAgreesAcrossConfigAndDevProxy(t *testing.T) {
	compiled := defaultConfig().Server.Addr

	fromFile, err := loadConfig("config.toml", true)
	if err != nil {
		t.Fatalf("loading config.toml: %v", err)
	}
	if fromFile.Server.Addr != compiled {
		t.Errorf("config.toml server.addr = %q, compiled default = %q", fromFile.Server.Addr, compiled)
	}

	vite, err := os.ReadFile("web/vite.config.ts")
	if err != nil {
		t.Fatalf("reading the dev proxy config: %v", err)
	}
	proxy := regexp.MustCompile(`defaultProxyTarget = 'http://localhost:(\d+)'`).FindSubmatch(vite)
	if proxy == nil {
		t.Fatal("could not find defaultProxyTarget in web/vite.config.ts")
	}
	port := compiled[strings.LastIndex(compiled, ":")+1:]
	if got := string(proxy[1]); got != port {
		t.Errorf("the dev proxy targets port %s, but the serve listener defaults to %s", got, port)
	}
}
