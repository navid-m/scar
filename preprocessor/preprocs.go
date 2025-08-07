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
	}
	if strings.Contains(output, "this.") {
		outp = replaceOutsideStringLiterals(outp, "this.", "this->")
	}
	if strings.Contains(output, " and ") {
		outp = replaceOutsideStringLiterals(outp, " and ", " && ")
	}
	if strings.Contains(output, " or ") {
		outp = replaceOutsideStringLiterals(outp, " or ", " || ")
	}
	if strings.Contains(output, "fmt!") {
		outp = strings.ReplaceAll(outp, "fmt!", "fmt")
		outp = insertSprintf(outp)
	}
	if strings.Contains(output, "i32") || strings.Contains(output, "u32") || strings.Contains(output, "i64") ||
		strings.Contains(output, "u64") || strings.Contains(output, "i16") || strings.Contains(output, "u16") ||
		strings.Contains(output, "u8") || strings.Contains(output, "i8") || strings.Contains(output, "f64") ||
		strings.Contains(output, "f32") {
		outp = "#include <stdint.h>\ntypedef int32_t i32;\ntypedef uint32_t u32;\ntypedef int64_t i64;\n" +
			"typedef uint64_t u64;\ntypedef int16_t i16;\ntypedef uint16_t u16;\ntypedef uint8_t u8;\ntypedef int8_t i8;\n" +
			"typedef double f64;\ntypedef float f32;\n" + outp
	}
	return outp
}

func insertSprintf(output string) string {
	return "#define fmt(...) ({ char* __buf = NULL; int __n = snprintf(NULL, 0, __VA_ARGS__); if (__n >= 0) { __buf = malloc(__n + 1);" +
		"if (__buf) snprintf(__buf, __n + 1, __VA_ARGS__); } __buf; })\n" + output
}

func insertCat(output string) string {
	return `#define cat(x, y) \
    ({ \
        static char __cat_buf[256]; \
        strcpy(__cat_buf, (x)); \
        strcat(__cat_buf, (y)); \
        __cat_buf; \
    })` + "\n" + strings.ReplaceAll(output, "cat!(", "cat(")
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

func replaceOutsideStringLiterals(code, target, replacement string) string {
	var (
		result    strings.Builder
		inString  = false
		escaped   = false
		targetLen = len(target)
		i         = 0
	)
	for i < len(code) {
		ch := code[i]
		if inString {
			if ch == '\\' && !escaped {
				escaped = true
				result.WriteByte(ch)
				i++
				continue
			}
			if ch == '"' && !escaped {
				inString = false
			}
			escaped = false
			result.WriteByte(ch)
			i++
		} else {
			if ch == '"' {
				inString = true
				result.WriteByte(ch)
				i++
			} else if i+targetLen <= len(code) && code[i:i+targetLen] == target {
				result.WriteString(replacement)
				i += targetLen
			} else {
				result.WriteByte(ch)
				i++
			}
		}
	}
	return result.String()
}
