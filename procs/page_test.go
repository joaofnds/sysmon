package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const chromeCountLine = `sysmon_process_count{app="Google Chrome",process="Google Chrome"} 1`

var errNettopTimedOut = errors.New("nettop timed out")

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

func (c *fakeClock) advance(d time.Duration) { c.now = c.now.Add(d) }

func newTestPage() (*metricsPage, *fakeClock) {
	clock := &fakeClock{now: time.Date(2026, 9, 27, 1, 0, 0, 0, time.UTC)}
	return newMetricsPage(clock.Now), clock
}

func sampling(processes ...process) sampler {
	return func(context.Context) ([]process, error) { return processes, nil }
}

func failingSample(context.Context) ([]process, error) {
	return nil, errNettopTimedOut
}

func mustRefresh(t *testing.T, page *metricsPage, sample sampler) {
	t.Helper()
	if err := page.refresh(t.Context(), sample); err != nil {
		t.Fatal(err)
	}
}

func scrape(page http.Handler) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	page.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	return response
}

func TestMetricsPage(t *testing.T) {
	t.Run("serves the processes the last sample found", func(t *testing.T) {
		page, _ := newTestPage()
		mustRefresh(t, page, sampling(holding(1, chrome, 4096, 12)))

		response := scrape(page)

		if response.Code != http.StatusOK {
			t.Fatalf("got status %d, want %d", response.Code, http.StatusOK)
		}
		if got, want := response.Header().Get("Content-Type"), "text/plain; version=0.0.4"; got != want {
			t.Fatalf("got Content-Type %q, want %q", got, want)
		}
		if !strings.Contains(response.Body.String(), chromeCountLine) {
			t.Fatalf("missing %s in\n%s", chromeCountLine, response.Body.String())
		}
	})

	t.Run("reports how long the last sample took", func(t *testing.T) {
		page, clock := newTestPage()
		slowSample := func(context.Context) ([]process, error) {
			clock.advance(2500 * time.Millisecond)
			return []process{holding(1, chrome, 4096, 12)}, nil
		}

		mustRefresh(t, page, slowSample)

		want := "sysmon_procs_sample_duration_seconds 2.5\n"
		if body := scrape(page).Body.String(); !strings.Contains(body, want) {
			t.Fatalf("missing %s in\n%s", want, body)
		}
	})

	t.Run("when nothing has been sampled yet", func(t *testing.T) {
		t.Run("answers 503", func(t *testing.T) {
			page, _ := newTestPage()

			response := scrape(page)

			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("got status %d, want %d", response.Code, http.StatusServiceUnavailable)
			}
		})
	})

	t.Run("when the last sample failed", func(t *testing.T) {
		t.Run("answers 503 in place of the processes sampled before", func(t *testing.T) {
			page, _ := newTestPage()
			mustRefresh(t, page, sampling(holding(1, chrome, 4096, 12)))

			err := page.refresh(t.Context(), failingSample)

			if !errors.Is(err, errNettopTimedOut) {
				t.Fatalf("got %v, want %v", err, errNettopTimedOut)
			}
			if response := scrape(page); response.Code != http.StatusServiceUnavailable {
				t.Fatalf("got status %d, want %d", response.Code, http.StatusServiceUnavailable)
			}
		})
	})
}
