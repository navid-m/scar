package logger

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestDebug(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	oldStderr := os.Stderr
	defer func() { os.Stderr = oldStderr }()
	os.Stderr = w
	Debug("convertMethodCallToC called with: '%s'", "root.sum()")
	w.Close()
	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	if err != nil {
		t.Fatalf("Failed to read from pipe: %v", err)
	}
	expectedOutput := "Debug: convertMethodCallToC called with: 'root.sum()'"
	if !strings.Contains(buf.String(), expectedOutput) {
		t.Errorf("Expected output to contain '%s', got '%s'", expectedOutput, buf.String())
	}
}
