// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Contains the parser cases for the parser/lexer operations.
// Includes compile-time module processing and parsing.

package lexer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RemoveComments removes both full-line and inline comments from source code
// but preserves comments inside $raw blocks (for C preprocessor directives)
func RemoveComments(source string) string {
	var result strings.Builder
	inString := false
	inRawBlock := false
	rawParenDepth := 0
	lineStart := 0

	for i := 0; i < len(source); i++ {
		if source[i] == '"' && (i == 0 || source[i-1] != '\\') {
			inString = !inString
		}

		// Check for $raw block start
		if !inString && i+6 < len(source) && source[i:i+6] == "$raw (" {
			inRawBlock = true
			rawParenDepth = 1
			// Skip ahead past "$raw ("
			for j := 0; j < 6; j++ {
				result.WriteByte(source[i+j])
			}
			i += 5 // Will be incremented by 1 in the loop
			continue
		}

		// Track parentheses depth inside $raw blocks
		if inRawBlock && !inString {
			if source[i] == '(' {
				rawParenDepth++
			} else if source[i] == ')' {
				rawParenDepth--
				if rawParenDepth == 0 {
					inRawBlock = false
				}
			}
		}

		if source[i] == '\n' {
			lineStart = i + 1
		}

		// Only remove comments if we're not in a raw block
		if !inString && !inRawBlock && source[i] == '#' {
			// Check if this is a full-line comment (only whitespace before #)
			isFullLineComment := true
			for j := lineStart; j < i; j++ {
				if source[j] != ' ' && source[j] != '\t' && source[j] != '\r' {
					isFullLineComment = false
					break
				}
			}

			if isFullLineComment {
				// Skip the entire line for full-line comments
				for i < len(source) && source[i] != '\n' {
					i++
				}
				if i < len(source) {
					result.WriteByte('\n') // Keep the newline
				}
				lineStart = i + 1
				continue
			} else {
				// For inline comments, skip from # to end of line
				for i < len(source) && source[i] != '\n' {
					i++
				}
				// Don't skip the newline character, let it be processed normally
				if i < len(source) {
					i-- // Back up one so the newline gets processed in the next iteration
				}
				continue
			}
		}

		result.WriteByte(source[i])
	}

	return result.String()
}

func parseAllImports(lines []string, startLine int) ([]*ImportStmt, error) {
	var imports []*ImportStmt
	line := strings.TrimSpace(lines[startLine])

	if strings.Contains(line, ",") {
		importLine := strings.TrimSpace(line[6:])
		moduleNames := strings.SplitSeq(importLine, ",")
		for moduleName := range moduleNames {
			moduleName = strings.TrimSpace(strings.Trim(moduleName, "\""))
			if moduleName != "" {
				imports = append(imports, &ImportStmt{Module: moduleName})
			}
		}
	} else {
		currentLine := startLine + 1
		for currentLine < len(lines) {
			line := lines[currentLine]
			trimmed := strings.TrimSpace(line)

			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				currentLine++
				continue
			}

			if getIndentation(line) == 0 {
				break
			}

			moduleNames := strings.SplitSeq(trimmed, ",")
			for moduleName := range moduleNames {
				moduleName = strings.TrimSpace(strings.Trim(moduleName, "\""))
				if moduleName != "" {
					imports = append(imports, &ImportStmt{Module: moduleName})
				}
			}

			currentLine++
		}
	}

	return imports, nil
}

