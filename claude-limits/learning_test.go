//go:build learning

package main

import (
	"errors"
	"slices"
	"testing"
)

func TestUsageAPILearning(t *testing.T) {
	api := anthropicUsageAPI()

	t.Run("reads the session and weekly limits of this Mac's plan", func(t *testing.T) {
		limits, err := readLimits(t.Context(), keychainCredentials, api)

		if err != nil {
			t.Fatal(err)
		}
		hasKind := func(kind string) bool {
			return slices.ContainsFunc(limits, func(l limit) bool { return l.Kind == kind })
		}
		if !hasKind("session") || !hasKind("weekly_all") {
			t.Fatalf("got %+v, want a session and a weekly_all limit", limits)
		}
	})

	t.Run("when the token is unknown", func(t *testing.T) {
		t.Run("reports the error status", func(t *testing.T) {
			_, err := api.limits(t.Context(), "unknown")

			if !errors.Is(err, errUsageAPI) {
				t.Fatalf("got %v, want %v", err, errUsageAPI)
			}
		})
	})
}
