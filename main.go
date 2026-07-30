package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/chromedp/chromedp"
)

func main() {
	if err := realMain(); err != nil {
		fmt.Fprintln(os.Stderr, "harvester:", err)
		os.Exit(1)
	}
}

func realMain() error {
	configPath := flag.String("config", "config.toml", "TOML config file")
	mode := flag.String("mode", "serve", "browse: drive the browser yourself; serve: REST + MCP server")
	termsPath := flag.String("terms", "", "override search.terms_file")
	enginesCSV := flag.String("engines", "", "override search.engines")
	typedCSV := flag.String("typed", "", "override search.typed (none to disable)")
	resultsSpec := flag.String("results", "", "override search.results, e.g. google=20")
	dataDir := flag.String("data", "", "override database.data_dir")
	userData := flag.String("userdata", "", "override browser.user_data_dir")
	profile := flag.String("profile", "", "override browser.profile")
	outDir := flag.String("out", "", "override search.artifact_dir")
	startURL := flag.String("start", "", "override browser.start_url")
	headless := flag.Bool("headless", false, "override browser.headless")
	fixUA := flag.Bool("fix-ua", true, "override browser.fix_ua")
	noSandbox := flag.Bool("no-sandbox", false, "override browser.no_sandbox")
	serverPort := flag.String("port", "", "override server.addr port (e.g. 8080)")
	minDelay := flag.Duration("min-delay", 0, "override search.min_delay")
	maxDelay := flag.Duration("max-delay", 0, "override search.max_delay")
	queryTimeout := flag.Duration("query-timeout", 0, "override search.query_timeout")
	flag.Parse()

	configExplicit := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "config" {
			configExplicit = true
		}
	})

	cfg, err := loadConfig(*configPath, configExplicit)
	if err != nil {
		return err
	}

	// Only flags the user actually typed override the config file. Without
	// flag.Visit every flag default would silently clobber the file.
	var flagErr error
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "terms":
			cfg.Search.TermsFile = *termsPath
		case "engines":
			cfg.Search.Engines = splitList(*enginesCSV)
		case "typed":
			cfg.Search.Typed = splitList(*typedCSV)
		case "results":
			counts, err := parseResultCounts(*resultsSpec)
			if err != nil {
				flagErr = err
				return
			}
			cfg.Search.Results = counts
		case "data":
			cfg.Database.DataDir = *dataDir
		case "userdata":
			cfg.Browser.UserDataDir = *userData
		case "profile":
			cfg.Browser.Profile = *profile
		case "out":
			cfg.Search.ArtifactDir = *outDir
		case "start":
			cfg.Browser.StartURL = *startURL
		case "headless":
			cfg.Browser.Headless = *headless
		case "fix-ua":
			cfg.Browser.FixUA = *fixUA
		case "no-sandbox":
			cfg.Browser.NoSandbox = *noSandbox
		case "port":
			host := "0.0.0.0"
			if strings.Contains(cfg.Server.Addr, ":") {
				host = strings.Split(cfg.Server.Addr, ":")[0]
			}
			cfg.Server.Addr = host + ":" + *serverPort
		case "min-delay":
			cfg.Search.MinDelay = Duration{*minDelay}
		case "max-delay":
			cfg.Search.MaxDelay = Duration{*maxDelay}
		case "query-timeout":
			cfg.Search.QueryTimeout = Duration{*queryTimeout}
		}
	})
	if flagErr != nil {
		return flagErr
	}

	dataPath, err := filepath.Abs(cfg.Database.DataDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dataPath, 0o755); err != nil {
		return err
	}

	// Turso supports multi-process access, so the CLI and the server can share
	// these files. The lock stays available for anyone who would rather not
	// rely on that.
	if cfg.Database.ExclusiveLock {
		lock, err := acquireLock(dataPath)
		if err != nil {
			return err
		}
		defer lock.release()
	}

	store, err := openStore(cfg.Database.Driver, filepath.Join(dataPath, cfg.Database.MainDB), cfg.Database.MaxOpenConns)
	if err != nil {
		return err
	}
	defer store.Close()

	logs, err := openLogStore(cfg.Database.Driver, filepath.Join(dataPath, cfg.Database.LogDB))
	if err != nil {
		return err
	}
	logWriter := &dbLogWriter{store: logs}
	defer func() {
		if dropped, err := logs.Close(); err != nil {
			fmt.Fprintln(os.Stderr, "harvester: closing log db:", err)
		} else if dropped > 0 {
			fmt.Fprintf(os.Stderr, "harvester: %d log lines dropped (buffer full)\n", dropped)
		}
	}()

	art, err := newArtifacts(cfg.Search.ArtifactDir, logWriter)
	if err != nil {
		return err
	}
	defer art.Close()

	ctx := context.Background()
	runID, err := store.StartRun(ctx, *mode, art.Dir)
	if err != nil {
		return err
	}
	logWriter.setRun(runID)

	userDataDir, err := filepath.Abs(cfg.Browser.UserDataDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(userDataDir, 0o755); err != nil {
		return err
	}
	profilePath := filepath.Join(userDataDir, cfg.Browser.Profile)
	if _, statErr := os.Stat(profilePath); os.IsNotExist(statErr) {
		art.Log.Printf("NOTE: profile %q does not exist yet and will be created empty.", cfg.Browser.Profile)
		art.Log.Printf("      A profile with no history is the strongest bot signal there is.")
		art.Log.Printf("      Run -mode browse once and use the browser normally before harvesting.")
	}

	// Shutdown signals must NOT be the parent of the browser context: cancelling
	// that context kills Chrome outright, which is exactly the ungraceful exit
	// that loses the profile's cookies. This context only breaks our own loops.
	stop, stopCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopCancel()

	art.Log.Printf("run id        : %s", runID)
	art.Log.Printf("run directory : %s", art.Dir)
	art.Log.Printf("database      : %s", filepath.Join(dataPath, cfg.Database.MainDB))
	art.Log.Printf("mode          : %s", *mode)
	art.Log.Printf("profile       : %s", profilePath)
	art.Log.Printf("headless      : %v", cfg.Browser.Headless)

	var runErr error
	switch *mode {
	case "browse":
		runErr = browseMode(cfg, art, userDataDir, stop)
	case "serve":
		runErr = serveWithBrowser(cfg, art, store, userDataDir, stop)
	default:
		runErr = fmt.Errorf("unknown -mode %q (want browse or serve)", *mode)
	}

	if err := store.FinishRun(ctx, runID); err != nil {
		art.Log.Printf("WARNING: could not mark run finished: %v", err)
	}
	return runErr
}