func parseTryCatchStatement(lines []string, lineNum, currentIndent int) (*Statement, int, error) {
	line := strings.TrimSpace(lines[lineNum])
	if line != "try:" {
		return nil, lineNum + 1, fmt.Errorf("try statement format error at line %d", lineNum+1)
	}

	var expectedBodyIndent = currentIndent + 4
	bodyStartLine := lineNum + 1
	for bodyStartLine < len(lines) {
		bodyLine := lines[bodyStartLine]
		if strings.TrimSpace(bodyLine) != "" && !strings.HasPrefix(strings.TrimSpace(bodyLine), "#") {
			expectedBodyIndent = getIndentation(bodyLine)
			break
		}
		bodyStartLine++
	}
	if expectedBodyIndent <= currentIndent {
		expectedBodyIndent = currentIndent + 4
	}

	tryBody, err := parseStatements(lines, lineNum+1, expectedBodyIndent)
	if err != nil {
		return nil, lineNum + 1, err
	}

	nextLine := findEndOfBlock(lines, lineNum+1, expectedBodyIndent)

	if nextLine >= len(lines) {
		return nil, nextLine, fmt.Errorf("try statement requires a catch block at line %d", lineNum+1)
	}

	nextTrimmed := strings.TrimSpace(lines[nextLine])
	if nextTrimmed != "catch:" {
		return nil, nextLine, fmt.Errorf("expected catch statement after try block at line %d", nextLine+1)
	}

	bodyStartLine = nextLine + 1
	for bodyStartLine < len(lines) {
		bodyLine := lines[bodyStartLine]
		if strings.TrimSpace(bodyLine) != "" && !strings.HasPrefix(strings.TrimSpace(bodyLine), "#") {
			expectedBodyIndent = getIndentation(bodyLine)
			break
		}
		bodyStartLine++
	}
	if expectedBodyIndent <= currentIndent {
		expectedBodyIndent = currentIndent + 4
	}

	catchBody, err := parseStatements(lines, nextLine+1, expectedBodyIndent)
	if err != nil {
		return nil, nextLine + 1, err
	}

	nextLine = findEndOfBlock(lines, nextLine+1, expectedBodyIndent)

	return &Statement{TryCatch: &TryCatchStmt{TryBody: tryBody, CatchBody: catchBody}}, nextLine, nil
}

func parsePubStatement(lines []string, lineNum, currentIndent int) (*Statement, int, error) {
	line := strings.TrimSpace(lines[lineNum])
	parts := strings.Fields(line)

	if len(parts) < 2 {
		return nil, lineNum + 1, fmt.Errorf("pub statement requires a type at line %d", lineNum+1)
	}

	switch parts[1] {
	case "class":
		return parsePubClassStatement(lines, lineNum, currentIndent)
	case "fn":
		return parsePubFunctionStatement(lines, lineNum, currentIndent)
	default:
		if len(parts) >= 5 && parts[3] == "=" && isValidType(parts[1]) {
			varType := parts[1]
			varName := parts[2]
			value := strings.Join(parts[4:], " ")

			if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
				value = value[1 : len(value)-1]
			}

			return &Statement{PubVarDecl: &PubVarDeclStmt{Type: varType, Name: varName, Value: value}}, lineNum + 1, nil
		}
		return nil, lineNum + 1, fmt.Errorf("invalid pub declaration at line %d", lineNum+1)
	}
}

