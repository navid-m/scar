// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Prior to C conversion (therefore: raw input) macros.
// Contains functions to process source-level macros in Scar code.

package preprocessor

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"scar/lexer"
	"strings"
)

func ProcessSourceLevelMacros(source string) string {
	source = lexer.RemoveComments(source)
	source = ProcessAppendExpressions(source)
	source = ProcessDeleteExpressions(source)
	source = lexer.ReplaceDoubleColonsOutsideStrings(source)
	return source
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

func ContainsExternalJson(source string) bool {
	return ContainsExternalJsonWithPath(source, "")
}

func ContainsExternalJsonWithPath(source string, basePath string) bool {
	visited := make(map[string]bool)
	return containsExternalJsonRecursive(source, basePath, visited)
}

func containsExternalJsonRecursive(source string, basePath string, visited map[string]bool) bool {
	if containsDirectJsonImport(source) {
		return true
	}

	imports := extractImports(source)
	for _, importPath := range imports {
		filePath := resolveImportPath(importPath, basePath)
		if filePath == "" {
			continue
		}

		if visited[filePath] {
			continue
		}
		visited[filePath] = true
		importedSource, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		if containsExternalJsonRecursive(string(importedSource), filepath.Dir(filePath), visited) {
			return true
		}
	}

	return false
}

func containsDirectJsonImport(source string) bool {
	target := `external import "jansson.h"`
	inString := false
	escaped := false

	for i := 0; i < len(source); i++ {
		char := source[i]
		if escaped {
			escaped = false
			continue
		}

		if char == '\\' {
			escaped = true
			continue
		}
		if char == '"' {
			inString = !inString
			continue
		}
		if !inString {
			if i+len(target) <= len(source) {
				if source[i:i+len(target)] == target {
					return true
				}
			}
		}
	}
	return false
}

func ContainsExternalCurl(source string) bool {
	return ContainsExternalCurlWithPath(source, "")
}

func ContainsExternalCurlWithPath(source string, basePath string) bool {
	visited := make(map[string]bool)
	return containsExternalCurlRecursive(source, basePath, visited)
}

func ContainsExternalRegex(source string) bool {
	return ContainsExternalRegexWithPath(source, "")
}

func ContainsExternalRegexWithPath(source string, basePath string) bool {
	visited := make(map[string]bool)
	return containsExternalRegexRecursive(source, basePath, visited)
}

func containsExternalRegexRecursive(source string, basePath string, visited map[string]bool) bool {
	if containsDirectRegexImport(source) {
		return true
	}

	imports := extractImports(source)
	for _, importPath := range imports {
		filePath := resolveImportPath(importPath, basePath)
		if filePath == "" {
			continue
		}

		if visited[filePath] {
			continue
		}
		visited[filePath] = true
		importedSource, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		if containsExternalRegexRecursive(string(importedSource), filepath.Dir(filePath), visited) {
			return true
		}
	}

	return false
}

func containsDirectRegexImport(source string) bool {
	target := `external import "pcre.h"`
	inString := false
	escaped := false

	for i := 0; i < len(source); i++ {
		char := source[i]
		if escaped {
			escaped = false
			continue
		}

		if char == '\\' {
			escaped = true
			continue
		}
		if char == '"' {
			inString = !inString
			continue
		}
		if !inString {
			if i+len(target) <= len(source) {
				if source[i:i+len(target)] == target {
					return true
				}
			}
		}
	}
	return false
}

func containsExternalCurlRecursive(source string, basePath string, visited map[string]bool) bool {
	if containsDirectCurlImport(source) {
		return true
	}

	imports := extractImports(source)
	for _, importPath := range imports {
		filePath := resolveImportPath(importPath, basePath)
		if filePath == "" {
			continue
		}

		if visited[filePath] {
			continue
		}
		visited[filePath] = true
		importedSource, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		if containsExternalCurlRecursive(string(importedSource), filepath.Dir(filePath), visited) {
			return true
		}
	}

	return false
}

func containsDirectCurlImport(source string) bool {
	target := `external import "curl/curl.h"`
	inString := false
	escaped := false

	for i := 0; i < len(source); i++ {
		char := source[i]
		if escaped {
			escaped = false
			continue
		}

		if char == '\\' {
			escaped = true
			continue
		}
		if char == '"' {
			inString = !inString
			continue
		}
		if !inString {
			if i+len(target) <= len(source) {
				if source[i:i+len(target)] == target {
					return true
				}
			}
		}
	}
	return false
}

func extractImports(source string) []string {
	importRegex := regexp.MustCompile(`import\s+"([^"]+)"`)
	matches := importRegex.FindAllStringSubmatch(source, -1)
	var imports []string
	for _, match := range matches {
		if len(match) > 1 {
			imports = append(imports, match[1])
		}
	}
	return imports
}

func resolveImportPath(importPath string, basePath string) string {
	if after, ok := strings.CutPrefix(importPath, "std/"); ok {
		stdPath := after
		if basePath != "" {
			projectRoot := findProjectRoot(basePath)
			if projectRoot != "" {
				return filepath.Join(projectRoot, "lib", stdPath+".scar")
			}
		}
		return filepath.Join("lib", stdPath+".scar")
	}

	if basePath != "" {
		return filepath.Join(basePath, importPath+".scar")
	}

	return importPath + ".scar"
}

func findProjectRoot(startPath string) string {
	currentPath := startPath
	for {
		libPath := filepath.Join(currentPath, "lib")
		if info, err := os.Stat(libPath); err == nil && info.IsDir() {
			return currentPath
		}
		parentPath := filepath.Dir(currentPath)
		if parentPath == currentPath {
			break
		}
		currentPath = parentPath
	}
	return ""
}
