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
	"strconv"
	"strings"
)

// Removes both full-line and inline comments from source code
// but preserves comments inside $raw blocks (for C preprocessor directives)
func RemoveComments(source string) string {
	var (
		result        strings.Builder
		inString      = false
		inRawBlock    = false
		rawParenDepth = 0
		lineStart     = 0
	)

	for i := 0; i < len(source); i++ {
		if source[i] == '"' && (i == 0 || source[i-1] != '\\') {
			inString = !inString
		}

		// Check for $raw block start
		if !inString && i+6 < len(source) && source[i:i+6] == "$raw (" {
			inRawBlock = true
			rawParenDepth = 1
			for j := range 6 {
				result.WriteByte(source[i+j])
			}
			i += 5
			continue
		}

		// Track parentheses depth inside $raw blocks
		if inRawBlock && !inString {
			switch source[i] {
			case '(':
				rawParenDepth++
			case ')':
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
		var (
			importLine  = strings.TrimSpace(line[6:])
			moduleNames = strings.Split(importLine, ",")
		)

		for _, moduleName := range moduleNames {
			moduleName = strings.TrimSpace(strings.Trim(moduleName, "\""))
			if moduleName != "" {
				imports = append(imports, &ImportStmt{Module: moduleName})
			}
		}
	} else {
		currentLine := startLine + 1
		for currentLine < len(lines) {
			var (
				line    = lines[currentLine]
				trimmed = strings.TrimSpace(line)
			)

			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				currentLine++
				continue
			}

			if getIndentation(line) == 0 {
				break
			}

			moduleNames := strings.Split(trimmed, ",")
			for _, moduleName := range moduleNames {
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
	case "struct":
		return parsePubStructStatement(lines, lineNum, currentIndent)
	case "fn":
		return parsePubFunctionStatement(lines, lineNum, currentIndent)
	case "allocate":
		var varType, varName, size string
		var equalIndex int

		for i, part := range parts {
			if part == "=" {
				equalIndex = i
				break
			}
		}

		if equalIndex == 0 || equalIndex >= len(parts)-1 {
			return nil, lineNum + 1, fmt.Errorf("pub allocate statement format error at line %d (expected: pub allocate [ref] type name = size)", lineNum+1)
		}

		if len(parts) >= 7 && parts[2] == "ref" {
			// pub allocate ref type name = size
			if equalIndex != 5 {
				return nil, lineNum + 1, fmt.Errorf("pub allocate statement format error at line %d (expected: pub allocate ref type name = size)", lineNum+1)
			}
			varType = "ref " + parts[3]
			varName = parts[4]
		} else {
			// pub allocate type name = size
			if equalIndex != 4 {
				return nil, lineNum + 1, fmt.Errorf("pub allocate statement format error at line %d (expected: pub allocate type name = size)", lineNum+1)
			}
			varType = parts[2]
			varName = parts[3]
		}

		size = strings.Join(parts[equalIndex+1:], " ")
		return &Statement{PubAllocate: &PubAllocateStmt{Type: varType, Name: varName, Size: size}}, lineNum + 1, nil
	default:
		if parts[1] == "fixed" {
			// Supported forms:
			//
			// - pub fixed val <type> <name> = <value>
			// - pub fixed val <name> = <value>  (type inferred)
			// - pub fixed <type> <name> = <value>
			if len(parts) >= 7 && parts[2] == "val" && parts[4] == "=" && isValidType(parts[3]) {
				var (
					varType = parts[3]
					varName = parts[5]
					value   = strings.Join(parts[6:], " ")
				)
				return &Statement{PubVarDecl: &PubVarDeclStmt{Type: varType, Name: varName, Value: value, IsConst: true, IsFixed: true}}, lineNum + 1, nil
			}
			if len(parts) >= 5 && parts[2] == "val" && parts[3] != "=" && parts[4] == "=" {
				// pub fixed val <name> = <value>
				var (
					varName = parts[3]
					value   = strings.Join(parts[5:], " ")
					varType = inferBasicTypeFromValue(value)
				)
				if varType == "string" && !(strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) {
					value = fmt.Sprintf("\"%s\"", value)
				}
				return &Statement{PubVarDecl: &PubVarDeclStmt{Type: varType, Name: varName, Value: value, IsConst: true, IsFixed: true}}, lineNum + 1, nil
			}
			if len(parts) >= 5 && parts[3] == "=" && isValidType(parts[2]) {
				// pub fixed <type> <name> = <value>
				var (
					varType = parts[2]
					varName = parts[3]
					value   = strings.Join(parts[4:], " ")
				)
				return &Statement{PubVarDecl: &PubVarDeclStmt{Type: varType, Name: varName, Value: value, IsFixed: true}}, lineNum + 1, nil
			}
			return nil, lineNum + 1, fmt.Errorf("invalid pub fixed declaration at line %d", lineNum+1)
		}
		// "pub val <name> = <value>" (inferred) and "pub val <type> <name> = <value>"
		if parts[1] == "val" {
			if len(parts) >= 6 && parts[4] == "=" && isValidType(parts[2]) {
				var (
					varType = parts[2]
					varName = parts[3]
					value   = strings.Join(parts[5:], " ")
				)
				return &Statement{PubVarDecl: &PubVarDeclStmt{Type: varType, Name: varName, Value: value, IsConst: true}}, lineNum + 1, nil
			}
			if len(parts) >= 5 && parts[3] == "=" {
				var (
					varName = parts[2]
					value   = strings.Join(parts[4:], " ")
					varType = inferBasicTypeFromValue(value)
				)
				if varType == "string" && !(strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) {
					value = fmt.Sprintf("\"%s\"", value)
				}
				return &Statement{PubVarDecl: &PubVarDeclStmt{Type: varType, Name: varName, Value: value, IsConst: true}}, lineNum + 1, nil
			}
			return nil, lineNum + 1, fmt.Errorf("invalid pub val declaration at line %d", lineNum+1)
		}
		if len(parts) >= 5 && parts[3] == "=" && isValidType(parts[1]) {
			var (
				varType = parts[1]
				varName = parts[2]
				value   = strings.Join(parts[4:], " ")
			)
			return &Statement{PubVarDecl: &PubVarDeclStmt{Type: varType, Name: varName, Value: value}}, lineNum + 1, nil
		}
		return nil, lineNum + 1, fmt.Errorf("invalid pub declaration at line %d", lineNum+1)
	}
}

func inferBasicTypeFromValue(value string) string {
	v := strings.TrimSpace(value)
	if len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"') {
		if len(v) > 256 {
			return "lstring"
		}
		return "string"
	}
	if len(v) >= 3 && v[0] == '\'' && v[len(v)-1] == '\'' {
		return "char"
	}
	if v == "true" || v == "false" {
		return "bool"
	}
	if strings.ContainsAny(v, ".eE") {
		if _, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSuffix(v, "f"), "F"), 64); err == nil {
			if strings.HasSuffix(v, "f") || strings.HasSuffix(v, "F") {
				return "float"
			}
			return "double"
		}
	}
	if _, err := strconv.ParseInt(v, 10, 64); err == nil {
		return "int"
	}
	return "int"
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
						paramList := strings.Split(paramsStr, ",")
						for _, paramStr := range paramList {
							paramStr = strings.TrimSpace(paramStr)
							paramParts := strings.Fields(paramStr)

							param := &MethodParameter{}

							if len(paramParts) >= 3 && paramParts[0] == "ref" {
								param.IsRef = true
								param.Type = paramParts[1]
								param.Name = paramParts[2]
								parameters = append(parameters, param)
							} else if len(paramParts) == 2 {
								param.Type = paramParts[0]
								param.Name = paramParts[1]
								param.IsRef = false
								parameters = append(parameters, param)
							} else if len(paramParts) == 1 {
								param.Type = "int"
								param.Name = paramParts[0]
								param.IsRef = false
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
		} else if strings.HasPrefix(trimmed, "fn ") || strings.HasPrefix(trimmed, "stat fn ") {
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

func parsePubStructStatement(lines []string, lineNum, currentIndent int) (*Statement, int, error) {
	line := strings.TrimSpace(lines[lineNum])
	parts := strings.Fields(line)

	if len(parts) < 3 || !strings.HasSuffix(line, ":") {
		return nil, lineNum + 1, fmt.Errorf("pub struct declaration format error at line %d", lineNum+1)
	}

	var (
		structName         = strings.TrimSuffix(parts[2], ":")
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

	var fields []*StructField
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
			return nil, nextLine + 1, fmt.Errorf("unexpected indentation in struct body at line %d", nextLine+1)
		}

		fieldParts := strings.Fields(trimmed)
		if len(fieldParts) != 2 {
			return nil, nextLine + 1, fmt.Errorf("invalid field declaration format at line %d (expected: type field_name)", nextLine+1)
		}

		fieldType := fieldParts[0]
		fieldName := fieldParts[1]

		if !isValidType(fieldType) {
			return nil, nextLine + 1, fmt.Errorf("invalid field type '%s' at line %d", fieldType, nextLine+1)
		}

		fields = append(fields, &StructField{
			Type: fieldType,
			Name: fieldName,
		})

		nextLine++
	}

	structStmt := &PubStructDeclStmt{
		Name:   structName,
		Fields: fields,
	}

	return &Statement{PubStructDecl: structStmt}, nextLine, nil
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

	var (
		constructor *ConstructorStmt
		methods     []*MethodDeclStmt
		nextLine    = lineNum + 1
	)

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

							param := &MethodParameter{}

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
		} else if strings.HasPrefix(trimmed, "fn ") || strings.HasPrefix(trimmed, "stat fn ") {
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

func parseStructStatement(lines []string, lineNum, currentIndent int) (*Statement, int, error) {
	line := strings.TrimSpace(lines[lineNum])
	parts := strings.Fields(line)

	if len(parts) < 2 || !strings.HasSuffix(line, ":") {
		return nil, lineNum + 1, fmt.Errorf("struct declaration format error at line %d", lineNum+1)
	}

	var (
		structName         = strings.TrimSuffix(parts[1], ":")
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

	var (
		fields   []*StructField
		nextLine = lineNum + 1
	)

	for nextLine < len(lines) {
		var (
			line    = lines[nextLine]
			trimmed = strings.TrimSpace(line)
		)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			nextLine++
			continue
		}

		indent := getIndentation(line)
		if indent < expectedBodyIndent {
			break
		}

		if indent != expectedBodyIndent {
			return nil, nextLine + 1, fmt.Errorf("unexpected indentation in struct body at line %d", nextLine+1)
		}

		fieldParts := strings.Fields(trimmed)
		if len(fieldParts) != 2 {
			return nil, nextLine + 1, fmt.Errorf("invalid field declaration format at line %d (expected: type field_name)", nextLine+1)
		}

		var (
			fieldType = fieldParts[0]
			fieldName = fieldParts[1]
		)

		if !isValidType(fieldType) {
			return nil, nextLine + 1, fmt.Errorf("invalid field type '%s' at line %d", fieldType, nextLine+1)
		}

		fields = append(fields, &StructField{
			Type: fieldType,
			Name: fieldName,
		})

		nextLine++
	}

	structStmt := &StructDeclStmt{
		Name:   structName,
		Fields: fields,
	}

	return &Statement{StructDecl: structStmt}, nextLine, nil
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
		rawParams := strings.Split(paramsStr, ",")
		for i, raw := range rawParams {
			param := strings.TrimSpace(raw)
			if param == "" {
				continue
			}
			if param == "..." {
				if i != len(rawParams)-1 {
					return nil, lineNum + 1, fmt.Errorf("variadic marker '...' must be the last parameter at line %d", lineNum+1)
				}
				parameters = append(parameters, &MethodParameter{IsVarargs: true})
				continue
			}

			paramParts := strings.Fields(param)

			var paramType, paramName string
			var isRef bool = false
			isList := false
			listType := ""

			if len(paramParts) >= 3 && paramParts[0] == "ref" {
				isRef = true
				paramType = paramParts[1]
				paramName = paramParts[2]
			} else if len(paramParts) == 2 {
				paramType = paramParts[0]
				paramName = paramParts[1]
			} else {
				return nil, lineNum + 1, fmt.Errorf("invalid parameter format at line %d", lineNum+1)
			}

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
				IsRef:    isRef,
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

	// Check for static method declaration
	isStatic := false
	if strings.HasPrefix(line, "stat fn ") {
		isStatic = true
		line = "fn " + strings.TrimSpace(line[8:]) // Remove "stat " prefix
	} else if !strings.HasPrefix(line, "fn ") {
		return nil, lineNum + 1, fmt.Errorf("invalid method declaration at line %d", lineNum+1)
	}

	if !strings.HasSuffix(line, ":") {
		return nil, lineNum + 1, fmt.Errorf("invalid method declaration at line %d", lineNum+1)
	}

	signature := strings.TrimSpace(line[3 : len(line)-1])
	parenStart := strings.Index(signature, "(")
	if parenStart == -1 {
		return nil, lineNum + 1, fmt.Errorf("method declaration missing parameters at line %d", lineNum+1)
	}

	methodName := strings.TrimSpace(signature[:parenStart])
	if methodName == "new" {
		return nil, lineNum + 1, fmt.Errorf("method name 'new' is reserved and cannot be used in class at line %d", lineNum+1)
	}

	parenEnd := strings.Index(signature, ")")
	if parenEnd == -1 || parenEnd <= parenStart {
		return nil, lineNum + 1, fmt.Errorf("method declaration missing closing parenthesis at line %d", lineNum+1)
	}

	paramsStr := strings.TrimSpace(signature[parenStart+1 : parenEnd])
	var parameters []*MethodParameter
	if paramsStr != "" {
		parts := strings.Split(paramsStr, ",")
		for i, raw := range parts {
			paramStr := strings.TrimSpace(raw)
			if paramStr == "" {
				continue
			}
			if paramStr == "..." {
				if i != len(parts)-1 {
					return nil, lineNum + 1, fmt.Errorf("variadic marker '...' must be the last parameter at line %d", lineNum+1)
				}
				parameters = append(parameters, &MethodParameter{IsVarargs: true})
				continue
			}

			paramParts := strings.Fields(paramStr)

			param := &MethodParameter{}

			if len(paramParts) >= 3 && paramParts[0] == "ref" {
				param.IsRef = true
				param.Type = paramParts[1]
				param.Name = paramParts[2]
				parameters = append(parameters, param)
			} else if len(paramParts) == 2 {
				param.Type = paramParts[0]
				param.Name = paramParts[1]
				param.IsRef = false
				parameters = append(parameters, param)
			} else if len(paramParts) == 1 {
				param.Type = "int"
				param.Name = paramParts[0]
				param.IsRef = false
				parameters = append(parameters, param)
			} else {
				return nil, lineNum + 1, fmt.Errorf("invalid parameter format at line %d", lineNum+1)
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
		IsStatic:   isStatic,
	}, nextLine, nil
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
	return LoadModuleWithCycleDetection(moduleName, baseDir, make(map[string]bool))
}

func LoadModuleWithCycleDetection(moduleName string, baseDir string, loadingStack map[string]bool) (*ModuleInfo, error) {
	if module, exists := LoadedModules[moduleName]; exists {
		return module, nil
	}

	// Check for circular dependency
	if loadingStack[moduleName] {
		return nil, fmt.Errorf("circular dependency detected: module '%s' is already being loaded", moduleName)
	}

	// Add to loading stack to detect cycles
	loadingStack[moduleName] = true
	defer delete(loadingStack, moduleName)

	var modulePath string
	isStdModule := false
	if strings.HasPrefix(moduleName, "std/") {
		exePath, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("could not resolve std module path: %v", err)
		}
		baseExeDir := filepath.Dir(exePath)
		moduleName = strings.TrimPrefix(moduleName, "std/")
		modulePath = filepath.Join(baseExeDir, "lib", moduleName+".scar")
		isStdModule = true
		if _, err := os.Stat(modulePath); err != nil {
			return nil, fmt.Errorf("std module '%s' not found at '%s'", moduleName, modulePath)
		}
	} else {
		possiblePaths := []string{
			filepath.Join(baseDir, moduleName+".scar"),
			filepath.Join(baseDir, "modules", moduleName+".scar"),
			filepath.Join(".", moduleName+".scar"),
		}

		if strings.Contains(moduleName, "/") || strings.Contains(moduleName, "\\") {
			normalizedPath := filepath.FromSlash(moduleName)
			possiblePaths = append([]string{
				filepath.Join(baseDir, normalizedPath+".scar"),
				filepath.Join(".", normalizedPath+".scar"),
			}, possiblePaths...)
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

	sourceWithoutComments := RemoveComments(string(data))
	sourceWithMacros := processModuleMacros(sourceWithoutComments)
	program, err := InnerParseWithIndentation(ReplaceDoubleColonsOutsideStrings(sourceWithMacros), modulePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse module '%s': %v", moduleName, err)
	}

	hoistedStatements, err := HoistFunctions(program.Statements)
	if err != nil {
		return nil, fmt.Errorf("failed to hoist functions in module '%s': %v", moduleName, err)
	}
	program.Statements = hoistedStatements

	module := &ModuleInfo{
		Name:            moduleName,
		FilePath:        modulePath,
		PublicVars:      make(map[string]*VarDeclStmt),
		PublicClasses:   make(map[string]*ClassDeclStmt),
		PublicFuncs:     make(map[string]*MethodDeclStmt),
		PublicMacros:    make(map[string]*MacroDeclStmt),
		ExternalImports: []string{},
		LocalImports:    []string{},
	}

	for _, stmt := range program.Statements {
		if stmt.ExternalImport != nil {
			module.ExternalImports = append(module.ExternalImports, stmt.ExternalImport.Header)
		}
		if stmt.LocalImport != nil {
			module.LocalImports = append(module.LocalImports, stmt.LocalImport.Header)
		}
		if stmt.PubVarDecl != nil {
			varDecl := &VarDeclStmt{
				Type:    stmt.PubVarDecl.Type,
				Name:    stmt.PubVarDecl.Name,
				Value:   stmt.PubVarDecl.Value,
				IsConst: stmt.PubVarDecl.IsConst,
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
		if stmt.PubMacroDecl != nil {
			macroDecl := &MacroDeclStmt{
				Name:       stmt.PubMacroDecl.Name,
				Parameters: stmt.PubMacroDecl.Parameters,
				Body:       stmt.PubMacroDecl.Body,
			}
			module.PublicMacros[stmt.PubMacroDecl.Name] = macroDecl
			qualifiedName := module.Name + "_" + stmt.PubMacroDecl.Name
			RegisteredMacros[qualifiedName] = true
		}
	}

	for _, importStmt := range program.Imports {
		if importStmt != nil && importStmt.Module != "" {
			importBaseDir := filepath.Dir(modulePath)
			if strings.HasPrefix(importStmt.Module, "std/") {
				importBaseDir = baseDir
			}
			_, err := LoadModuleWithCycleDetection(importStmt.Module, importBaseDir, loadingStack)
			if err != nil {
				return nil, fmt.Errorf("failed to load dependency '%s' for module '%s': %v", importStmt.Module, moduleName, err)
			}
		}
	}

	{
		explicit := make(map[string]bool)
		for _, imp := range program.Imports {
			if imp != nil && imp.Module != "" {
				explicit[imp.Module] = true
			}
		}

		src := sourceWithMacros
		inStr := false
		var current strings.Builder
		var refs = make(map[string]bool)
		for i := 0; i < len(src); i++ {
			ch := src[i]
			if ch == '"' && (i == 0 || src[i-1] != '\\') {
				inStr = !inStr
				current.Reset()
				continue
			}
			if inStr {
				continue
			}
			if ch == ':' && i+1 < len(src) && src[i+1] == ':' {
				token := strings.TrimSpace(current.String())
				if token != "" {
					j := len(token) - 1
					for j >= 0 {
						c := token[j]
						if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
							j--
							continue
						}
						break
					}
					mod := token[j+1:]
					if mod != "" && mod != moduleName {
						ref := mod
						if isStdModule {
							ref = "std/" + mod
						}
						if !explicit[ref] {
							refs[ref] = true
						}
					}
				}
				current.Reset()
				i++ // skip second ':'
				continue
			}
			if len := current.Len(); len > 128 {
				current.Reset()
			}
			current.WriteByte(ch)
		}

		if len(refs) > 0 {
			importBaseDir := filepath.Dir(modulePath)
			for ref := range refs {
				if _, exists := LoadedModules[ref]; exists {
					continue
				}
				// Cycle detection
				if loadingStack[ref] {
					continue
				}
				if _, err := LoadModuleWithCycleDetection(ref, importBaseDir, loadingStack); err != nil {
					// Non-fatal: continue loading other inferred modules
					continue
				}
			}
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
		rawParams := strings.Split(paramsStr, ",")
		for i, raw := range rawParams {
			paramStr := strings.TrimSpace(raw)
			if paramStr == "" {
				continue
			}
			if paramStr == "..." {
				if i != len(rawParams)-1 {
					return nil, lineNum + 1, fmt.Errorf("variadic marker '...' must be the last parameter at line %d", lineNum+1)
				}
				parameters = append(parameters, &MethodParameter{IsVarargs: true})
				continue
			}

			paramParts := strings.Fields(paramStr)

			param := &MethodParameter{}

			if len(paramParts) >= 3 && paramParts[0] == "ref" {
				param.IsRef = true
				param.Type = paramParts[1]
				param.Name = paramParts[2]
				parameters = append(parameters, param)
			} else if len(paramParts) == 2 {
				param.Type = paramParts[0]
				param.Name = paramParts[1]
				param.IsRef = false
				parameters = append(parameters, param)
			} else if len(paramParts) == 1 {
				param.Type = "int"
				param.Name = paramParts[0]
				param.IsRef = false
				parameters = append(parameters, param)
			} else {
				return nil, lineNum + 1, fmt.Errorf("invalid parameter format at line %d", lineNum+1)
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
		moduleNames := strings.Split(trimmed, ",")
		for _, moduleName := range moduleNames {
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

func processModuleMacros(source string) string {
	// fmt.Println("=== DEBUG: Original source before macro expansion ===")
	// fmt.Println(source)
	// fmt.Println("=== END Original source ===")

	macros := collectModuleMacroDefinitions(source)

	// fmt.Printf("=== DEBUG: Found %d macros ===\n", len(macros))
	for name, macro := range macros {
		fmt.Printf("Macro: %s(%s)\n", name, strings.Join(macro.Parameters, ", "))
		// for i, bodyLine := range macro.Body {
		// fmt.Printf("  [%d]: %s\n", i, bodyLine)
		// }
	}
	// fmt.Println("=== END Macros ===")

	expanded := expandModuleMacros(source, macros)

	// fmt.Println("=== DEBUG: Expanded source after macro expansion ===")
	// fmt.Println(expanded)
	// fmt.Println("=== END Expanded source ===")

	return expanded
}

type ModuleMacro struct {
	Name       string
	Parameters []string
	Body       []string
}

func collectModuleMacroDefinitions(source string) map[string]*ModuleMacro {
	macros := make(map[string]*ModuleMacro)
	lines := strings.Split(source, "\n")

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		if (strings.HasPrefix(line, "macro ") || strings.HasPrefix(line, "pub macro ")) && strings.HasSuffix(line, ":") {
			macro, endLine := parseModuleMacroDefinition(lines, i)
			if macro != nil {
				macros[macro.Name] = macro
				i = endLine - 1
			}
		}
	}

	return macros
}

func parseModuleMacroDefinition(lines []string, startLine int) (*ModuleMacro, int) {
	line := strings.TrimSpace(lines[startLine])
	signature := strings.TrimSpace(line[:len(line)-1])

	var macroName string
	var paramStr string

	if strings.HasPrefix(signature, "pub macro ") {
		signature = strings.TrimSpace(signature[10:]) // Remove "pub macro "
	} else {
		signature = strings.TrimSpace(signature[6:]) // Remove "macro "
	}

	parenStart := strings.Index(signature, "(")
	if parenStart == -1 {
		return nil, startLine + 1
	}

	macroName = strings.TrimSpace(signature[:parenStart])
	parenEnd := strings.LastIndex(signature, ")")
	if parenEnd == -1 || parenEnd <= parenStart {
		return nil, startLine + 1
	}

	paramStr = strings.TrimSpace(signature[parenStart+1 : parenEnd])

	var parameters []string
	if paramStr != "" {
		paramList := strings.Split(paramStr, ",")
		for _, param := range paramList {
			param = strings.TrimSpace(param)
			if param != "" {
				parameters = append(parameters, param)
			}
		}
	}

	var body []string
	currentLine := startLine + 1
	macroIndent := -1

	for currentLine < len(lines) {
		bodyLine := lines[currentLine]
		bodyTrimmed := strings.TrimSpace(bodyLine)

		if bodyTrimmed == "" || strings.HasPrefix(bodyTrimmed, "#") {
			currentLine++
			continue
		}

		indent := getModuleMacroIndentation(bodyLine)
		if macroIndent == -1 {
			macroIndent = indent
		}

		if indent < macroIndent {
			break
		}

		relativeIndent := indent - macroIndent
		relativeLine := strings.Repeat(" ", relativeIndent) + bodyTrimmed
		body = append(body, relativeLine)
		currentLine++
	}

	return &ModuleMacro{
		Name:       macroName,
		Parameters: parameters,
		Body:       body,
	}, currentLine
}

func getModuleMacroIndentation(line string) int {
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

func expandModuleMacros(source string, macros map[string]*ModuleMacro) string {
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

				indent := getModuleMacroIndentation(bodyLine)
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

		expandedLine := expandModuleMacroCallsInLine(line, macros)
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

	return strings.Join(result, "\n")
}

func expandModuleMacroCallsInLine(line string, macros map[string]*ModuleMacro) string {
	result := line

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
			args := parseModuleMacroArguments(argsStr)

			expanded := expandModuleMacro(macro, args)

			before := result[:start]
			after := result[end+1:]

			indent := getModuleMacroLineIndentation(result[:start])
			indentedExpanded := addModuleMacroIndentationToExpansion(expanded, indent)

			result = before + indentedExpanded + after
		}
	}

	return result
}

func parseModuleMacroArguments(argsStr string) []string {
	if strings.TrimSpace(argsStr) == "" {
		return []string{}
	}

	var (
		args       []string
		current    strings.Builder
		parenCount = 0
		inString   = false
	)

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

func expandModuleMacro(macro *ModuleMacro, args []string) string {
	if len(args) != len(macro.Parameters) {
		return ""
	}

	var expandedLines []string

	for _, bodyLine := range macro.Body {
		expanded := bodyLine

		for i, param := range macro.Parameters {
			arg := args[i]
			expanded = strings.ReplaceAll(expanded, param, arg)
		}

		expandedLines = append(expandedLines, expanded)
	}

	return strings.Join(expandedLines, "\n")
}

func getModuleMacroLineIndentation(text string) string {
	var indent strings.Builder
	for _, char := range text {
		if char == ' ' || char == '\t' {
			indent.WriteRune(char)
		} else {
			break
		}
	}
	return indent.String()
}

func addModuleMacroIndentationToExpansion(expanded, indent string) string {
	lines := strings.Split(expanded, "\n")
	var result []string

	for i, line := range lines {
		if i == 0 {
			result = append(result, strings.TrimLeft(line, " \t"))
		} else {
			result = append(result, indent+line)
		}
	}

	return strings.Join(result, "\n")
}
