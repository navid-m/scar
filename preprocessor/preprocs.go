// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Post C conversion macros.
// This file contains functions to insert macros and process expressions in C code.

package preprocessor

import (
	"regexp"
	"strings"
)

func InsertMacros(output string) string {
	outp := output
	if strings.Contains(output, "nil") {
		outp = insertNilMacro(outp)
	}
	if strings.Contains(output, "len") {
		outp = insertLen(outp)
	}
	if strings.Contains(output, "ord") {
		outp = insertOrd(outp)
	}
	if strings.Contains(output, "rand") {
		outp = replaceRandCalls(outp)
		outp = insertRand(outp)
	}
	outp = replaceOutsideLiterals(outp, "this.", "this->")
	return outp
}

func replaceOutsideLiterals(s, old, new string) string {
	var result strings.Builder
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' && (i == 0 || s[i-1] != '\\') {
			inString = !inString
		}
		if !inString && i+len(old) <= len(s) && s[i:i+len(old)] == old {
			result.WriteString(new)
			i += len(old) - 1
		} else {
			result.WriteByte(c)
		}
		escaped = !escaped && c == '\\'
	}
	return result.String()
}

func insertNilMacro(output string) string {
	return "#define nil NULL\n" + output
}

func insertLen(output string) string {
	return "#define len(x) (sizeof(x) / sizeof((x)[0]))\n" + output
}

func insertOrd(output string) string {
	return "#define ord(x) ((int)(x))\n" + output
}

func insertRand(output string) string {
	return "#include <stdlib.h>\n#include <time.h>\nstatic int __scar_rand_seeded = 0;\nstatic inline int __scar_rand(int x, int y) { if (!__scar_rand_seeded)" +
		" { srand(time(NULL)); __scar_rand_seeded = 1; } return (rand() % ((y) - (x) + 1)) + (x); }\n#define rand__internal(x, y) __scar_rand((x), (y))\n" + output
}

func replaceRandCalls(output string) string {
	randRegex := regexp.MustCompile(`\brand\s*\(([^,)]+),\s*([^)]+)\)`)
	return randRegex.ReplaceAllString(output, "rand__internal($1, $2)")
}