func parsePubClassStatement(lines []string, lineNum, currentIndent int) (*Statement, int, error) {
	line := strings.TrimSpace(lines[lineNum])
	parts := strings.Fields(line)

	if len(parts) < 3 || !strings.HasSuffix(line, ":") {
		return nil, lineNum + 1, fmt.Errorf("pub class declaration format error at line %d", lineNum+1)
	}
	var (
		className          = strings.TrimSuffix(parts[2], ":")
		expectedBodyIndent = currentIndent + 4
	)
	if currentIndent == 0 {
		bodyStartLine := lineNum + 1
		for bodyStartLine < len(lines) {
			bodyLine := lines[bodyStartLine]
			if strings.TrimSpace(bodyLine) != "" && !strings.HasPrefix(strings.TrimSpace(bodyLine), "#") {
				expectedBodyIndent = getIndentation(bodyLine)
				break
			}
			bodyStartLine++
		}
		if expectedBodyIndent <= currentIndent {
			expectedBodyIndent = currentIndent + 4
		}
	}

	var constructor *ConstructorStmt
	var methods []*MethodDeclStmt
	nextLine := lineNum + 1

	for nextLine < len(lines) {
		line := lines[nextLine]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			nextLine++
			continue
		}

		indent := getIndentation(line)
		if indent < expectedBodyIndent {
			break
		}

		if indent != expectedBodyIndent {
			return nil, nextLine + 1, fmt.Errorf("unexpected indentation in class body at line %d", nextLine+1)
		}

		if strings.HasPrefix(trimmed, "init") {
			var (
				parameters     []*MethodParameter
				initBody       []*Statement
				initBodyIndent int
				initStartLine  int
			)
			if strings.Contains(trimmed, "(") && strings.Contains(trimmed, ")") {
				parenStart := strings.Index(trimmed, "(")
				parenEnd := strings.Index(trimmed, ")")
				if parenStart != -1 && parenEnd != -1 && parenEnd > parenStart {
					paramsStr := strings.TrimSpace(trimmed[parenStart+1 : parenEnd])
					if paramsStr != "" {
						paramList := strings.SplitSeq(paramsStr, ",")
						for paramStr := range paramList {
							paramStr = strings.TrimSpace(paramStr)
							paramParts := strings.Fields(paramStr)
							if len(paramParts) == 2 {
								param := &MethodParameter{
									Type: paramParts[0],
									Name: paramParts[1],
								}
								parameters = append(parameters, param)
							} else if len(paramParts) == 1 {
								param := &MethodParameter{
									Type: "int",
									Name: paramParts[0],
								}
								parameters = append(parameters, param)
							}
						}
					}
				}
			}
			initBodyIndent = expectedBodyIndent + 4
			initStartLine = nextLine + 1
			for initStartLine < len(lines) {
				initLine := lines[initStartLine]
				if strings.TrimSpace(initLine) != "" && !strings.HasPrefix(strings.TrimSpace(initLine), "#") {
					initBodyIndent = getIndentation(initLine)
					break
				}
				initStartLine++
			}

			initBody, err := parseStatements(lines, initStartLine, initBodyIndent)
			if err != nil {
				return nil, nextLine + 1, err
			}

			constructor = &ConstructorStmt{Parameters: parameters, Fields: initBody}
			nextLine = findEndOfBlock(lines, initStartLine, initBodyIndent)
		} else if strings.HasPrefix(trimmed, "fn ") {
			method, newNextLine, err := parseMethodStatement(lines, nextLine, expectedBodyIndent)
			if err != nil {
				return nil, nextLine + 1, err
			}
			methods = append(methods, method)
			nextLine = newNextLine
		} else {
			nextLine++
		}
	}

	pubClassStmt := &PubClassDeclStmt{
		Name:        className,
		Constructor: constructor,
		Methods:     methods,
	}

	return &Statement{PubClassDecl: pubClassStmt}, nextLine, nil
}

