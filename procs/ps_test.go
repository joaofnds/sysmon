package main

import (
	"reflect"
	"testing"
)

func TestParsePS(t *testing.T) {
	t.Run("reads pid, cpu time, resident memory and executable path", func(t *testing.T) {
		out := "  541  70:56.85  92608 /usr/libexec/logd\n"

		got, err := parsePS(out)

		want := []psProcess{{PID: 541, Path: "/usr/libexec/logd", CPUSeconds: 70*60 + 56.85, ResidentBytes: 92608 * 1024}}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, %v, want %+v", got, err, want)
		}
	})

	t.Run("keeps spaces inside the path", func(t *testing.T) {
		got, err := parsePS("  900   0:01.00   100 /Applications/Google Chrome.app/Contents/MacOS/Google Chrome\n")

		if err != nil || got[0].Path != "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" {
			t.Fatalf("got %+v, %v", got, err)
		}
	})

	t.Run("reads the path when the pid equals the resident kilobytes", func(t *testing.T) {
		got, err := parsePS("  736   0:00.05    736 /usr/libexec/autofsd\n")

		if err != nil || got[0].Path != "/usr/libexec/autofsd" {
			t.Fatalf("got %+v, %v", got, err)
		}
	})

	t.Run("reads cpu time past a thousand minutes", func(t *testing.T) {
		got, err := parsePS("    1 2848:04.27  30912 /sbin/launchd\n")

		if err != nil || got[0].CPUSeconds != 2848*60+4.27 {
			t.Fatalf("got %+v, %v", got, err)
		}
	})

	t.Run("when a line is malformed", func(t *testing.T) {
		t.Run("returns an error", func(t *testing.T) {
			_, err := parsePS("  541  soon  92608 /usr/libexec/logd\n")

			if err == nil {
				t.Fatal("got no error")
			}
		})
	})
}
