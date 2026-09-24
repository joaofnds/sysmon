package main

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type netBytes struct {
	Received uint64
	Sent     uint64
}

func networkByPID(ctx context.Context) (map[int]netBytes, error) {
	// Only external interfaces, so traffic between local processes stays out of the totals,
	// as it does in mactop's.
	out, err := exec.CommandContext(ctx, "/usr/bin/nettop", "-P", "-L", "1", "-x", "-t", "external", "-J", "bytes_in,bytes_out").Output()
	if err != nil {
		return nil, fmt.Errorf("running nettop: %w", err)
	}

	return parseNettop(string(out))
}

func parseNettop(out string) (map[int]netBytes, error) {
	byPID := map[int]netBytes{}
	_, rows, _ := strings.Cut(out, "\n")
	for line := range strings.Lines(rows) {
		fields := strings.Split(strings.TrimSpace(line), ",")
		if len(fields) < 3 {
			return nil, fmt.Errorf("nettop line %q: want name.pid, bytes in and bytes out", line)
		}

		dot := strings.LastIndex(fields[0], ".")
		if dot < 0 {
			return nil, fmt.Errorf("nettop line %q: no pid after the name", line)
		}
		pid, err := strconv.Atoi(fields[0][dot+1:])
		if err != nil {
			return nil, fmt.Errorf("nettop line %q: pid: %w", line, err)
		}
		received, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("nettop line %q: bytes in: %w", line, err)
		}
		sent, err := strconv.ParseUint(fields[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("nettop line %q: bytes out: %w", line, err)
		}

		byPID[pid] = netBytes{Received: received, Sent: sent}
	}

	return byPID, nil
}
