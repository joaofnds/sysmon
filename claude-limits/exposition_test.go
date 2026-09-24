package main

import (
	"strings"
	"testing"
	"time"
)

func TestWriteMetrics(t *testing.T) {
	t.Run("writes each limit's used percent and reset time", func(t *testing.T) {
		var b strings.Builder

		writeMetrics(&b, []limit{
			{Kind: "session", UsedPercent: 18, ResetsAt: time.Unix(1790259600, 974_000_000)},
			{Kind: "weekly_scoped", Model: "Fable", UsedPercent: 13, ResetsAt: time.Unix(1790553600, 0)},
		})

		want := "# HELP claude_limit_used_percent Share of the limit used so far, in percent.\n" +
			"# TYPE claude_limit_used_percent gauge\n" +
			`claude_limit_used_percent{limit="session",model=""} 18` + "\n" +
			`claude_limit_used_percent{limit="weekly_scoped",model="Fable"} 13` + "\n" +
			"# HELP claude_limit_reset_timestamp_seconds When the limit resets, in seconds since the Unix epoch.\n" +
			"# TYPE claude_limit_reset_timestamp_seconds gauge\n" +
			`claude_limit_reset_timestamp_seconds{limit="session",model=""} 1790259600` + "\n" +
			`claude_limit_reset_timestamp_seconds{limit="weekly_scoped",model="Fable"} 1790553600` + "\n"
		if b.String() != want {
			t.Fatalf("got\n%s\nwant\n%s", b.String(), want)
		}
	})

	t.Run("leaves out the reset time of a limit that has none", func(t *testing.T) {
		var b strings.Builder

		writeMetrics(&b, []limit{{Kind: "session"}})

		if strings.Contains(b.String(), "claude_limit_reset_timestamp_seconds{") {
			t.Fatalf("wrote a reset time in\n%s", b.String())
		}
	})

	t.Run("escapes backslashes, quotes and newlines in label values", func(t *testing.T) {
		var b strings.Builder

		writeMetrics(&b, []limit{{Kind: `weekly"scoped`, Model: `a\b"c` + "\nd", UsedPercent: 1}})

		want := `claude_limit_used_percent{limit="weekly\"scoped",model="a\\b\"c\nd"} 1`
		if !strings.Contains(b.String(), want) {
			t.Fatalf("missing %s in\n%s", want, b.String())
		}
	})
}
