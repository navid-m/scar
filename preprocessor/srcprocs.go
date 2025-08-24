// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Prior to C conversion (therefore: raw input) macros.
// Contains functions to process source-level macros in Scar code.

package preprocessor

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"scar/lexer"
	"strings"
)

func ProcessSourceLevelMacros(source string) string {
	source = lexer.RemoveComments(source)
	source = ProcessUnsafeAliases(source)
	source = ProcessMacros(source)
	source = ProcessAppendExpressions(source)
	source = ProcessDeleteExpressions(source)
	source = lexer.ReplaceDoubleColonsOutsideStrings(source)
	return source
}

func ProcessUnsafeAliases(source string) string {
	lines := strings.Split(source, "\n")
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "unsafe alias ") {
			aliasPart := strings.TrimSpace(trimmed[13:])
			parts := strings.SplitN(aliasPart, "=", 2)
			if len(parts) == 2 {
				aliasName := strings.TrimSpace(parts[0])
				target := strings.TrimSpace(parts[1])
				if strings.Contains(target, "::") {
					lexer.UnsafeAliases[aliasName] = target
					continue
				}
			}
		}
		result = append(result, line)
	}

	output := strings.Join(result, "\n")
	for aliasName, target := range lexer.UnsafeAliases {
		pattern := aliasName + "("
		replacement := target + "("
		output = strings.ReplaceAll(output, pattern, replacement)
	}

	return output
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

type Macro struct {
	Name       string
	Parameters []string
	Body       []string
}

func ProcessMacros(source string) string {
	macros := collectMacroDefinitions(source)
	importedMacros := loadImportedMacros(source)
	maps.Copy(macros, importedMacros)

	const maxIters = 10
	prev := source
	for i := 0; i < maxIters; i++ {
		next := expandMacros(prev, macros)
		if next == prev {
			return next
		}
		prev = next
	}
	return prev
}

func collectMacroDefinitions(source string) map[string]*Macro {
	macros := make(map[string]*Macro)
	lines := strings.Split(source, "\n")

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		if (strings.HasPrefix(line, "macro ") || strings.HasPrefix(line, "pub macro ")) && strings.HasSuffix(line, ":") {
			macro, endLine := parseMacroDefinition(lines, i)
			if macro != nil {
				macros[macro.Name] = macro
				i = endLine - 1
			}
		}
	}

	return macros
}

func parseMacroDefinition(lines []string, startLine int) (*Macro, int) {
	line := strings.TrimSpace(lines[startLine])
	signature := strings.TrimSpace(line[:len(line)-1])
	parenStart := strings.Index(signature, "(")
	parenEnd := strings.LastIndex(signature, ")")

	if parenStart == -1 || parenEnd == -1 {
		return nil, startLine + 1
	}

	var macroName string
	if strings.HasPrefix(signature, "pub macro ") {
		macroName = strings.TrimSpace(signature[10:parenStart])
	} else {
		macroName = strings.TrimSpace(signature[6:parenStart])
	}
	paramsStr := strings.TrimSpace(signature[parenStart+1 : parenEnd])

	var parameters []string
	if paramsStr != "" {
		paramList := strings.SplitSeq(paramsStr, ",")
		for param := range paramList {
			param = strings.TrimSpace(param)
			if param != "" {
				parameters = append(parameters, param)
			}
		}
	}

	var (
		body           []string
		currentLine    = startLine + 1
		macroIndent    = -1
		macroIndentStr string
	)

	for currentLine < len(lines) {
		bodyLine := lines[currentLine]
		trimmed := strings.TrimSpace(bodyLine)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			currentLine++
			continue
		}

		indent := getIndentation(bodyLine)

		if macroIndent == -1 {
			macroIndent = indent
			macroIndentStr = getLineIndentation(bodyLine)
		}

		if indent < macroIndent {
			break
		}

		stripped := bodyLine
		if strings.HasPrefix(stripped, macroIndentStr) {
			stripped = stripped[len(macroIndentStr):]
		} else {
			i := 0
			width := 0
			for i < len(stripped) && width < macroIndent {
				if stripped[i] == ' ' {
					width += 1
					i++
				} else if stripped[i] == '\t' {
					width += 4
					i++
				} else {
					break
				}
			}
			stripped = stripped[i:]
		}
		stripped = strings.TrimRight(stripped, " \t")
		body = append(body, stripped)
		currentLine++
	}

	return &Macro{
		Name:       macroName,
		Parameters: parameters,
		Body:       body,
	}, currentLine
}