// browseMode opens a real window on the persistent profile and hands it to the
// user. Everything Chrome learns - cookies, consent, history, captcha
// exemptions - stays in the profile directory.
func browseMode(cfg Config, art *artifacts, userDataDir string, stop context.Context) error {
	if cfg.Browser.Headless {
		art.Log.Printf("browse mode ignores headless: there would be nothing to drive")
	}

	s, err := launch(context.Background(), sessionOpts{
		headless:    false,
		noSandbox:   cfg.Browser.NoSandbox,
		userDataDir: userDataDir,
		profileName: cfg.Browser.Profile,
		normalizeUA: false,
		log:         art.Log,
	})
	if err != nil {
		return err
	}
	defer s.close()

	if err := chromedp.Run(s.ctx, chromedp.Navigate(cfg.Browser.StartURL)); err != nil {
		art.Log.Printf("WARNING: could not open %s: %v", cfg.Browser.StartURL, err)
	}

	art.Log.Printf("browser is open on profile %q", cfg.Browser.Profile)
	fmt.Println()
	fmt.Println("  Use the browser like you normally would: accept consent dialogs, run a few")
	fmt.Println("  searches by hand, click through to a couple of results, let it sit a while.")
	fmt.Println()
	fmt.Println("  Finish by closing the browser window, or by pressing Enter here.")
	fmt.Println("  Either way Chrome shuts down cleanly and writes its cookies to disk.")
	fmt.Println()
	fmt.Print("  Waiting... ")

	enter := make(chan struct{})
	go func() {
		bufio.NewReader(os.Stdin).ReadString('\n')
		close(enter)
	}()

	select {
	case <-enter:
		art.Log.Printf("closing browser on request")
	case <-s.closed():
		// No terminal needed: this is the path used when driving the browser
		// from a tablet over a remote desktop.
		art.Log.Printf("browser window was closed; Chrome saved the profile on its way out")
	case <-stop.Done():
		art.Log.Printf("signal received, closing browser")
	}
	return nil
}

// serveWithBrowser launches Chrome once and serves REST + MCP on top of it.
func serveWithBrowser(cfg Config, art *artifacts, store *Store, userDataDir string, stop context.Context) error {
	s, err := launch(context.Background(), sessionOpts{
		headless:    cfg.Browser.Headless,
		noSandbox:   cfg.Browser.NoSandbox,
		userDataDir: userDataDir,
		profileName: cfg.Browser.Profile,
		normalizeUA: cfg.Browser.FixUA,
		log:         art.Log,
	})
	if err != nil {
		return err
	}
	defer s.close()

	return serveMode(cfg, art, newHarvester(cfg, store, art.Log, s), stop)
}

// waitOrStop pauses between queries, returning false if a shutdown signal
// arrived first.
func waitOrStop(stop context.Context, min, max time.Duration) bool {
	if stop.Err() != nil {
		return false
	}
	d := min
	if max > min {
		d = min + time.Duration(rand.Int63n(int64(max-min)))
	}
	if d <= 0 {
		return true
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-stop.Done():
		return false
	}
}