func parseClassStatement(lines []string, lineNum, currentIndent int) (*Statement, int, error) {
	line := strings.TrimSpace(lines[lineNum])
	parts := strings.Fields(line)

	if len(parts) < 2 || !strings.HasSuffix(line, ":") {
		return nil, lineNum + 1, fmt.Errorf("class declaration format error at line %d", lineNum+1)
	}

	var (
		className          = strings.TrimSuffix(parts[1], ":")
		expectedBodyIndent = currentIndent + 4
	)
	if currentIndent == 0 {
		bodyStartLine := lineNum + 1
		for bodyStartLine < len(lines) {
			bodyLine := lines[bodyStartLine]
			if strings.TrimSpace(bodyLine) != "" && !strings.HasPrefix(strings.TrimSpace(bodyLine), "#") {
				expectedBodyIndent = getIndentation(bodyLine)
				break
			}
			bodyStartLine++
		}
		if expectedBodyIndent <= currentIndent {
			expectedBodyIndent = currentIndent + 4
		}
	}

	var constructor *ConstructorStmt
	var methods []*MethodDeclStmt
	nextLine := lineNum + 1

	for nextLine < len(lines) {
		line := lines[nextLine]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			nextLine++
			continue
		}

		indent := getIndentation(line)
		if indent < expectedBodyIndent {
			break
		}

		if indent != expectedBodyIndent {
			return nil, nextLine + 1, fmt.Errorf("unexpected indentation in class body at line %d", nextLine+1)
		}

		if strings.HasPrefix(trimmed, "init") {
			var parameters []*MethodParameter
			var initBody []*Statement
			var initBodyIndent int
			var initStartLine int

			if strings.Contains(trimmed, "(") && strings.Contains(trimmed, ")") {
				parenStart := strings.Index(trimmed, "(")
				parenEnd := strings.Index(trimmed, ")")
				if parenStart != -1 && parenEnd != -1 && parenEnd > parenStart {
					paramsStr := strings.TrimSpace(trimmed[parenStart+1 : parenEnd])
					if paramsStr != "" {
						paramList := strings.SplitSeq(paramsStr, ",")
						for paramStr := range paramList {
							paramStr = strings.TrimSpace(paramStr)
							paramParts := strings.Fields(paramStr)

							param := &MethodParameter{}

							// Handle ref parameters
							if len(paramParts) >= 3 && paramParts[0] == "ref" {
								param.IsRef = true
								param.Type = paramParts[1]
								param.Name = paramParts[2]
							} else if len(paramParts) == 2 {
								param.Type = paramParts[0]
								param.Name = paramParts[1]
							} else if len(paramParts) == 1 {
								param.Type = "int"
								param.Name = paramParts[0]
							}

							parameters = append(parameters, param)
						}
					}
				}
			}

			initBodyIndent = expectedBodyIndent + 4
			initStartLine = nextLine + 1
			for initStartLine < len(lines) {
				initLine := lines[initStartLine]
				if strings.TrimSpace(initLine) != "" && !strings.HasPrefix(strings.TrimSpace(initLine), "#") {
					initBodyIndent = getIndentation(initLine)
					break
				}
				initStartLine++
			}

			initBody, err := parseStatements(lines, initStartLine, initBodyIndent)
			if err != nil {
				return nil, nextLine + 1, err
			}

			constructor = &ConstructorStmt{Parameters: parameters, Fields: initBody}
			nextLine = findEndOfBlock(lines, initStartLine, initBodyIndent)
		} else if strings.HasPrefix(trimmed, "fn ") {
			method, newNextLine, err := parseMethodStatement(lines, nextLine, expectedBodyIndent)
			if err != nil {
				return nil, nextLine + 1, err
			}
			methods = append(methods, method)
			nextLine = newNextLine
		} else {
			nextLine++
		}
	}

	classStmt := &ClassDeclStmt{
		Name:        className,
		Constructor: constructor,
		Methods:     methods,
	}

	return &Statement{ClassDecl: classStmt}, nextLine, nil
}

