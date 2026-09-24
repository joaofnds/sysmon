package main

import (
	"strings"
	"testing"
)

func TestWriteMetrics(t *testing.T) {
	t.Run("writes each counter and gauge by app and process, sorted", func(t *testing.T) {
		logd := group{App: "logd", Process: "logd"}
		var chromeTotal counters
		chromeTotal[cpuSeconds] = 1.5
		chromeTotal[networkSentBytes] = 2048
		var b strings.Builder

		writeMetrics(&b,
			map[group]counters{logd: {}, chromeGroup: chromeTotal},
			map[group]usage{chromeGroup: {ResidentBytes: 4096, Processes: 3}})

		for _, want := range []string{
			"# TYPE sysmon_process_cpu_seconds_total counter\n" +
				`sysmon_process_cpu_seconds_total{app="Google Chrome",process="Google Chrome"} 1.5` + "\n" +
				`sysmon_process_cpu_seconds_total{app="logd",process="logd"} 0` + "\n",
			`sysmon_process_network_sent_bytes_total{app="Google Chrome",process="Google Chrome"} 2048` + "\n",
			"# TYPE sysmon_process_resident_bytes gauge\n" +
				`sysmon_process_resident_bytes{app="Google Chrome",process="Google Chrome"} 4096` + "\n" +
				"# HELP sysmon_process_count",
			`sysmon_process_count{app="Google Chrome",process="Google Chrome"} 3` + "\n",
		} {
			if !strings.Contains(b.String(), want) {
				t.Errorf("missing\n%s\nin\n%s", want, b.String())
			}
		}
	})

	t.Run("escapes backslashes, quotes and newlines in label values", func(t *testing.T) {
		odd := group{App: `a\b"c` + "\nd", Process: "p"}
		var b strings.Builder

		writeMetrics(&b, nil, map[group]usage{odd: {Processes: 1}})

		want := `sysmon_process_count{app="a\\b\"c\nd",process="p"} 1`
		if !strings.Contains(b.String(), want) {
			t.Fatalf("missing %s in\n%s", want, b.String())
		}
	})
}
