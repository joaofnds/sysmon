package main

import (
	"maps"
	"math"
)

type counter int

const (
	cpuSeconds counter = iota
	diskReadBytes
	diskWrittenBytes
	networkReceivedBytes
	networkSentBytes
	gpuSeconds
	energyJoules
	idleWakeups
	pageIns
	counterKinds
)

// counters holds a process's counters, NaN where a sample could not read one.
type counters [counterKinds]float64

func unreadCounters() counters {
	var c counters
	for i := range c {
		c[i] = math.NaN()
	}
	return c
}

type gauge int

const (
	residentBytes gauge = iota
	footprintBytes
	threads
	openFiles
	openSockets
	openDescriptors
	gaugeKinds
)

// gauges holds what a process holds right now, NaN where a sample could not read it.
type gauges [gaugeKinds]float64

func unreadGauges() gauges {
	var g gauges
	for i := range g {
		g[i] = math.NaN()
	}
	return g
}

// addRead sums the values other has read into into, leaving a value unread only while
// neither has read it.
func addRead(into, other []float64) {
	for i, v := range other {
		switch {
		case math.IsNaN(v):
		case math.IsNaN(into[i]):
			into[i] = v
		default:
			into[i] += v
		}
	}
}

type process struct {
	PID        int
	Executable string
	Gauges     gauges
	Counters   counters
}

type usage struct {
	Gauges    gauges
	Processes int
}

// ledger turns the lifetime counters of individual processes into totals per group that
// keep growing after a process exits, so the rate of a group stays right as its processes
// come and go.
type ledger struct {
	recorded bool
	previous map[int]process
	totals   map[group]counters
	usage    map[group]usage
}

func newLedger() *ledger {
	return &ledger{previous: map[int]process{}, totals: map[group]counters{}}
}

// Record adds what each process used since the previous record. The first record only
// sets the baseline, since its counters hold everything used before the ledger started.
func (l *ledger) Record(processes []process) {
	current := make(map[int]process, len(processes))
	l.usage = map[group]usage{}
	for _, p := range processes {
		used, baseline := l.usedSince(p)
		current[p.PID] = baseline
		g := groupOf(p.Executable)

		total, ok := l.totals[g]
		if !ok {
			total = unreadCounters()
		}
		addRead(total[:], used[:])
		l.totals[g] = total

		u, ok := l.usage[g]
		if !ok {
			u.Gauges = unreadGauges()
		}
		addRead(u.Gauges[:], p.Gauges[:])
		u.Processes++
		l.usage[g] = u
	}
	l.previous = current
	l.recorded = true
}

// usedSince returns what p used since the previous record, and the counters to measure
// the next record against. A counter that was not read comes back unread and keeps its
// previous value as the baseline. One that shrank, or that is read for the first time on
// a process seen before, adds nothing and starts over, so a process running since before
// the ledger started never adds its lifetime at once.
func (l *ledger) usedSince(p process) (used counters, baseline process) {
	before, seen := l.previous[p.PID]
	seen = seen && before.Executable == p.Executable
	baseline = p
	for i, now := range p.Counters {
		switch {
		case math.IsNaN(now):
			used[i] = now
			if seen {
				baseline.Counters[i] = before.Counters[i]
			}
		case !seen:
			if l.recorded {
				used[i] = now
			}
		default:
			if grown := now - before.Counters[i]; grown > 0 {
				used[i] = grown
			}
		}
	}
	return used, baseline
}

func (l *ledger) Totals() map[group]counters { return maps.Clone(l.totals) }

func (l *ledger) Usage() map[group]usage { return maps.Clone(l.usage) }