func parseTopLevelFunctionStatement(lines []string, lineNum, currentIndent int) (*Statement, int, error) {
	line := strings.TrimSpace(lines[lineNum])
	if !strings.HasPrefix(line, "fn ") || !strings.HasSuffix(line, ":") {
		return nil, lineNum + 1, fmt.Errorf("function declaration format error at line %d", lineNum+1)
	}

	// Extract function name and parameters
	parenStart := strings.Index(line, "(")
	parenEnd := strings.LastIndex(line, ")")
	colonIndex := strings.LastIndex(line, ":")
	if parenStart == -1 || parenEnd == -1 || colonIndex == -1 || parenEnd < parenStart {
		return nil, lineNum + 1, fmt.Errorf("function declaration syntax error at line %d", lineNum+1)
	}

	// Extract return type
	returnTypePart := strings.TrimSpace(line[parenEnd+1 : colonIndex])
	returnType := "void"
	if returnTypePart != "" && returnTypePart != "->" {
		returnType = strings.TrimSpace(strings.Split(returnTypePart, "->")[1])
	}

	// Extract function name
	funcNamePart := strings.TrimSpace(line[3:parenStart])
	funcName := funcNamePart
	if strings.HasPrefix(funcName, "pub ") {
		funcName = strings.TrimSpace(funcName[4:])
	}

	// Parse parameters
	paramsStr := strings.TrimSpace(line[parenStart+1 : parenEnd])
	var parameters []*MethodParameter
	if paramsStr != "" {
		paramList := strings.Split(paramsStr, ",")
		for _, param := range paramList {
			param = strings.TrimSpace(param)
			if param == "" {
				continue
			}
			paramParts := strings.Fields(param)
			if len(paramParts) < 2 {
				return nil, lineNum + 1, fmt.Errorf("invalid parameter format at line %d", lineNum+1)
			}

			paramType := paramParts[0]
			paramName := paramParts[1]
			isList := false
			listType := ""

			if strings.HasPrefix(paramType, "list[") && strings.HasSuffix(paramType, "]") {
				isList = true
				listType = strings.TrimPrefix(strings.TrimSuffix(paramType, "]"), "list[")
				paramType = listType
			}

			if !isValidType(paramType) && !isList {
				return nil, lineNum + 1, fmt.Errorf("invalid parameter type '%s' at line %d", paramType, lineNum+1)
			}

			parameters = append(parameters, &MethodParameter{
				Type:     paramType,
				IsList:   isList,
				ListType: listType,
				Name:     paramName,
			})
		}
	}

	// Parse function body
	expectedBodyIndent := currentIndent + 4
	if currentIndent == 0 {
		bodyStartLine := lineNum + 1
		for bodyStartLine < len(lines) {
			bodyLine := lines[bodyStartLine]
			if strings.TrimSpace(bodyLine) != "" && !strings.HasPrefix(strings.TrimSpace(bodyLine), "#") {
				expectedBodyIndent = getIndentation(bodyLine)
				break
			}
			bodyStartLine++
		}
		if expectedBodyIndent <= currentIndent {
			expectedBodyIndent = currentIndent + 4
		}
	}

	body, err := parseStatements(lines, lineNum+1, expectedBodyIndent)
	if err != nil {
		return nil, lineNum + 1, err
	}

	nextLine := findEndOfBlock(lines, lineNum+1, expectedBodyIndent)

	stmt := &Statement{}
	if strings.HasPrefix(line, "pub fn ") {
		stmt.PubTopLevelFuncDecl = &PubTopLevelFuncDeclStmt{
			Name:       funcName,
			Parameters: parameters,
			ReturnType: returnType,
			Body:       body,
		}
	} else {
		stmt.TopLevelFuncDecl = &TopLevelFuncDeclStmt{
			Name:       funcName,
			Parameters: parameters,
			ReturnType: returnType,
			Body:       body,
		}
	}

	return stmt, nextLine, nil
}

func parseMethodStatement(lines []string, lineNum, currentIndent int) (*MethodDeclStmt, int, error) {
	line := strings.TrimSpace(lines[lineNum])

	if !strings.HasPrefix(line, "fn ") || !strings.HasSuffix(line, ":") {
		return nil, lineNum + 1, fmt.Errorf("invalid method declaration at line %d", lineNum+1)
	}

	signature := strings.TrimSpace(line[3 : len(line)-1])
	parenStart := strings.Index(signature, "(")
	if parenStart == -1 {
		return nil, lineNum + 1, fmt.Errorf("method declaration missing parameters at line %d", lineNum+1)
	}

	methodName := strings.TrimSpace(signature[:parenStart])
	parenEnd := strings.Index(signature, ")")
	if parenEnd == -1 || parenEnd <= parenStart {
		return nil, lineNum + 1, fmt.Errorf("method declaration missing closing parenthesis at line %d", lineNum+1)
	}

	paramsStr := strings.TrimSpace(signature[parenStart+1 : parenEnd])
	var parameters []*MethodParameter
	if paramsStr != "" {
		paramList := strings.SplitSeq(paramsStr, ",")
		for paramStr := range paramList {
			paramStr = strings.TrimSpace(paramStr)
			paramParts := strings.Fields(paramStr)
			if len(paramParts) == 2 {
				param := &MethodParameter{
					Type: paramParts[0],
					Name: paramParts[1],
				}
				parameters = append(parameters, param)
			} else if len(paramParts) == 1 {
				param := &MethodParameter{
					Type: "int",
					Name: paramParts[0],
				}
				parameters = append(parameters, param)
			}
		}
	}

	returnTypePart := strings.TrimSpace(signature[parenEnd+1:])
	var returnType string
	if strings.HasPrefix(returnTypePart, "->") {
		returnType = strings.TrimSpace(returnTypePart[2:])
	} else {
		returnType = "void"
	}

	expectedBodyIndent := currentIndent + 4
	bodyStartLine := lineNum + 1
	for bodyStartLine < len(lines) {
		bodyLine := lines[bodyStartLine]
		if strings.TrimSpace(bodyLine) != "" && !strings.HasPrefix(strings.TrimSpace(bodyLine), "#") {
			expectedBodyIndent = getIndentation(bodyLine)
			break
		}
		bodyStartLine++
	}

	body, err := parseStatements(lines, bodyStartLine, expectedBodyIndent)
	if err != nil {
		return nil, lineNum + 1, err
	}

	nextLine := findEndOfBlock(lines, bodyStartLine, expectedBodyIndent)

	return &MethodDeclStmt{
		Name:       methodName,
		Parameters: parameters,
		ReturnType: returnType,
		Body:       body,
	}, nextLine, nil
}

