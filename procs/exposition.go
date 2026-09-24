package main

import (
	"cmp"
	"fmt"
	"io"
	"maps"
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
}

func writeMetrics(w io.Writer, totals map[group]counters, current map[group]usage) {
	groups := sortedGroups(totals)
	for kind, m := range counterMetrics {
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n", m.name, m.help, m.name)
		for _, g := range groups {
			fmt.Fprintf(w, "%s%s %s\n", m.name, labels(g), strconv.FormatFloat(totals[g][kind], 'g', -1, 64))
		}
	}

	groups = sortedGroups(current)
	fmt.Fprint(w, "# HELP sysmon_process_resident_bytes Memory held in RAM by running processes.\n"+
		"# TYPE sysmon_process_resident_bytes gauge\n")
	for _, g := range groups {
		fmt.Fprintf(w, "sysmon_process_resident_bytes%s %d\n", labels(g), current[g].ResidentBytes)
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
