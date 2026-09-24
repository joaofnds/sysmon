package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	oneSessionLimit = `{"limits": [{"kind": "session", "percent": 18, "resets_at": null, "scope": null}]}`
	sessionLine     = `claude_limit_used_percent{limit="session",model="",surface=""} 18`
)

var errKeychainLocked = errors.New("the Keychain is locked")

func credentialsOf(token string) []byte {
	return []byte(`{"claudeAiOauth": {"accessToken": "` + token + `"}}`)
}

func keychainHolding(token string) func(context.Context) ([]byte, error) {
	return func(context.Context) ([]byte, error) { return credentialsOf(token), nil }
}

func lockedKeychain(context.Context) ([]byte, error) {
	return nil, errKeychainLocked
}

func mustRefresh(t *testing.T, page *metricsPage, credentials func(context.Context) ([]byte, error), api usageAPI) {
	t.Helper()
	if err := page.refresh(t.Context(), credentials, api); err != nil {
		t.Fatal(err)
	}
}

func scrape(page http.Handler) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	page.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	return response
}

func TestMetricsPage(t *testing.T) {
	t.Run("serves the limits the last read found", func(t *testing.T) {
		var page metricsPage
		mustRefresh(t, &page, keychainHolding("access"), stubUsageAPI(t, http.StatusOK, oneSessionLimit))

		response := scrape(&page)

		if response.Code != http.StatusOK {
			t.Fatalf("got status %d, want %d", response.Code, http.StatusOK)
		}
		if got, want := response.Header().Get("Content-Type"), "text/plain; version=0.0.4"; got != want {
			t.Fatalf("got Content-Type %q, want %q", got, want)
		}
		if !strings.Contains(response.Body.String(), sessionLine) {
			t.Fatalf("missing %s in\n%s", sessionLine, response.Body.String())
		}
	})

	t.Run("reads the token again on every refresh", func(t *testing.T) {
		var page metricsPage
		api := stubUsageAPI(t, http.StatusOK, oneSessionLimit)
		item := credentialsOf("expired")
		keychain := func(context.Context) ([]byte, error) { return item, nil }
		_ = page.refresh(t.Context(), keychain, api)
		item = credentialsOf("access")

		err := page.refresh(t.Context(), keychain, api)

		if err != nil {
			t.Fatal(err)
		}
		if response := scrape(&page); response.Code != http.StatusOK {
			t.Fatalf("got status %d, want %d", response.Code, http.StatusOK)
		}
	})

	t.Run("when nothing has been read yet", func(t *testing.T) {
		t.Run("answers 503", func(t *testing.T) {
			var page metricsPage

			response := scrape(&page)

			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("got status %d, want %d", response.Code, http.StatusServiceUnavailable)
			}
		})
	})

	t.Run("when the last read failed", func(t *testing.T) {
		t.Run("answers 503 in place of the limits read before", func(t *testing.T) {
			var page metricsPage
			api := stubUsageAPI(t, http.StatusOK, oneSessionLimit)
			mustRefresh(t, &page, keychainHolding("access"), api)

			err := page.refresh(t.Context(), lockedKeychain, api)

			if !errors.Is(err, errKeychainLocked) {
				t.Fatalf("got %v, want %v", err, errKeychainLocked)
			}
			if response := scrape(&page); response.Code != http.StatusServiceUnavailable {
				t.Fatalf("got status %d, want %d", response.Code, http.StatusServiceUnavailable)
			}
		})
	})
}
