package main

import (
	"cmp"
	"fmt"
	"io"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
)

var counterMetrics = [counterKinds]struct{ name, help string }{
	cpuSeconds:           {"sysmon_process_cpu_seconds_total", "CPU time used, in seconds."},
	diskReadBytes:        {"sysmon_process_disk_read_bytes_total", "Bytes read from disk."},
	diskWrittenBytes:     {"sysmon_process_disk_written_bytes_total", "Bytes written to disk."},
	networkReceivedBytes: {"sysmon_process_network_received_bytes_total", "Bytes received over the network."},
	networkSentBytes:     {"sysmon_process_network_sent_bytes_total", "Bytes sent over the network."},
	gpuSeconds:           {"sysmon_process_gpu_seconds_total", "GPU time used, in seconds."},
	energyJoules:         {"sysmon_process_energy_joules_total", "Energy used, in joules, as estimated by macOS."},
	idleWakeups:          {"sysmon_process_idle_wakeups_total", "Times the process woke the CPU from idle."},
	pageIns:              {"sysmon_process_pageins_total", "Memory pages read in from disk."},
}

var gaugeMetrics = [gaugeKinds]struct{ name, help string }{
	residentBytes:   {"sysmon_process_resident_bytes", "Memory held in RAM by running processes."},
	footprintBytes:  {"sysmon_process_footprint_bytes", "Memory footprint of running processes, as Activity Monitor shows it."},
	threads:         {"sysmon_process_threads", "Threads of running processes."},
	openFiles:       {"sysmon_process_open_files", "Files and directories open in running processes."},
	openSockets:     {"sysmon_process_open_sockets", "Sockets open in running processes."},
	openDescriptors: {"sysmon_process_open_descriptors", "File descriptors of every kind open in running processes."},
}

func writeMetrics(w io.Writer, totals map[group]counters, current map[group]usage) {
	groups := sortedGroups(totals)
	for kind, m := range counterMetrics {
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n", m.name, m.help, m.name)
		for _, g := range groups {
			if v := totals[g][kind]; !math.IsNaN(v) {
				fmt.Fprintf(w, "%s%s %s\n", m.name, labels(g), strconv.FormatFloat(v, 'g', -1, 64))
			}
		}
	}

	groups = sortedGroups(current)
	for kind, m := range gaugeMetrics {
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n", m.name, m.help, m.name)
		for _, g := range groups {
			if v := current[g].Gauges[kind]; !math.IsNaN(v) {
				fmt.Fprintf(w, "%s%s %s\n", m.name, labels(g), strconv.FormatFloat(v, 'g', -1, 64))
			}
		}
	}
	fmt.Fprint(w, "# HELP sysmon_process_count Running processes.\n"+
		"# TYPE sysmon_process_count gauge\n")
	for _, g := range groups {
		fmt.Fprintf(w, "sysmon_process_count%s %d\n", labels(g), current[g].Processes)
	}
}

func sortedGroups[V any](m map[group]V) []group {
	return slices.SortedFunc(maps.Keys(m), func(a, b group) int {
		return cmp.Or(cmp.Compare(a.App, b.App), cmp.Compare(a.Process, b.Process))
	})
}

var labelEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)

func labels(g group) string {
	return fmt.Sprintf(`{app="%s",process="%s"}`, labelEscaper.Replace(g.App), labelEscaper.Replace(g.Process))
}
