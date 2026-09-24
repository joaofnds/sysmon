package main

import (
	"maps"
	"testing"
)

func TestParseNettop(t *testing.T) {
	t.Run("reads bytes received and sent by pid", func(t *testing.T) {
		out := ",bytes_in,bytes_out,\nkernel_task.0,160296,118636,\nmDNSResponder.679,80647069,53556445,\n"

		got, err := parseNettop(out)

		want := map[int]netBytes{0: {Received: 160296, Sent: 118636}, 679: {Received: 80647069, Sent: 53556445}}
		if err != nil || !maps.Equal(got, want) {
			t.Fatalf("got %v, %v, want %v", got, err, want)
		}
	})

	t.Run("takes the pid after the last dot of the name", func(t *testing.T) {
		got, err := parseNettop(",bytes_in,bytes_out,\ncom.apple.Safa.1234,10,20,\n")

		if err != nil || got[1234] != (netBytes{Received: 10, Sent: 20}) {
			t.Fatalf("got %v, %v", got, err)
		}
	})

	t.Run("when a line is malformed", func(t *testing.T) {
		t.Run("returns an error", func(t *testing.T) {
			_, err := parseNettop(",bytes_in,bytes_out,\nkernel_task,1,2,\n")

			if err == nil {
				t.Fatal("got no error")
			}
		})
	})
}
