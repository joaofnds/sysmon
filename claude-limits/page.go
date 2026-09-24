package main

import (
	"bytes"
	"context"
	"net/http"
	"sync/atomic"
)

type metricsPage struct {
	body atomic.Pointer[[]byte]
}

func (p *metricsPage) refresh(ctx context.Context, credentials func(context.Context) ([]byte, error), api usageAPI) error {
	limits, err := readLimits(ctx, credentials, api)
	if err != nil {
		p.body.Store(nil)
		return err
	}

	var b bytes.Buffer
	writeMetrics(&b, limits)
	body := b.Bytes()
	p.body.Store(&body)
	return nil
}

func (p *metricsPage) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	body := p.body.Load()
	if body == nil {
		http.Error(w, "the last read of the usage API failed", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = w.Write(*body)
}

func readLimits(ctx context.Context, credentials func(context.Context) ([]byte, error), api usageAPI) ([]limit, error) {
	item, err := credentials(ctx)
	if err != nil {
		return nil, err
	}

	token, err := accessToken(item)
	if err != nil {
		return nil, err
	}

	return api.limits(ctx, token)
}
