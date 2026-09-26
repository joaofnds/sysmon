package main

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

var readAt = time.Date(2026, 9, 27, 1, 0, 0, 0, time.UTC)

func readOnce(t *testing.T, reads *readsPage, read func(t *testing.T) error) {
	t.Helper()
	reads.record(read(t), time.Second, readAt)
}

func succeeding(t *testing.T) error {
	var page metricsPage
	return page.refresh(t.Context(), keychainHolding("access"), stubUsageAPI(t, http.StatusOK, oneSessionLimit))
}

func answeredWith(status int, body string) func(t *testing.T) error {
	return func(t *testing.T) error {
		var page metricsPage
		return page.refresh(t.Context(), keychainHolding("access"), stubUsageAPI(t, status, body))
	}
}

func withoutLogin(t *testing.T) error {
	var page metricsPage
	return page.refresh(t.Context(), lockedKeychain, stubUsageAPI(t, http.StatusOK, oneSessionLimit))
}

func assertServes(t *testing.T, reads *readsPage, want string) {
	t.Helper()
	response := scrape(reads)
	if response.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), want) {
		t.Fatalf("missing %s in\n%s", want, response.Body.String())
	}
}

func TestReadsPage(t *testing.T) {
	t.Run("counts each read by its outcome", func(t *testing.T) {
		var reads readsPage

		readOnce(t, &reads, succeeding)
		readOnce(t, &reads, succeeding)

		assertServes(t, &reads, `claude_limits_reads_total{outcome="ok"} 2`+"\n")
	})

	t.Run("counts a read the API refused under its status code", func(t *testing.T) {
		var reads readsPage

		readOnce(t, &reads, answeredWith(http.StatusTooManyRequests, `{"type":"error"}`))

		assertServes(t, &reads, `claude_limits_reads_total{outcome="http_429"} 1`+"\n")
	})

	t.Run("counts a read without a Claude.ai login as no_login", func(t *testing.T) {
		var reads readsPage

		readOnce(t, &reads, withoutLogin)

		assertServes(t, &reads, `claude_limits_reads_total{outcome="no_login"} 1`+"\n")
	})

	t.Run("counts any other failed read as failed", func(t *testing.T) {
		var reads readsPage

		readOnce(t, &reads, answeredWith(http.StatusOK, `{}`))

		assertServes(t, &reads, `claude_limits_reads_total{outcome="failed"} 1`+"\n")
	})

	t.Run("reports the outcome of the last read alone", func(t *testing.T) {
		var reads readsPage
		readOnce(t, &reads, succeeding)

		readOnce(t, &reads, answeredWith(http.StatusTooManyRequests, `{"type":"error"}`))

		assertServes(t, &reads, `claude_limits_last_read_outcome{outcome="http_429"} 1`+"\n")
		if body := scrape(&reads).Body.String(); strings.Contains(body, `claude_limits_last_read_outcome{outcome="ok"}`) {
			t.Fatalf("still reports an earlier outcome in\n%s", body)
		}
	})

	t.Run("reports a success after a refused read", func(t *testing.T) {
		var reads readsPage
		readOnce(t, &reads, answeredWith(http.StatusUnauthorized, `{"type":"error"}`))

		readOnce(t, &reads, succeeding)

		assertServes(t, &reads, `claude_limits_last_read_outcome{outcome="ok"} 1`+"\n")
	})

	t.Run("reports when a read last succeeded", func(t *testing.T) {
		var reads readsPage
		reads.record(succeeding(t), time.Second, readAt)

		reads.record(withoutLogin(t), time.Second, readAt.Add(5*time.Minute))

		assertServes(t, &reads, "claude_limits_last_success_timestamp_seconds 1790470800\n")
	})

	t.Run("reports how long the last read took", func(t *testing.T) {
		var reads readsPage

		reads.record(succeeding(t), 420*time.Millisecond, readAt)

		assertServes(t, &reads, "claude_limits_last_read_duration_seconds 0.42\n")
	})

	t.Run("when no read has happened yet", func(t *testing.T) {
		t.Run("leaves out the last outcome", func(t *testing.T) {
			var reads readsPage

			if body := scrape(&reads).Body.String(); strings.Contains(body, "claude_limits_last_read_outcome{") {
				t.Fatalf("wrote a last outcome in\n%s", body)
			}
		})
	})

	t.Run("when no read has succeeded", func(t *testing.T) {
		t.Run("leaves out the last success", func(t *testing.T) {
			var reads readsPage

			readOnce(t, &reads, withoutLogin)

			if body := scrape(&reads).Body.String(); strings.Contains(body, "claude_limits_last_success_timestamp_seconds ") {
				t.Fatalf("wrote a last success in\n%s", body)
			}
		})
	})
}
