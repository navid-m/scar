// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Prior to C conversion (therefore: raw input) macros.
// Contains functions to process source-level macros in Scar code.

package preprocessor

import (
	"bytes"
	"strings"
)

func ProcessSourceLevelMacros(source string) string {
	source = RemoveComments(source)
	source = ProcessAppendExpressions(source)
	source = ProcessDeleteExpressions(source)
	return source
}

func RemoveComments(source string) string {
	var result strings.Builder
	inString := false
	inComment := false
	lineStart := 0

	for i := 0; i < len(source); i++ {
		if source[i] == '"' && (i == 0 || source[i-1] != '\\') {
			inString = !inString
		}
		if source[i] == '\n' {
			lineStart = i + 1
			inComment = false
		}
		if !inString && source[i] == '#' {
			isFullLineComment := true
			for j := lineStart; j < i; j++ {
				if source[j] != ' ' && source[j] != '\t' && source[j] != '\r' {
					isFullLineComment = false
					break
				}
			}

			if isFullLineComment {
				for i < len(source) && source[i] != '\n' {
					i++
				}
				if i < len(source) {
					result.WriteByte('\n')
				}
				lineStart = i + 1
				continue
			} else if !inComment {
				inComment = true
				if i > 0 && source[i-1] != '\n' {
					result.WriteByte('\n')
				}
				for i < len(source) && source[i] != '\n' {
					i++
				}
				if i < len(source) {
					result.WriteByte('\n')
				}
				lineStart = i + 1
				continue
			}
		}
		if !inComment {
			result.WriteByte(source[i])
		}
	}

	return result.String()
}



func ProcessAppendExpressions(source string) string {
	var result bytes.Buffer
	inString := false
	i := 0

	for i < len(source) {
		if source[i] == '"' {
			if i == 0 || source[i-1] != '\\' {
				inString = !inString
			}
		}

		if !inString && i+8 <= len(source) && source[i:i+8] == "append!(" {
			j := i + 8
			parenCount := 1
			argStart := j
			endParen := -1

			for j < len(source) {
				if source[j] == '(' {
					parenCount++
				} else if source[j] == ')' {
					parenCount--
					if parenCount == 0 {
						endParen = j
						break
					}
				}
				j++
			}

			if endParen != -1 {
				argStr := source[argStart:endParen]
				pCount := 0
				comma := -1

				for k, char := range argStr {
					if char == '(' {
						pCount++
					} else if char == ')' {
						pCount--
					} else if char == ',' && pCount == 0 {
						comma = k
						break
					}
				}

				if comma != -1 {
					mapName := strings.TrimSpace(argStr[:comma])
					value := strings.TrimSpace(argStr[comma+1:])
					result.WriteString("(" + mapName + "_append_helper(" + mapName + "_keys, " + mapName + "_values, &" + mapName + "_size, " + value + "), " + mapName + ")")
					i = endParen + 1
					continue
				}
			}
		}

		result.WriteByte(source[i])
		i++
	}

	return result.String()
}

// Replaces delete!(mapName, key) with mapName after removing the key
func ProcessDeleteExpressions(source string) string {
	var result bytes.Buffer
	inString := false
	i := 0

	for i < len(source) {
		if source[i] == '"' {
			if i == 0 || source[i-1] != '\\' {
				inString = !inString
			}
		}

		if !inString && i+8 <= len(source) && source[i:i+8] == "delete!(" {
			j := i + 8
			parenCount := 1
			argStart := j
			endParen := -1

			for j < len(source) {
				if source[j] == '(' {
					parenCount++
				} else if source[j] == ')' {
					parenCount--
					if parenCount == 0 {
						endParen = j
						break
					}
				}
				j++
			}

			if endParen != -1 {
				argStr := source[argStart:endParen]
				pCount := 0
				comma := -1

				for k, char := range argStr {
					if char == '(' {
						pCount++
					} else if char == ')' {
						pCount--
					} else if char == ',' && pCount == 0 {
						comma = k
						break
					}
				}

				if comma != -1 {
					mapName := strings.TrimSpace(argStr[:comma])
					key := strings.TrimSpace(argStr[comma+1:])
					result.WriteString("(" + mapName + "_delete_helper(" + mapName + "_keys, " + mapName + "_values, &" + mapName + "_size, " + key + "), " + mapName + ")")
					i = endParen + 1
					continue
				}
			}
		}

		result.WriteByte(source[i])
		i++
	}

	return result.String()
}
