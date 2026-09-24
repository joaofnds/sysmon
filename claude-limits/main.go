// Command claude-limits serves, in the Prometheus text format, how much of each usage
// limit of this Mac's Claude plan is used and when each limit resets. It reads them with
// the Claude.ai login Claude Code keeps in the Keychain, and never refreshes that login.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:2114", "address to serve /metrics on")
	interval := flag.Duration("interval", 5*time.Minute, "time between reads of the usage API")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var page atomic.Pointer[[]byte]
	http.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		p := page.Load()
		if p == nil {
			http.Error(w, "the last read of the usage API failed", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.Write(*p)
	})
	server := &http.Server{Addr: *listen, ReadHeaderTimeout: 5 * time.Second}

	api := usageAPI{client: &http.Client{Timeout: 10 * time.Second}, baseURL: "https://api.anthropic.com"}
	var wg sync.WaitGroup
	wg.Go(func() { readEvery(ctx, *interval, api, &page) })

	serveErr := make(chan error, 1)
	go func() { serveErr <- server.ListenAndServe() }()
	select {
	case err := <-serveErr:
		stop()
		wg.Wait()
		log.Fatal(err)
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Print(err)
	}
	wg.Wait()
}

func readEvery(ctx context.Context, interval time.Duration, api usageAPI, page *atomic.Pointer[[]byte]) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		readCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		limits, err := read(readCtx, api)
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Print(err)
			page.Store(nil)
		} else {
			var b bytes.Buffer
			writeMetrics(&b, limits)
			p := b.Bytes()
			page.Store(&p)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// read takes the access token from the Keychain on every call, so it follows each
// refresh Claude Code makes.
func read(ctx context.Context, api usageAPI) ([]limit, error) {
	item, err := exec.CommandContext(ctx, "/usr/bin/security", "find-generic-password", "-s", "Claude Code-credentials", "-w").Output()
	if err != nil {
		return nil, fmt.Errorf("reading Claude Code's credentials from the Keychain: %w", err)
	}
	token, err := accessToken(item)
	if err != nil {
		return nil, err
	}
	return api.limits(ctx, token)
}
