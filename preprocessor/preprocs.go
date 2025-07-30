package preprocessor

import "strings"

func insertLen(output string) string {
	return "#define len(x) (sizeof(x) / sizeof((x)[0]))\n" + output
}

func InsertMacros(output string) string {
	outp := output
	if strings.ContainsAny(output, "len") {
		outp = insertLen(outp)
	}
	return outp
}
