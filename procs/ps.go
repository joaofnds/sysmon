package main

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type psProcess struct {
	PID           int
	Path          string
	CPUSeconds    float64
	ResidentBytes uint64
}

func listProcesses(ctx context.Context) ([]psProcess, error) {
	out, err := exec.CommandContext(ctx, "/bin/ps", "-axo", "pid=,time=,rss=,comm=").Output()
	if err != nil {
		return nil, fmt.Errorf("running ps: %w", err)
	}

	return parsePS(string(out))
}

func parsePS(out string) ([]psProcess, error) {
	var procs []psProcess
	for line := range strings.Lines(out) {
		pidField, rest := cutField(line)
		timeField, rest := cutField(rest)
		rssField, path := cutField(rest)
		path = strings.TrimSpace(path)
		if path == "" {
			return nil, fmt.Errorf("ps line %q: want pid, time, rss and command", line)
		}

		pid, err := strconv.Atoi(pidField)
		if err != nil {
			return nil, fmt.Errorf("ps line %q: pid: %w", line, err)
		}
		cpu, err := parseCPUTime(timeField)
		if err != nil {
			return nil, fmt.Errorf("ps line %q: %w", line, err)
		}
		kib, err := strconv.ParseUint(rssField, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("ps line %q: rss: %w", line, err)
		}

		procs = append(procs, psProcess{PID: pid, Path: path, CPUSeconds: cpu, ResidentBytes: kib * 1024})
	}

	return procs, nil
}

func cutField(s string) (field, rest string) {
	s = strings.TrimLeft(s, " ")
	field, rest, _ = strings.Cut(s, " ")
	return field, rest
}

func parseCPUTime(s string) (float64, error) {
	minutes, seconds, ok := strings.Cut(s, ":")
	if !ok {
		return 0, fmt.Errorf("cpu time %q: want minutes:seconds", s)
	}

	m, err := strconv.Atoi(minutes)
	if err != nil {
		return 0, fmt.Errorf("cpu time %q: %w", s, err)
	}
	sec, err := strconv.ParseFloat(seconds, 64)
	if err != nil {
		return 0, fmt.Errorf("cpu time %q: %w", s, err)
	}

	return float64(m*60) + sec, nil
}
