// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Post C conversion macros.
// This file contains functions to insert macros and process expressions in C code.

package preprocessor

import "strings"

func insertLen(output string) string {
	return "#define len(x) (sizeof(x) / sizeof((x)[0]))\n" + output
}

func InsertMacros(output string) string {
	outp := output
	if strings.Contains(output, "len") {
		outp = insertLen(outp)
	}
	return outp
}
