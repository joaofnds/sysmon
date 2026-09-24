package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

func writeMetrics(w io.Writer, limits []limit) {
	fmt.Fprint(w, "# HELP claude_limit_used_percent Share of the limit used so far, in percent.\n"+
		"# TYPE claude_limit_used_percent gauge\n")
	for _, l := range limits {
		fmt.Fprintf(w, "claude_limit_used_percent%s %s\n", labels(l), strconv.FormatFloat(l.UsedPercent, 'g', -1, 64))
	}

	fmt.Fprint(w, "# HELP claude_limit_reset_timestamp_seconds When the limit resets, in seconds since the Unix epoch.\n"+
		"# TYPE claude_limit_reset_timestamp_seconds gauge\n")
	for _, l := range limits {
		if l.ResetsAt.IsZero() {
			continue
		}
		fmt.Fprintf(w, "claude_limit_reset_timestamp_seconds%s %d\n", labels(l), l.ResetsAt.Unix())
	}
}

var labelEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)

func labels(l limit) string {
	return fmt.Sprintf(`{limit="%s",model="%s",surface="%s"}`,
		labelEscaper.Replace(l.Kind), labelEscaper.Replace(l.Model), labelEscaper.Replace(l.Surface))
}
