package main

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"
)

// readsPage serves how the collector's reads of the usage API went. It answers even while
// the reads fail, which is when the metrics page answers 503.
type readsPage struct {
	mu          sync.Mutex
	outcomes    map[string]int
	lastSuccess time.Time
	lastTook    time.Duration
}

func (r *readsPage) record(err error, took time.Duration, at time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.outcomes == nil {
		r.outcomes = map[string]int{}
	}
	r.outcomes[outcomeOf(err)]++
	r.lastTook = took
	if err == nil {
		r.lastSuccess = at
	}
}

func outcomeOf(err error) string {
	if err == nil {
		return "ok"
	}

	if refused, ok := errors.AsType[statusError](err); ok {
		return "http_" + strconv.Itoa(refused.code)
	}

	if errors.Is(err, errNoAccessToken) {
		return "no_login"
	}

	return "failed"
}

func (r *readsPage) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprint(w, "# HELP claude_limits_reads_total Reads of the usage API since the collector started, by outcome.\n"+
		"# TYPE claude_limits_reads_total counter\n")
	for _, outcome := range slices.Sorted(maps.Keys(r.outcomes)) {
		fmt.Fprintf(w, "claude_limits_reads_total{outcome=%q} %d\n", outcome, r.outcomes[outcome])
	}

	fmt.Fprintf(w, "# HELP claude_limits_last_read_duration_seconds How long the last read of the usage API took.\n"+
		"# TYPE claude_limits_last_read_duration_seconds gauge\n"+
		"claude_limits_last_read_duration_seconds %s\n", strconv.FormatFloat(r.lastTook.Seconds(), 'g', -1, 64))

	if !r.lastSuccess.IsZero() {
		fmt.Fprintf(w, "# HELP claude_limits_last_success_timestamp_seconds When a read of the usage API last succeeded, in seconds since the Unix epoch.\n"+
			"# TYPE claude_limits_last_success_timestamp_seconds gauge\n"+
			"claude_limits_last_success_timestamp_seconds %d\n", r.lastSuccess.Unix())
	}
}
