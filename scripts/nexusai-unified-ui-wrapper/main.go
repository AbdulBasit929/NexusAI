package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	defaultBackendURL = "http://127.0.0.1:8080"
	defaultChild      = "/entrypoint.sh"
	defaultListen     = ":8088"
	defaultUIRoot     = "/opt/nexusai/unified-ui"
)

var backendPrefixes = []string{
	"/api",
	"/v1",
	"/tts",
	"/video",
	"/backend",
	"/models",
	"/backends",
	"/branding",
	"/swagger",
	"/static",
	"/generated-audio",
	"/generated-images",
	"/generated-videos",
	"/version",
	"/system",
	"/readyz",
	"/healthz",
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func routeToBackend(requestPath string) bool {
	for _, prefix := range backendPrefixes {
		if requestPath == prefix || strings.HasPrefix(requestPath, prefix+"/") {
			return true
		}
	}
	return false
}

func localAsset(root, requestPath string) (string, bool) {
	clean := strings.TrimPrefix(path.Clean("/"+requestPath), "/")
	if clean == "." || clean == "" {
		return "", false
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	fileAbs, err := filepath.Abs(filepath.Join(rootAbs, filepath.FromSlash(clean)))
	if err != nil {
		return "", false
	}
	if fileAbs != rootAbs && !strings.HasPrefix(fileAbs, rootAbs+string(os.PathSeparator)) {
		return "", false
	}

	info, err := os.Stat(fileAbs)
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return fileAbs, true
}

func newGateway(uiRoot string, backendURL *url.URL) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(backendURL)
	proxy.FlushInterval = -1

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if routeToBackend(r.URL.Path) {
			proxy.ServeHTTP(w, r)
			return
		}

		if file, ok := localAsset(uiRoot, r.URL.Path); ok {
			if contentType := mime.TypeByExtension(filepath.Ext(file)); contentType != "" {
				w.Header().Set("Content-Type", contentType)
			}
			http.ServeFile(w, r, file)
			return
		}

		acceptsHTML := strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/html")
		if (r.Method == http.MethodGet || r.Method == http.MethodHead) && acceptsHTML {
			http.ServeFile(w, r, filepath.Join(uiRoot, "index.html"))
			return
		}

		proxy.ServeHTTP(w, r)
	})
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}

func run() int {
	backendURL, err := url.Parse(envOrDefault("NEXUSAI_BACKEND_URL", defaultBackendURL))
	if err != nil {
		log.Printf("invalid NEXUSAI_BACKEND_URL: %v", err)
		return 1
	}

	uiRoot := envOrDefault("NEXUSAI_UI_ROOT", defaultUIRoot)
	if _, err := os.Stat(filepath.Join(uiRoot, "index.html")); err != nil {
		log.Printf("unified UI index is unavailable: %v", err)
		return 1
	}

	child := exec.Command(envOrDefault("NEXUSAI_BASE_ENTRYPOINT", defaultChild), os.Args[1:]...)
	child.Env = os.Environ()
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	if err := child.Start(); err != nil {
		log.Printf("could not start accepted LocalAI entrypoint: %v", err)
		return 1
	}

	server := &http.Server{
		Addr:              envOrDefault("NEXUSAI_UI_LISTEN", defaultListen),
		Handler:           newGateway(uiRoot, backendURL),
		ReadHeaderTimeout: 15 * time.Second,
	}

	childDone := make(chan error, 1)
	go func() { childDone <- child.Wait() }()

	serverDone := make(chan error, 1)
	go func() { serverDone <- server.ListenAndServe() }()

	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}

	select {
	case sig := <-signals:
		_ = child.Process.Signal(sig)
		shutdown()
		return exitCode(<-childDone)
	case err := <-childDone:
		shutdown()
		return exitCode(err)
	case err := <-serverDone:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("unified UI gateway stopped: %v", err)
		}
		_ = child.Process.Signal(syscall.SIGTERM)
		return exitCode(<-childDone)
	}
}

func main() {
	fmt.Fprintf(os.Stdout, "NexusAIUnifiedUIGateway=%s\n", envOrDefault("NEXUSAI_UI_LISTEN", defaultListen))
	os.Exit(run())
}
