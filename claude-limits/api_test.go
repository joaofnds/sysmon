package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// fakeUsageAPI answers like Anthropic's usage endpoint does for the token "access": 401
// without that token or the OAuth beta header, else the given status and body.
func fakeUsageAPI(t *testing.T, status int, body string) usageAPI {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/oauth/usage" ||
			r.Header.Get("Authorization") != "Bearer access" ||
			r.Header.Get("anthropic-beta") != "oauth-2025-04-20" {
			http.Error(w, `{"type":"error"}`, http.StatusUnauthorized)
			return
		}
		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	return usageAPI{client: server.Client(), baseURL: server.URL}
}

func TestUsageAPI(t *testing.T) {
	t.Run("reads the limits of the token's plan", func(t *testing.T) {
		api := fakeUsageAPI(t, http.StatusOK, `{"limits": [{"kind": "session", "percent": 18, "resets_at": null, "scope": null}]}`)

		limits, err := api.limits(t.Context(), "access")

		if err != nil {
			t.Fatal(err)
		}
		if want := []limit{{Kind: "session", UsedPercent: 18}}; !reflect.DeepEqual(limits, want) {
			t.Fatalf("got %+v, want %+v", limits, want)
		}
	})

	t.Run("rejects an answer other than 200", func(t *testing.T) {
		api := fakeUsageAPI(t, http.StatusTooManyRequests, `{"type":"error"}`)

		_, err := api.limits(t.Context(), "access")

		if !errors.Is(err, errUsageAPI) {
			t.Fatalf("got %v, want %v", err, errUsageAPI)
		}
	})
}
