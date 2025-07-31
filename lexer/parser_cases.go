// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Contains the parser cases for the parser/lexer operations.

package lexer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
		return nil, lineNum + 1, fmt.Errorf("pub statement requires a declaration at line %d", lineNum+1)
	}

	switch parts[1] {
	case "class":
		return parsePubClassStatement(lines, lineNum, currentIndent)
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

	className := parts[2]
	if strings.HasSuffix(className, ":") {
		className = className[:len(className)-1]
	}

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
						paramList := strings.Split(paramsStr, ",")
						for _, paramStr := range paramList {
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

	className := parts[1]
	if strings.HasSuffix(className, ":") {
		className = className[:len(className)-1]
	}

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
		return nil, lineNum + 1, fmt.Errorf("invalid top-level function declaration at line %d", lineNum+1)
	}

	signature := strings.TrimSpace(line[3 : len(line)-1])
	parenStart := strings.Index(signature, "(")
	if parenStart == -1 {
		return nil, lineNum + 1, fmt.Errorf("function declaration missing parameters at line %d", lineNum+1)
	}

	funcName := strings.TrimSpace(signature[:parenStart])
	parenEnd := strings.Index(signature, ")")
	if parenEnd == -1 || parenEnd <= parenStart {
		return nil, lineNum + 1, fmt.Errorf("function declaration missing closing parenthesis at line %d", lineNum+1)
	}

	paramsStr := strings.TrimSpace(signature[parenStart+1 : parenEnd])
	var parameters []*MethodParameter
	if paramsStr != "" {
		paramList := strings.Split(paramsStr, ",")
		for _, paramStr := range paramList {
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

	funcDecl := &TopLevelFuncDeclStmt{
		Name:       funcName,
		Parameters: parameters,
		ReturnType: returnType,
		Body:       body,
	}

	return &Statement{TopLevelFuncDecl: funcDecl}, nextLine, nil
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
func handleIndexAssignment(line, varName, value string) string {
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
	possiblePaths := []string{
		filepath.Join(baseDir, moduleName+".x"),
		filepath.Join(baseDir, "modules", moduleName+".x"),
		filepath.Join(".", moduleName+".x"),
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

	data, err := os.ReadFile(modulePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read module '%s': %v", moduleName, err)
	}

	program, err := ParseWithIndentation(string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to parse module '%s': %v", moduleName, err)
	}

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
	}

	LoadedModules[moduleName] = module
	return module, nil
}
