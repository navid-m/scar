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
	if strings.Contains(output, "cat") {
		outp = insertCat(outp)
		outp = strings.ReplaceAll(outp, "cat!(", "cat(")
	}
	return outp
}

func insertCat(output string) string {
	return `#define cat(x, y) \
    ({ \
        static char __cat_buf[256]; \
        strcpy(__cat_buf, (x)); \
        strcat(__cat_buf, (y)); \
        __cat_buf; \
    })` + "\n" + output
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