func expandMacros(source string, macros map[string]*Macro) string {
	lines := strings.Split(source, "\n")
	var result []string

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if (strings.HasPrefix(trimmed, "macro ") || strings.HasPrefix(trimmed, "pub macro ")) && strings.HasSuffix(trimmed, ":") {
			currentLine := i + 1
			macroIndent := -1

			for currentLine < len(lines) {
				bodyLine := lines[currentLine]
				bodyTrimmed := strings.TrimSpace(bodyLine)

				if bodyTrimmed == "" || strings.HasPrefix(bodyTrimmed, "#") {
					currentLine++
					continue
				}

				indent := getIndentation(bodyLine)
				if macroIndent == -1 {
					macroIndent = indent
				}

				if indent < macroIndent {
					break
				}

				currentLine++
			}

			i = currentLine - 1
			continue
		}

		expandedLine := expandMacroCallsInLine(line, macros)
		if expandedLine != line {
			expandedLines := strings.Split(expandedLine, "\n")
			for _, expLine := range expandedLines {
				if strings.TrimSpace(expLine) != "" {
					result = append(result, expLine)
				}
			}
		} else {
			result = append(result, line)
		}
	}
	fmt.Println("AFTER MACRO PROCESSING: ", strings.Join(result, "\n"))
	return strings.Join(result, "\n")
}

func loadImportedMacros(source string) map[string]*Macro {
	macros := make(map[string]*Macro)
	lines := strings.SplitSeq(source, "\n")

	for line := range lines {
		trimmed := strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(trimmed, "import "); ok {
			moduleName := strings.Trim(after, "\"")
			moduleMacros := loadMacrosFromModule(moduleName)
			for name, macro := range moduleMacros {
				macros[name] = macro
			}
		}
	}

	return macros
}

func loadMacrosFromModule(moduleName string) map[string]*Macro {
	macros := make(map[string]*Macro)

	moduleSource := findAndReadModule(moduleName)
	if moduleSource == "" {
		return macros
	}

	lines := strings.Split(moduleSource, "\n")

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		if strings.HasPrefix(line, "pub macro ") && strings.HasSuffix(line, ":") {
			macro, endLine := parseMacroDefinition(lines, i)
			if macro != nil {
				qualifiedName := getModuleShortName(moduleName) + "_" + macro.Name
				macros[qualifiedName] = macro
				i = endLine - 1
			}
		}
	}

	return macros
}

func findAndReadModule(moduleName string) string {
	var possiblePaths []string

	resolved := resolveImportPath(moduleName, "")
	possiblePaths = append(possiblePaths, resolved)

	possiblePaths = append(possiblePaths,
		moduleName+".scar",                                  // Direct path
		filepath.Join(".", moduleName+".scar"),              // Current dir
		filepath.Join("tests", moduleName+".scar"),          // In tests dir root
		filepath.Join("tests", "prims", moduleName+".scar"), // In tests/prims
		filepath.Clean(moduleName+".scar"),
	)

	for _, p := range possiblePaths {
		if data, err := os.ReadFile(p); err == nil {
			return string(data)
		}
	}

	return ""
}

func getModuleShortName(moduleName string) string {
	parts := strings.Split(moduleName, "/")
	return parts[len(parts)-1]
}

