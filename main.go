// DSP Ledger: a local tracker for Dyson Sphere Program planets, production
// lines and logistics routes.
//
// The program serves the tracker page on 127.0.0.1, opens it in the default
// browser, and stores everything in a single JSON file
// (dsp-ledger-data.json) next to the executable. Standard library only, so it
// cross-compiles to Windows, Linux and macOS with no extra dependencies.
package main

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed web/index.html
var indexHTML []byte

// seed.json is written as the data file the first time the app runs.
//
//go:embed seed.json
var seedJSON []byte

const (
	appID       = "dsp-ledger"
	appVersion  = "1.0.0"
	dataName    = "dsp-ledger-data.json"
	defaultPort = 8686
	maxBody     = 16 << 20 // 16 MiB is far beyond any realistic factory
)

type state struct {
	Version int               `json:"version"`
	Planets []json.RawMessage `json:"planets"`
	Lines   []json.RawMessage `json:"lines"`
	Routes  []json.RawMessage `json:"routes"`
}

type store struct {
	mu       sync.Mutex
	path     string
	backedUp bool
}

func main() {
	port := flag.Int("port", defaultPort, "port to listen on (127.0.0.1 only)")
	noBrowser := flag.Bool("no-browser", false, "don't open the browser automatically")
	dataFlag := flag.String("data", "", "path to the data file (default: next to the executable)")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("DSP Ledger", appVersion)
		return
	}

	path, err := resolveDataPath(*dataFlag)
	if err != nil {
		fatal(err)
	}
	st := &store{path: path}
	if err := st.ensure(); err != nil {
		fatal(err)
	}

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(*port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		// Port taken. If it's another copy of DSP Ledger, just reopen its page.
		if alreadyRunning(addr) {
			url := "http://" + addr + "/"
			fmt.Println("DSP Ledger is already running at", url)
			if !*noBrowser {
				openBrowser(url)
			}
			return
		}
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			fatal(fmt.Errorf("could not open a local port: %w", err))
		}
	}
	host := ln.Addr().String()
	url := "http://" + host + "/"

	quit := make(chan struct{})
	var quitOnce sync.Once
	srv := &http.Server{
		Handler:           routes(st, host, func() { quitOnce.Do(func() { close(quit) }) }),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatal(err)
		}
	}()

	fmt.Println("DSP Ledger", appVersion)
	fmt.Println("  Open:      ", url)
	fmt.Println("  Data file: ", st.path)
	fmt.Println()
	fmt.Println("Leave this running while you use the tracker.")
	fmt.Println("Close this window, press Ctrl+C, or use Quit in the app to stop it.")

	if !*noBrowser {
		openBrowser(url)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sig:
	case <-quit:
	}
	fmt.Println("Stopping DSP Ledger…")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func routes(st *store, host string, stop func()) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(indexHTML)
	})

	mux.HandleFunc("/api/info", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"app": appID, "version": appVersion, "dataPath": st.path})
	})

	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			b, err := st.read()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write(b)
		case http.MethodPut:
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				http.Error(w, "expected application/json", http.StatusUnsupportedMediaType)
				return
			}
			body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
			if err != nil || len(body) > maxBody {
				http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
				return
			}
			if err := st.write(body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.Header().Set("Allow", "GET, PUT")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/quit", func(w http.ResponseWriter, r *http.Request) {
		// The custom header forces a CORS preflight, so other websites can't trigger this.
		if r.Method != http.MethodPost || r.Header.Get("X-DSP-Ledger") != "1" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		go func() { time.Sleep(200 * time.Millisecond); stop() }()
	})

	return guard(host, mux)
}

// guard only answers requests addressed to this local server, which blocks
// DNS-rebinding tricks from other websites. No CORS headers are ever sent,
// so cross-origin pages can't read or write the data.
func guard(host string, next http.Handler) http.Handler {
	_, port, _ := net.SplitHostPort(host)
	ok := map[string]bool{host: true, "localhost:" + port: true}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !ok[r.Host] {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// ---- storage ----

func resolveDataPath(flagPath string) (string, error) {
	if flagPath != "" {
		abs, err := filepath.Abs(flagPath)
		if err != nil {
			return "", err
		}
		return abs, os.MkdirAll(filepath.Dir(abs), 0o755)
	}
	// Prefer the folder the executable lives in, so the app stays portable.
	if exe, err := os.Executable(); err == nil {
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		dir := filepath.Dir(exe)
		if writable(dir) {
			return filepath.Join(dir, dataName), nil
		}
	}
	// Fall back to the per-user config folder (e.g. Program Files isn't writable).
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("no writable folder for the data file: %w", err)
	}
	dir := filepath.Join(cfg, "dsp-ledger")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, dataName), nil
}

func writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".dsp-ledger-probe-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}

func (s *store) ensure() error {
	if _, err := os.Stat(s.path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	seed := seedJSON
	if _, err := parseState(seed); err != nil {
		seed = []byte(`{"version":1,"planets":[],"lines":[],"routes":[]}`)
	}
	return atomicWrite(s.path, seed)
}

func (s *store) read() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	if _, err := parseState(b); err != nil {
		return nil, fmt.Errorf("the data file isn't valid: %v", err)
	}
	return b, nil
}

func (s *store) write(body []byte) error {
	st, err := parseState(body)
	if err != nil {
		return err
	}
	out, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Once per launch, keep a copy of the file as it was when the app started.
	if !s.backedUp {
		if old, err := os.ReadFile(s.path); err == nil && len(old) > 0 {
			_ = atomicWrite(strings.TrimSuffix(s.path, ".json")+".bak.json", old)
		}
		s.backedUp = true
	}
	return atomicWrite(s.path, append(out, '\n'))
}

func parseState(b []byte) (*state, error) {
	var st state
	dec := json.NewDecoder(bytes.NewReader(b))
	if err := dec.Decode(&st); err != nil {
		return nil, fmt.Errorf("invalid JSON: %v", err)
	}
	if st.Planets == nil {
		st.Planets = []json.RawMessage{}
	}
	if st.Lines == nil {
		st.Lines = []json.RawMessage{}
	}
	if st.Routes == nil {
		st.Routes = []json.RawMessage{}
	}
	if st.Version == 0 {
		st.Version = 1
	}
	return &st, nil
}

// atomicWrite writes to a temp file then renames it over the target, so a
// crash mid-save never leaves a half-written data file.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".dsp-ledger-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// ---- helpers ----

func alreadyRunning(addr string) bool {
	c := http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := c.Get("http://" + addr + "/api/info")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var info struct {
		App string `json:"app"`
	}
	return json.NewDecoder(resp.Body).Decode(&info) == nil && info.App == appID
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		fmt.Println("Couldn't open a browser automatically. Open this address yourself:", url)
		return
	}
	go cmd.Wait()
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "DSP Ledger error:", err)
	if runtime.GOOS == "windows" {
		// Keep the console window open long enough to read the message.
		fmt.Fprintln(os.Stderr, "Press Enter to close.")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	}
	os.Exit(1)
}
