package logger

import (
	"fmt"
	"os"
)

var Loud = true

func Debug(msg string, args ...any) {
	if Loud {
		if len(args) > 0 {
			msg = fmt.Sprintf(msg, args...)
		}
		fmt.Fprintf(os.Stderr, "Debug: %s", msg)
	}
}
