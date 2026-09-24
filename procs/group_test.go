package main

import "testing"

func TestGroupOf(t *testing.T) {
	for _, row := range []struct {
		name string
		path string
		want group
	}{
		{"groups a helper under its outermost app bundle",
			"/Applications/Google Chrome.app/Contents/Frameworks/Google Chrome Framework.framework/Versions/1/Helpers/Google Chrome Helper (Renderer).app/Contents/MacOS/Google Chrome Helper (Renderer)",
			group{App: "Google Chrome", Process: "Google Chrome Helper (Renderer)"}},
		{"groups an app extension under the app that ships it",
			"/System/Volumes/Preboot/Cryptexes/App/System/Applications/Safari.app/Contents/Extensions/SafariWidgetExtension.appex/Contents/MacOS/SafariWidgetExtension",
			group{App: "Safari", Process: "SafariWidgetExtension"}},
		{"names a process outside any app bundle after its executable",
			"/usr/libexec/logd",
			group{App: "logd", Process: "logd"}},
		{"ignores bundles that are not apps",
			"/System/Library/Frameworks/Speech.framework/Versions/A/XPCServices/localspeechrecognition.xpc/Contents/MacOS/localspeechrecognition",
			group{App: "localspeechrecognition", Process: "localspeechrecognition"}},
		{"takes a bare command name as is",
			"claude",
			group{App: "claude", Process: "claude"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			if got := groupOf(row.path); got != row.want {
				t.Fatalf("got %+v, want %+v", got, row.want)
			}
		})
	}
}