// TODO: Replace placeholder for handling index assignment.
func handleIndexAssignment(_, _, value string) string {
	return value
}

func parseElifStatement(lines []string, lineNum, currentIndent int) (*ElifStmt, int, error) {
	line := strings.TrimSpace(lines[lineNum])
	parts := strings.Fields(line)

	if len(parts) < 2 || !strings.HasSuffix(line, ":") {
		return nil, lineNum + 1, fmt.Errorf("elif statement format error at line %d", lineNum+1)
	}

	var (
		colonIndex         = strings.LastIndex(line, ":")
		conditionPart      = strings.TrimSpace(line[4:colonIndex])
		condition          = conditionPart
		expectedBodyIndent = currentIndent + 4
	)

	if currentIndent == 0 {
		bodyStartLine := lineNum + 1
		for bodyStartLine < len(lines) {
			bodyLine := lines[bodyStartLine]
			if strings.TrimSpace(bodyLine) != "" && !strings.HasPrefix(strings.TrimSpace(bodyLine), "#") {
				expectedBodyIndent = getIndentation(bodyLine)
				break
			}
			bodyStartLine++
		}
		if expectedBodyIndent <= currentIndent {
			expectedBodyIndent = currentIndent + 4
		}
	}

	body, err := parseStatements(lines, lineNum+1, expectedBodyIndent)
	if err != nil {
		return nil, lineNum + 1, err
	}
	nextLine := findEndOfBlock(lines, lineNum+1, expectedBodyIndent)

	return &ElifStmt{Condition: condition, Body: body}, nextLine, nil
}

func parseElseStatement(lines []string, lineNum, currentIndent int) (*ElseStmt, int, error) {
	line := strings.TrimSpace(lines[lineNum])

	if line != "else:" {
		return nil, lineNum + 1, fmt.Errorf("else statement format error at line %d", lineNum+1)
	}

	var expectedBodyIndent = currentIndent + 4

	if currentIndent == 0 {
		bodyStartLine := lineNum + 1
		for bodyStartLine < len(lines) {
			bodyLine := lines[bodyStartLine]
			if strings.TrimSpace(bodyLine) != "" && !strings.HasPrefix(strings.TrimSpace(bodyLine), "#") {
				expectedBodyIndent = getIndentation(bodyLine)
				break
			}
			bodyStartLine++
		}
		if expectedBodyIndent <= currentIndent {
			expectedBodyIndent = currentIndent + 4
		}
	}

	body, err := parseStatements(lines, lineNum+1, expectedBodyIndent)
	if err != nil {
		return nil, lineNum + 1, err
	}

	nextLine := findEndOfBlock(lines, lineNum+1, expectedBodyIndent)

	return &ElseStmt{Body: body}, nextLine, nil
}

