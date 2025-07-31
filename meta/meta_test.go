package meta

import "testing"

func TestUsage(t *testing.T) {
	if ShowUsage() != nil {
		t.Error("This failed for some reason.")
	}
}
