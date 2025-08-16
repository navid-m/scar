// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Contains logger functionality for the scar programming language.

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

func ErrorAndExit(msg string, args ...any) {
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}
	fmt.Fprintf(os.Stderr, "\033[31m%s\033[0m\n", msg)
	os.Exit(1)
}