func expandMacroCallsInLine(line string, macros map[string]*Macro) string {
	result := line

	const maxLineIters = 5
	for iter := 0; iter < maxLineIters; iter++ {
		changed := false
		result = strings.ReplaceAll(result, "::", "_")

		for macroName, macro := range macros {
			pattern := macroName + "("

			for strings.Contains(result, pattern) {
				start := strings.Index(result, pattern)
				if start == -1 {
					break
				}
				parenCount := 0
				end := start + len(pattern) - 1

				for end < len(result) {
					if result[end] == '(' {
						parenCount++
					} else if result[end] == ')' {
						parenCount--
						if parenCount == 0 {
							break
						}
					}
					end++
				}

				if parenCount != 0 {
					break
				}

				argsStr := result[start+len(pattern) : end]
				args := parseArguments(argsStr)

				expanded := expandMacro(macro, args)

				before := result[:start]
				after := result[end+1:]

				indent := getLineIndentation(result[:start])
				indentedExpanded := addIndentationToExpansion(expanded, indent)

				newResult := before + indentedExpanded + after
				if newResult != result {
					result = newResult
					changed = true
				} else {
					break
				}
			}
		}

		if !changed {
			break
		}
	}

	result = strings.ReplaceAll(result, "::", "_")
	return result
}

func parseArguments(argsStr string) []string {
	if strings.TrimSpace(argsStr) == "" {
		return []string{}
	}

	var args []string
	var current strings.Builder
	parenCount := 0
	inString := false

	for i, char := range argsStr {
		switch char {
		case '"':
			if i == 0 || argsStr[i-1] != '\\' {
				inString = !inString
			}
			current.WriteRune(char)
		case '(':
			if !inString {
				parenCount++
			}
			current.WriteRune(char)
		case ')':
			if !inString {
				parenCount--
			}
			current.WriteRune(char)
		case ',':
			if !inString && parenCount == 0 {
				args = append(args, strings.TrimSpace(current.String()))
				current.Reset()
			} else {
				current.WriteRune(char)
			}
		default:
			current.WriteRune(char)
		}
	}

	if current.Len() > 0 {
		args = append(args, strings.TrimSpace(current.String()))
	}

	return args
}

func expandMacro(macro *Macro, args []string) string {
	if len(args) != len(macro.Parameters) {
		return ""
	}

	var expandedLines []string

	for _, bodyLine := range macro.Body {
		expanded := bodyLine

		for i, param := range macro.Parameters {
			arg := args[i]
			expanded = replaceParameter(expanded, param, arg)
		}

		expandedLines = append(expandedLines, expanded)
	}

	return strings.Join(expandedLines, "\n")
}

func replaceParameter(text, param, arg string) string {
	pattern := `\b` + regexp.QuoteMeta(param) + `\b`
	re := regexp.MustCompile(pattern)
	return re.ReplaceAllString(text, arg)
}

func getIndentation(line string) int {
	indent := 0
	for _, char := range line {
		if char == ' ' {
			indent++
		} else if char == '\t' {
			indent += 4
		} else {
			break
		}
	}
	return indent
}

func getLineIndentation(text string) string {
	lastNL := strings.LastIndex(text, "\n")
	start := lastNL + 1
	var indent strings.Builder
	for i := start; i < len(text); i++ {
		ch := text[i]
		if ch == ' ' || ch == '\t' {
			indent.WriteByte(ch)
		} else {
			break
		}
	}
	return indent.String()
}

func addIndentationToExpansion(expanded, indent string) string {
	lines := strings.Split(expanded, "\n")
	var result []string

	for _, line := range lines {
		result = append(result, indent+line)
	}

	return strings.Join(result, "\n")
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

func ContainsExternalNet(source string) bool {
	return ContainsExternalNetWithPath(source, "")
}

func ContainsExternalNetWithPath(source string, basePath string) bool {
	visited := make(map[string]bool)
	return containsExternalNetRecursive(source, basePath, visited)
}

func containsDirectNetImport(source string) bool {
	target := `external import "winsock2.h"`
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

func containsExternalNetRecursive(source string, basePath string, visited map[string]bool) bool {
	if containsDirectNetImport(source) {
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

		if containsExternalNetRecursive(string(importedSource), filepath.Dir(filePath), visited) {
			return true
		}
	}
	return false
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
	var (
		target   = `external import "jansson.h"`
		inString = false
		escaped  = false
	)

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
