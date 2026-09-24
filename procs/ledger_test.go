package main

import (
	"maps"
	"math"
	"testing"
)

const chrome = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

var chromeGroup = groupOf(chrome)

func running(pid int, executable string, cpu float64) process {
	var c counters
	c[cpuSeconds] = cpu
	return process{PID: pid, Executable: executable, Counters: c}
}

func holding(pid int, executable string, resident, threadCount float64) process {
	p := running(pid, executable, 0)
	p.Gauges = unreadGauges()
	p.Gauges[residentBytes], p.Gauges[threads] = resident, threadCount
	return p
}

func cpuOf(l *ledger, g group) float64 {
	return l.Totals()[g][cpuSeconds]
}

func TestLedger(t *testing.T) {
	t.Run("starts every app it first sees at zero", func(t *testing.T) {
		l := newLedger()

		l.Record([]process{running(1, chrome, 50)})

		if got := l.Totals(); !maps.Equal(got, map[group]counters{chromeGroup: {}}) {
			t.Fatalf("got %v", got)
		}
	})

	t.Run("adds what a running process used since the last record", func(t *testing.T) {
		l := newLedger()
		l.Record([]process{running(1, chrome, 50)})

		l.Record([]process{running(1, chrome, 53)})

		if got := cpuOf(l, chromeGroup); got != 3 {
			t.Fatalf("got %v, want 3", got)
		}
	})

	t.Run("adds everything a process started since the last record used", func(t *testing.T) {
		l := newLedger()
		l.Record([]process{running(1, chrome, 50)})

		l.Record([]process{running(1, chrome, 50), running(2, chrome, 4)})

		if got := cpuOf(l, chromeGroup); got != 4 {
			t.Fatalf("got %v, want 4", got)
		}
	})

	t.Run("keeps what an exited process used", func(t *testing.T) {
		l := newLedger()
		l.Record([]process{running(1, chrome, 50)})
		l.Record([]process{running(1, chrome, 53)})

		l.Record(nil)

		if got := cpuOf(l, chromeGroup); got != 3 {
			t.Fatalf("got %v, want 3", got)
		}
	})

	t.Run("sums what the running processes of each app hold, and counts them", func(t *testing.T) {
		l := newLedger()

		l.Record([]process{holding(1, chrome, 100, 7), holding(2, chrome, 20, 3)})

		got := l.Usage()[chromeGroup]
		if got.Gauges[residentBytes] != 120 || got.Gauges[threads] != 10 || got.Processes != 2 {
			t.Fatalf("got %v", got)
		}
	})

	t.Run("when a counter shrank", func(t *testing.T) {
		t.Run("adds nothing for it", func(t *testing.T) {
			l := newLedger()
			l.Record([]process{running(1, chrome, 50)})

			l.Record([]process{running(1, chrome, 40)})

			if got := cpuOf(l, chromeGroup); got != 0 {
				t.Fatalf("got %v, want 0", got)
			}
		})
	})

	t.Run("when a pid is reused by another executable", func(t *testing.T) {
		t.Run("counts it as a new process", func(t *testing.T) {
			l := newLedger()
			l.Record([]process{running(1, "/usr/libexec/logd", 50)})

			l.Record([]process{running(1, chrome, 2)})

			if got := cpuOf(l, chromeGroup); got != 2 {
				t.Fatalf("got %v, want 2", got)
			}
		})
	})

	t.Run("when a counter was not read", func(t *testing.T) {
		t.Run("keeps its baseline until it is read again", func(t *testing.T) {
			l := newLedger()
			l.Record([]process{running(1, chrome, 50)})
			l.Record([]process{running(1, chrome, math.NaN())})

			l.Record([]process{running(1, chrome, 53)})

			if got := cpuOf(l, chromeGroup); got != 3 {
				t.Fatalf("got %v, want 3", got)
			}
		})

		t.Run("only sets its baseline when a process seen before is first read", func(t *testing.T) {
			l := newLedger()
			l.Record([]process{running(1, chrome, math.NaN())})

			l.Record([]process{running(1, chrome, 50)})

			if got := cpuOf(l, chromeGroup); got != 0 {
				t.Fatalf("got %v, want 0", got)
			}
		})

		t.Run("leaves it unread for an app none of whose processes were read", func(t *testing.T) {
			l := newLedger()
			l.Record(nil)

			l.Record([]process{running(1, chrome, math.NaN())})

			if got := cpuOf(l, chromeGroup); !math.IsNaN(got) {
				t.Fatalf("got %v, want NaN", got)
			}
		})

		t.Run("adds what the processes that were read used", func(t *testing.T) {
			l := newLedger()
			l.Record([]process{running(1, chrome, 10), running(2, chrome, 10), running(3, chrome, 10)})

			l.Record([]process{running(1, chrome, 13), running(2, chrome, math.NaN()), running(3, chrome, 14)})

			if got := cpuOf(l, chromeGroup); got != 7 {
				t.Fatalf("got %v, want 7", got)
			}
		})
	})

	t.Run("when a gauge was not read", func(t *testing.T) {
		t.Run("leaves it unread for an app none of whose processes were read", func(t *testing.T) {
			l := newLedger()

			l.Record([]process{holding(1, chrome, 100, math.NaN())})

			if got := l.Usage()[chromeGroup].Gauges[threads]; !math.IsNaN(got) {
				t.Fatalf("got %v, want NaN", got)
			}
		})

		t.Run("sums the processes that were read", func(t *testing.T) {
			l := newLedger()

			l.Record([]process{holding(1, chrome, 1, 3), holding(2, chrome, 1, math.NaN()), holding(3, chrome, 1, 4)})

			if got := l.Usage()[chromeGroup].Gauges[threads]; got != 7 {
				t.Fatalf("got %v, want 7", got)
			}
		})
	})
}