func LoadModule(moduleName string, baseDir string) (*ModuleInfo, error) {
	if module, exists := LoadedModules[moduleName]; exists {
		return module, nil
	}

	var modulePath string
	if strings.HasPrefix(moduleName, "std/") {
		exePath, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("could not resolve std module path: %v", err)
		}
		baseExeDir := filepath.Dir(exePath)
		moduleName = strings.TrimPrefix(moduleName, "std/")
		modulePath = filepath.Join(baseExeDir, "lib", moduleName+".scar")
		if _, err := os.Stat(modulePath); err != nil {
			return nil, fmt.Errorf("std module '%s' not found at '%s'", moduleName, modulePath)
		}
	} else {
		possiblePaths := []string{
			filepath.Join(baseDir, moduleName+".scar"),
			filepath.Join(baseDir, "modules", moduleName+".scar"),
			filepath.Join(".", moduleName+".scar"),
		}

		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				modulePath = path
				break
			}
		}

		if modulePath == "" {
			return nil, fmt.Errorf("module '%s' not found", moduleName)
		}
	}

	data, err := os.ReadFile(modulePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read module '%s': %v", moduleName, err)
	}

	// Remove comments from the imported module source
	sourceWithoutComments := RemoveComments(string(data))

	program, err := ParseWithIndentation(ReplaceDoubleColonsOutsideStrings(sourceWithoutComments))
	if err != nil {
		return nil, fmt.Errorf("failed to parse module '%s': %v", moduleName, err)
	}

	// Reorder function declarations to handle hoisting
	hoistedStatements, err := HoistFunctions(program.Statements)
	if err != nil {
		return nil, fmt.Errorf("failed to hoist functions in module '%s': %v", moduleName, err)
	}
	program.Statements = hoistedStatements

	module := &ModuleInfo{
		Name:          moduleName,
		FilePath:      modulePath,
		PublicVars:    make(map[string]*VarDeclStmt),
		PublicClasses: make(map[string]*ClassDeclStmt),
		PublicFuncs:   make(map[string]*MethodDeclStmt),
	}

	for _, stmt := range program.Statements {
		if stmt.PubVarDecl != nil {
			varDecl := &VarDeclStmt{
				Type:  stmt.PubVarDecl.Type,
				Name:  stmt.PubVarDecl.Name,
				Value: stmt.PubVarDecl.Value,
			}
			module.PublicVars[stmt.PubVarDecl.Name] = varDecl
		}
		if stmt.PubClassDecl != nil {
			classDecl := &ClassDeclStmt{
				Name:        stmt.PubClassDecl.Name,
				Constructor: stmt.PubClassDecl.Constructor,
				Methods:     stmt.PubClassDecl.Methods,
			}
			module.PublicClasses[stmt.PubClassDecl.Name] = classDecl
		}
		if stmt.PubTopLevelFuncDecl != nil {
			funcDecl := &MethodDeclStmt{
				Name:       stmt.PubTopLevelFuncDecl.Name,
				Parameters: stmt.PubTopLevelFuncDecl.Parameters,
				ReturnType: stmt.PubTopLevelFuncDecl.ReturnType,
				Body:       stmt.PubTopLevelFuncDecl.Body,
			}
			module.PublicFuncs[stmt.PubTopLevelFuncDecl.Name] = funcDecl
		}
	}

	LoadedModules[moduleName] = module
	return module, nil
}

func ReplaceDoubleColonsOutsideStrings(input string) string {
	var result strings.Builder
	inString := false
	for i := 0; i < len(input); i++ {
		ch := input[i]
		if ch == '"' {
			if i > 0 && input[i-1] == '\\' {
				result.WriteByte(ch)
				continue
			}
			inString = !inString
			result.WriteByte(ch)
			continue
		}
		if !inString && ch == ':' && i+1 < len(input) && input[i+1] == ':' {
			result.WriteByte('_')
			i++
			continue
		}

		result.WriteByte(ch)
	}

	return result.String()
}

