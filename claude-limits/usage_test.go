package main

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestParseLimits(t *testing.T) {
	t.Run("reads the session, weekly and per-model limits", func(t *testing.T) {
		body := `{
		  "five_hour": {"utilization": 18.0, "resets_at": "2026-09-24T14:19:59.633159+00:00"},
		  "limits": [
		    {"kind": "session", "group": "session", "percent": 18, "severity": "normal",
		     "resets_at": "2026-09-24T14:19:59.633159+00:00", "scope": null, "is_active": false},
		    {"kind": "weekly_all", "group": "weekly", "percent": 29, "severity": "normal",
		     "resets_at": "2026-09-27T23:59:59.633177+00:00", "scope": null, "is_active": true},
		    {"kind": "weekly_scoped", "group": "weekly", "percent": 13, "severity": "normal",
		     "resets_at": "2026-09-27T23:59:59.633330+00:00",
		     "scope": {"model": {"id": null, "display_name": "Fable"}, "surface": null}, "is_active": false}
		  ]
		}`

		limits, err := parseLimits([]byte(body))

		if err != nil {
			t.Fatal(err)
		}
		want := []limit{
			{Kind: "session", UsedPercent: 18, ResetsAt: time.Date(2026, 9, 24, 14, 19, 59, 633159000, time.UTC)},
			{Kind: "weekly_all", UsedPercent: 29, ResetsAt: time.Date(2026, 9, 27, 23, 59, 59, 633177000, time.UTC)},
			{Kind: "weekly_scoped", Model: "Fable", UsedPercent: 13, ResetsAt: time.Date(2026, 9, 27, 23, 59, 59, 633330000, time.UTC)},
		}
		if !reflect.DeepEqual(limits, want) {
			t.Fatalf("got %+v, want %+v", limits, want)
		}
	})

	t.Run("when a limit is scoped to a surface", func(t *testing.T) {
		t.Run("reads the surface", func(t *testing.T) {
			body := `{"limits": [{"kind": "weekly_scoped", "percent": 4, "resets_at": null,
			  "scope": {"model": null, "surface": {"display_name": "Claude Code"}}}]}`

			limits, err := parseLimits([]byte(body))

			if err != nil {
				t.Fatal(err)
			}
			if want := []limit{{Kind: "weekly_scoped", Surface: "Claude Code", UsedPercent: 4}}; !reflect.DeepEqual(limits, want) {
				t.Fatalf("got %+v, want %+v", limits, want)
			}
		})
	})

	t.Run("when a limit has no reset time", func(t *testing.T) {
		t.Run("leaves the reset time zero", func(t *testing.T) {
			body := `{"limits": [{"kind": "session", "percent": 0, "resets_at": null, "scope": null}]}`

			limits, err := parseLimits([]byte(body))

			if err != nil {
				t.Fatal(err)
			}
			if want := []limit{{Kind: "session"}}; !reflect.DeepEqual(limits, want) {
				t.Fatalf("got %+v, want %+v", limits, want)
			}
		})
	})

	t.Run("when a limit is incomplete", func(t *testing.T) {
		for _, row := range []struct{ name, body string }{
			{"rejects a limit without a kind", `{"limits": [{"percent": 18, "resets_at": null, "scope": null}]}`},
			{"rejects a limit without a percent", `{"limits": [{"kind": "session", "resets_at": null, "scope": null}]}`},
		} {
			t.Run(row.name, func(t *testing.T) {
				_, err := parseLimits([]byte(row.body))

				if !errors.Is(err, errIncompleteLimit) {
					t.Fatalf("got %v, want %v", err, errIncompleteLimit)
				}
			})
		}
	})

	t.Run("when the response has no limits", func(t *testing.T) {
		t.Run("rejects it", func(t *testing.T) {
			_, err := parseLimits([]byte(`{"five_hour": {"utilization": 18.0}}`))

			if !errors.Is(err, errNoLimits) {
				t.Fatalf("got %v, want %v", err, errNoLimits)
			}
		})
	})
}
