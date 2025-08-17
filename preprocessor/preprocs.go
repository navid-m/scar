// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Post C conversion macros.
// This file contains functions to insert macros and process expressions in C code.

package preprocessor

import (
	"regexp"
	"scar/lexer"
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

	outp = fixCustomClassReturnTypes(outp)
	outp = fixMethodCalls(outp)
	outp = fixPropertyAccess(outp)

	// Don't convert this. to this-> for struct constructors since structs use value semantics
	// The conversion will be handled by the renderer based on context

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
	if strings.Contains(output, "lstring") {
		outp = insertLstring(outp)
	}
	if strings.Contains(output, "cstring") {
		outp = insertCstring(outp)
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

func fixPropertyAccess(outp string) string {
	lines := strings.Split(outp, "\n")
	objectVars := make(map[string]bool)
	declRe := regexp.MustCompile(`\b(\w+)\*\s+(\w+)\s*(?:[=;,,\)])`)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || isInsideStringLiteral(trimmed) || strings.HasPrefix(trimmed, "#include") {
			continue
		}
		matches := declRe.FindAllStringSubmatch(trimmed, -1)
		for _, m := range matches {
			if len(m) > 2 {
				objectVars[m[2]] = true
			}
		}
	}

	if len(objectVars) == 0 {
		return outp
	}

	var b strings.Builder
	inString := false
	escaped := false
	i := 0
	for i < len(outp) {
		ch := outp[i]
		if inString {
			b.WriteByte(ch)
			if ch == '\\' && !escaped {
				escaped = true
			} else {
				if ch == '"' && !escaped {
					inString = false
				}
				escaped = false
			}
			i++
			continue
		}

		if ch == '"' {
			inString = true
			b.WriteByte(ch)
			i++
			continue
		}
		if i+5 <= len(outp) && outp[i:i+5] == "fmt!(" {
			b.WriteString("fmt!(")
			i += 5
			startArgs := i
			paren := 1
			innerInString := false
			innerEsc := false
			for i < len(outp) && paren > 0 {
				c := outp[i]
				if innerInString {
					if c == '\\' && !innerEsc {
						innerEsc = true
					} else {
						if c == '"' && !innerEsc {
							innerInString = false
						}
						innerEsc = false
					}
				} else {
					if c == '"' {
						innerInString = true
					} else if c == '(' {
						paren++
					} else if c == ')' {
						paren--
						if paren == 0 {
							break
						}
					}
				}
				i++
			}
			args := outp[startArgs:i]
			for obj := range objectVars {
				args = replaceOutsideStringLiterals(args, obj+".", obj+"->")
			}
			b.WriteString(args)
			if i < len(outp) && outp[i] == ')' {
				b.WriteByte(')')
				i++
			}
			continue
		}

		b.WriteByte(ch)
		i++
	}

	return b.String()
}

func insertCstring(output string) string {
	return "typedef char* cstring;\n" + output
}

func insertLstring(output string) string {
	return "typedef char* lstring;\n" + output
}

func insertSprintf(output string) string {
	return "#define fmt(...) ({ int __len = snprintf(NULL, 0, __VA_ARGS__) + 1; char* __buf = malloc(__len); if(__buf) snprintf(__buf, __len, __VA_ARGS__); __buf; })\n" + output
}

func insertCat(output string) string {
	return `#define cat(x, y) \
    ({ \
        static char __cat_buf[256]; \
        const char* __x_val = (x); \
        const char* __y_val = (y); \
        strcpy(__cat_buf, __x_val); \
        strcat(__cat_buf, __y_val); \
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

func fixCustomClassReturnTypes(output string) string {
	lines := strings.Split(output, "\n")
	customClasses := make(map[string]bool)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "_new(") {
			re := regexp.MustCompile(`^(\w+)\*\s+(\w+)_new\(`)
			if matches := re.FindStringSubmatch(trimmed); len(matches) > 2 {
				className := matches[1]
				if className == matches[2] {
					customClasses[className] = true
				}
			}
		}
	}
	for i, line := range lines {
		re := regexp.MustCompile(`^(\s*)(\w+)\s+(\w+)\s*=\s*(\w+\([^)]*\));(.*)$`)
		if matches := re.FindStringSubmatch(line); len(matches) > 5 {
			indent := matches[1]
			className := matches[2]
			varName := matches[3]
			functionCall := matches[4]
			rest := matches[5]
			if customClasses[className] {
				lines[i] = indent + className + "* " + varName + " = " + functionCall + ";" + rest
			}
		}
	}
	return strings.Join(lines, "\n")
}

func fixMethodCalls(output string) string {
	lines := strings.Split(output, "\n")
	objectTypes := make(map[string]string)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		re := regexp.MustCompile(`(\w+)\*\s+(\w+)\s*=`)
		if matches := re.FindStringSubmatch(trimmed); len(matches) > 2 {
			className := matches[1]
			objectName := matches[2]
			objectTypes[objectName] = className
		}
	}

	for i, line := range lines {
		if !isInsideStringLiteral(line) {
			re := regexp.MustCompile(`(\w+)\.(\w+)\(`)
			matches := re.FindAllStringSubmatch(line, -1)

			for _, match := range matches {
				if len(match) > 2 {
					objectName := match[1]
					methodName := match[2]
					if className, exists := objectTypes[objectName]; exists {
						oldPattern := objectName + "." + methodName + "("
						newPattern := className + "_" + methodName + "(" + objectName
						if strings.Contains(line, oldPattern+")") {
							newPattern += ")"
							line = strings.ReplaceAll(line, oldPattern+")", newPattern)
						} else {
							newPattern += ", "
							line = strings.ReplaceAll(line, oldPattern, newPattern)
						}
					}
				}
			}
			lines[i] = line
		}
	}

	return strings.Join(lines, "\n")
}

func isInsideStringLiteral(line string) bool {
	inString := false
	for i, char := range line {
		if char == '"' && (i == 0 || line[i-1] != '\\') {
			inString = !inString
		}
	}
	return inString
}

func RemoveMacroFunctionCalls(cCode string) string {
	lines := strings.Split(cCode, "\n")
	var cleanLines []string
	for _, line := range lines {
		cleaned := line
		for macroName := range lexer.RegisteredMacros {
			pattern := regexp.MustCompile(`\s*` + regexp.QuoteMeta(macroName) + `\s*\([^)]*\)\s*;?\s*`)
			cleaned = pattern.ReplaceAllString(cleaned, "")
		}
		if strings.TrimSpace(cleaned) != "" || strings.TrimSpace(line) == strings.TrimSpace(cleaned) {
			cleanLines = append(cleanLines, cleaned)
		}
	}
	return strings.Join(cleanLines, "\n")
}
