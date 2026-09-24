// Command claude-limits serves, in the Prometheus text format, how much of each usage
// limit of this Mac's Claude plan is used and when each limit resets. It reads them with
// the Claude.ai login Claude Code keeps in the Keychain, and never refreshes that login.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:2114", "address to serve /metrics on")
	interval := flag.Duration("interval", 5*time.Minute, "time between reads of the usage API")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var page metricsPage
	http.Handle("/metrics", &page)
	server := &http.Server{Addr: *listen, ReadHeaderTimeout: 5 * time.Second}

	var wg sync.WaitGroup
	wg.Go(func() { refreshEvery(ctx, *interval, &page, anthropicUsageAPI()) })

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

func refreshEvery(ctx context.Context, interval time.Duration, page *metricsPage, api usageAPI) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		refreshCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := page.refresh(refreshCtx, keychainCredentials, api)
		cancel()
		if err != nil && ctx.Err() == nil {
			log.Print(err)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
