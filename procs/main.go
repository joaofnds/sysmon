// Command sysmon-procs serves, in the Prometheus text format, what each app on this Mac
// uses: CPU time, memory, disk, network, GPU time, energy, wakeups, page-ins, threads
// and open files and sockets.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:2113", "address to serve /metrics on")
	interval := flag.Duration("interval", 10*time.Second, "time between samples")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var page atomic.Pointer[[]byte]
	http.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		p := page.Load()
		if p == nil {
			http.Error(w, "no sample yet", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.Write(*p)
	})
	server := &http.Server{Addr: *listen, ReadHeaderTimeout: 5 * time.Second}

	var wg sync.WaitGroup
	wg.Go(func() { sampleEvery(ctx, *interval, &page) })

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

func sampleEvery(ctx context.Context, interval time.Duration, page *atomic.Pointer[[]byte]) {
	l := newLedger()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		sampleCtx, cancel := context.WithTimeout(ctx, interval)
		processes, err := sample(sampleCtx)
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Print(err)
		} else {
			l.Record(processes)
			var b bytes.Buffer
			writeMetrics(&b, l.Totals(), l.Usage())
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

// sample reads every running process with the counters macOS lets this user read,
// leaving NaN where it does not.
func sample(ctx context.Context) ([]process, error) {
	var (
		network    map[int]netBytes
		networkErr error
		done       = make(chan struct{})
	)
	go func() {
		defer close(done)
		network, networkErr = networkByPID(ctx)
	}()
	listed, psErr := listProcesses(ctx)
	gpu, gpuErr := gpuSecondsByPID()
	<-done
	if err := errors.Join(psErr, networkErr, gpuErr); err != nil {
		return nil, fmt.Errorf("sampling processes: %w", err)
	}

	processes := make([]process, 0, len(listed))
	for _, p := range listed {
		u := resourceUsageOf(p.PID)
		c := counters{
			cpuSeconds:       p.CPUSeconds,
			gpuSeconds:       gpu[p.PID],
			diskReadBytes:    u.DiskReadBytes,
			diskWrittenBytes: u.DiskWrittenBytes,
			energyJoules:     u.EnergyJoules,
			idleWakeups:      u.IdleWakeups,
			pageIns:          u.PageIns,
		}
		if n, ok := network[p.PID]; ok {
			c[networkReceivedBytes], c[networkSentBytes] = float64(n.Received), float64(n.Sent)
		} else {
			c[networkReceivedBytes], c[networkSentBytes] = math.NaN(), math.NaN()
		}

		r := openResourcesOf(p.PID)
		g := gauges{
			residentBytes:   float64(p.ResidentBytes),
			footprintBytes:  u.FootprintBytes,
			threads:         r.Threads,
			openFiles:       r.Files,
			openSockets:     r.Sockets,
			openDescriptors: r.Descriptors,
		}

		processes = append(processes, process{PID: p.PID, Executable: p.Path, Gauges: g, Counters: c})
	}
	return processes, nil
}
