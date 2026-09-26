package main

import (
	"bytes"
	"context"
	"net/http"
	"sync/atomic"
	"time"
)

type sampler func(context.Context) ([]process, error)

type metricsPage struct {
	body   atomic.Pointer[[]byte]
	ledger *ledger
	now    func() time.Time
}

func newMetricsPage(now func() time.Time) *metricsPage {
	return &metricsPage{ledger: newLedger(), now: now}
}

func (p *metricsPage) refresh(ctx context.Context, sample sampler) error {
	start := p.now()
	processes, err := sample(ctx)
	took := p.now().Sub(start)
	if err != nil {
		p.body.Store(nil)
		return err
	}

	p.ledger.Record(processes)
	var b bytes.Buffer
	writeMetrics(&b, p.ledger.Totals(), p.ledger.Usage())
	writeSampleDuration(&b, took)
	body := b.Bytes()
	p.body.Store(&body)
	return nil
}

func (p *metricsPage) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	body := p.body.Load()
	if body == nil {
		http.Error(w, "no current sample of the processes", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = w.Write(*body)
}
