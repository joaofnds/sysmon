package main

import (
	"path"
	"strings"
)

type group struct {
	App     string
	Process string
}

func groupOf(executable string) group {
	process := path.Base(executable)
	for part := range strings.SplitSeq(executable, "/") {
		if app, ok := strings.CutSuffix(part, ".app"); ok {
			return group{App: app, Process: process}
		}
	}

	return group{App: process, Process: process}
}