func parsePubFunctionStatement(lines []string, lineNum, currentIndent int) (*Statement, int, error) {
	line := strings.TrimSpace(lines[lineNum])

	if !strings.HasPrefix(line, "pub fn ") || !strings.HasSuffix(line, ":") {
		return nil, lineNum + 1, fmt.Errorf("invalid pub function declaration at line %d", lineNum+1)
	}

	signature := strings.TrimSpace(line[7 : len(line)-1]) // Remove "pub fn " and ":"
	parenStart := strings.Index(signature, "(")
	if parenStart == -1 {
		return nil, lineNum + 1, fmt.Errorf("pub function declaration missing parameters at line %d", lineNum+1)
	}

	funcName := strings.TrimSpace(signature[:parenStart])
	parenEnd := strings.Index(signature, ")")
	if parenEnd == -1 || parenEnd <= parenStart {
		return nil, lineNum + 1, fmt.Errorf("pub function declaration missing closing parenthesis at line %d", lineNum+1)
	}

	paramsStr := strings.TrimSpace(signature[parenStart+1 : parenEnd])
	var parameters []*MethodParameter
	if paramsStr != "" {
		paramList := strings.SplitSeq(paramsStr, ",")
		for paramStr := range paramList {
			paramStr = strings.TrimSpace(paramStr)
			paramParts := strings.Fields(paramStr)
			if len(paramParts) == 2 {
				param := &MethodParameter{
					Type: paramParts[0],
					Name: paramParts[1],
				}
				parameters = append(parameters, param)
			} else if len(paramParts) == 1 {
				param := &MethodParameter{
					Type: "int",
					Name: paramParts[0],
				}
				parameters = append(parameters, param)
			}
		}
	}

	returnTypePart := strings.TrimSpace(signature[parenEnd+1:])
	var returnType string
	if strings.HasPrefix(returnTypePart, "->") {
		returnType = strings.TrimSpace(returnTypePart[2:])
	} else {
		returnType = "void"
	}
	var (
		expectedBodyIndent = currentIndent + 4
		bodyStartLine      = lineNum + 1
	)
	for bodyStartLine < len(lines) {
		bodyLine := lines[bodyStartLine]
		if strings.TrimSpace(bodyLine) != "" && !strings.HasPrefix(strings.TrimSpace(bodyLine), "#") {
			expectedBodyIndent = getIndentation(bodyLine)
			break
		}
		bodyStartLine++
	}

	body, err := parseStatements(lines, bodyStartLine, expectedBodyIndent)
	if err != nil {
		return nil, lineNum + 1, err
	}

	var (
		nextLine    = findEndOfBlock(lines, bodyStartLine, expectedBodyIndent)
		pubFuncDecl = &PubTopLevelFuncDeclStmt{
			Name:       funcName,
			Parameters: parameters,
			ReturnType: returnType,
			Body:       body,
		}
	)

	return &Statement{PubTopLevelFuncDecl: pubFuncDecl}, nextLine, nil
}

func splitRespectingParens(input string) []string {
	var (
		result     []string
		current    strings.Builder
		parenCount = 0
		inQuotes   = false
	)

	for i, char := range input {
		switch char {
		case '"':
			if i == 0 || input[i-1] != '\\' {
				inQuotes = !inQuotes
			}
			current.WriteRune(char)
		case '(':
			if !inQuotes {
				parenCount++
			}
			current.WriteRune(char)
		case ')':
			if !inQuotes {
				parenCount--
			}
			current.WriteRune(char)
		case ',':
			if !inQuotes && parenCount == 0 {
				result = append(result, strings.TrimSpace(current.String()))
				current.Reset()
			} else {
				current.WriteRune(char)
			}
		default:
			current.WriteRune(char)
		}
	}

	if current.Len() > 0 {
		result = append(result, strings.TrimSpace(current.String()))
	}

	return result
}

func parseBulkImport(lines []string, lineNum int) (*Statement, int, error) {
	var imports []*ImportStmt
	currentLine := lineNum + 1

	for currentLine < len(lines) {
		line := lines[currentLine]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			currentLine++
			continue
		}
		if getIndentation(line) == 0 {
			break
		}
		moduleNames := strings.SplitSeq(trimmed, ",")
		for moduleName := range moduleNames {
			moduleName = strings.TrimSpace(strings.Trim(moduleName, "\""))
			if moduleName != "" {
				imports = append(imports, &ImportStmt{Module: moduleName})
			}
		}

		currentLine++
	}

	if len(imports) == 0 {
		return nil, lineNum + 1, fmt.Errorf("bulk import has no modules at line %d", lineNum+1)
	}
	return &Statement{Import: imports[0]}, currentLine, nil
}
