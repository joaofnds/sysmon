package main

import (
	"errors"
	"testing"
)

func TestAccessToken(t *testing.T) {
	t.Run("takes the access token of Claude Code's Claude.ai login", func(t *testing.T) {
		item := `{"claudeAiOauth": {"accessToken": "access", "refreshToken": "refresh", "expiresAt": 1790259600000}}`

		token, err := accessToken([]byte(item))

		if err != nil {
			t.Fatal(err)
		}
		if token != "access" {
			t.Fatalf("got %q, want %q", token, "access")
		}
	})

	t.Run("rejects credentials without a Claude.ai login", func(t *testing.T) {
		_, err := accessToken([]byte(`{"mcpOAuth": {}}`))

		if !errors.Is(err, errNoAccessToken) {
			t.Fatalf("got %v, want %v", err, errNoAccessToken)
		}
	})
}
