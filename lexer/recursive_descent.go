// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Contains the recursive descent parser for the scar programming language.

package lexer

import (
	"fmt"
	"slices"
	"strings"
)

func parseStatements(lines []string, startLine, expectedIndent int) ([]*Statement, error) {
	var (
		statements []*Statement
		i          = startLine
	)

	for i < len(lines) {
		line := lines[i]

		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			i++
			continue
		}

		indent := getIndentation(line)

		if expectedIndent == 0 && len(statements) == 0 {
			expectedIndent = indent
		}

		if indent < expectedIndent {
			break
		}

		if indent > expectedIndent {
			return nil, fmt.Errorf("unexpected indentation at line %d (expected %d, got %d)", i+1, expectedIndent, indent)
		}

		stmt, nextLine, err := parseStatement(lines, i, indent)
		if err != nil {
			return nil, err
		}

		statements = append(statements, stmt)
		i = nextLine
	}

	return statements, nil
}

func splitRespectingQuotes(input string) []string {
	var (
		result          []string
		current         strings.Builder
		parenDepth      = 0
		inString        = false
		stringDelimiter byte
		escapeNext      = false
	)

	for i := 0; i < len(input); i++ {
		ch := input[i]

		if inString {
			if escapeNext {
				escapeNext = false
				current.WriteByte(ch)
				continue
			}
			if ch == '\\' {
				escapeNext = true
				current.WriteByte(ch)
				continue
			}
			if ch == stringDelimiter {
				inString = false
				current.WriteByte(ch)
				continue
			}
			current.WriteByte(ch)
			continue
		}

		switch ch {
		case '"', '\'':
			inString = true
			stringDelimiter = ch
			current.WriteByte(ch)
		case '(':
			parenDepth++
			current.WriteByte(ch)
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
			current.WriteByte(ch)
		case ',':
			if parenDepth == 0 {
				result = append(result, strings.TrimSpace(current.String()))
				current.Reset()
			} else {
				current.WriteByte(ch)
			}
		default:
			current.WriteByte(ch)
		}
	}

	if current.Len() > 0 {
		result = append(result, strings.TrimSpace(current.String()))
	}

	return result
}

func parseMacroDeclaration(lines []string, lineNum, currentIndent int) (*Statement, int, error) {
	line := strings.TrimSpace(lines[lineNum])

	if !strings.HasSuffix(line, ":") {
		return nil, lineNum + 1, fmt.Errorf("macro declaration must end with ':' at line %d", lineNum+1)
	}

	signature := strings.TrimSpace(line[:len(line)-1])

	parenStart := strings.Index(signature, "(")
	if parenStart == -1 {
		return nil, lineNum + 1, fmt.Errorf("macro declaration missing parameters at line %d", lineNum+1)
	}

	macroName := strings.TrimSpace(signature[6:parenStart])
	if macroName == "" {
		return nil, lineNum + 1, fmt.Errorf("macro declaration missing name at line %d", lineNum+1)
	}

	parenEnd := strings.LastIndex(signature, ")")
	if parenEnd == -1 || parenEnd <= parenStart {
		return nil, lineNum + 1, fmt.Errorf("macro declaration missing closing parenthesis at line %d", lineNum+1)
	}

	paramsStr := strings.TrimSpace(signature[parenStart+1 : parenEnd])
	var parameters []string
	if paramsStr != "" {
		paramList := splitRespectingQuotes(paramsStr)
		for _, param := range paramList {
			param = strings.TrimSpace(param)
			if param != "" {
				parameters = append(parameters, param)
			}
		}
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

	var body []string
	nextLine := lineNum + 1

	for nextLine < len(lines) {
		bodyLine := lines[nextLine]
		trimmed := strings.TrimSpace(bodyLine)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			nextLine++
			continue
		}

		indent := getIndentation(bodyLine)
		if indent < expectedBodyIndent {
			break
		}

		if indent != expectedBodyIndent {
			return nil, nextLine + 1, fmt.Errorf("unexpected indentation in macro body at line %d", nextLine+1)
		}

		body = append(body, trimmed)
		nextLine++
	}

	if len(body) == 0 {
		return nil, lineNum + 1, fmt.Errorf("macro body cannot be empty at line %d", lineNum+1)
	}

	macroStmt := &MacroDeclStmt{
		Name:       macroName,
		Parameters: parameters,
		Body:       body,
	}

	RegisteredMacros[macroName] = true

	return &Statement{MacroDecl: macroStmt}, nextLine, nil
}

func parsePubMacroDeclaration(lines []string, lineNum, currentIndent int) (*Statement, int, error) {
	line := strings.TrimSpace(lines[lineNum])

	if !strings.HasSuffix(line, ":") {
		return nil, lineNum + 1, fmt.Errorf("pub macro declaration must end with ':' at line %d", lineNum+1)
	}

	signature := strings.TrimSpace(line[:len(line)-1])

	parenStart := strings.Index(signature, "(")
	if parenStart == -1 {
		return nil, lineNum + 1, fmt.Errorf("pub macro declaration missing parameters at line %d", lineNum+1)
	}

	macroName := strings.TrimSpace(signature[10:parenStart])
	if macroName == "" {
		return nil, lineNum + 1, fmt.Errorf("pub macro declaration missing name at line %d", lineNum+1)
	}

	parenEnd := strings.LastIndex(signature, ")")
	if parenEnd == -1 || parenEnd <= parenStart {
		return nil, lineNum + 1, fmt.Errorf("pub macro declaration missing closing parenthesis at line %d", lineNum+1)
	}

	paramsStr := strings.TrimSpace(signature[parenStart+1 : parenEnd])
	var parameters []string
	if paramsStr != "" {
		paramList := splitRespectingQuotes(paramsStr)
		for _, param := range paramList {
			param = strings.TrimSpace(param)
			if param != "" {
				parameters = append(parameters, param)
			}
		}
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

	var body []string
	nextLine := lineNum + 1

	for nextLine < len(lines) {
		bodyLine := lines[nextLine]
		trimmed := strings.TrimSpace(bodyLine)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			nextLine++
			continue
		}

		indent := getIndentation(bodyLine)
		if indent < expectedBodyIndent {
			break
		}

		body = append(body, trimmed)
		nextLine++
	}

	if len(body) == 0 {
		return nil, lineNum + 1, fmt.Errorf("pub macro body cannot be empty at line %d", lineNum+1)
	}

	pubMacroStmt := &PubMacroDeclStmt{
		Name:       macroName,
		Parameters: parameters,
		Body:       body,
	}

	RegisteredMacros[macroName] = true

	return &Statement{PubMacroDecl: pubMacroStmt}, nextLine, nil
}

func parseEnumDeclaration(lines []string, startLine, indentLevel int) (*Statement, int, error) {
	var (
		line     = strings.TrimSpace(lines[startLine])
		isPublic = strings.HasPrefix(line, "pub ")
	)
	if isPublic {
		line = strings.TrimSpace(strings.TrimPrefix(line, "pub"))
	}
	parts := strings.Fields(line)
	if len(parts) < 2 || parts[0] != "enum" {
		return nil, startLine, fmt.Errorf("invalid enum declaration: %s", line)
	}
	enumName := strings.TrimSuffix(parts[1], ":")
	if enumName == "" {
		return nil, startLine, fmt.Errorf("missing enum name in declaration: %s", line)
	}
	var (
		values   = []string{}
		nextLine = startLine + 1
	)
	if strings.Contains(line, "{") && strings.Contains(line, "}") {
		valuesStr := line[strings.Index(line, "{")+1 : strings.Index(line, "}")]
		for val := range strings.SplitSeq(valuesStr, ",") {
			val = strings.TrimSpace(val)
			if val != "" {
				values = append(values, val)
			}
		}
	} else if nextLine < len(lines) {
		for nextLine < len(lines) {
			if strings.TrimSpace(lines[nextLine]) == "" {
				nextLine++
				continue
			}
			currentIndent := getIndentation(lines[nextLine])
			if currentIndent <= indentLevel {
				break
			}
			line := strings.TrimSpace(lines[nextLine])
			if line == "" || strings.HasPrefix(line, "#") {
				nextLine++
				continue
			}
			line = strings.TrimSuffix(line, ",")
			values = append(values, line)
			nextLine++
		}
	}
	if isPublic {
		return &Statement{
			PubEnumDecl: &PubEnumDeclStmt{
				Name:   enumName,
				Values: values,
			},
		}, nextLine, nil
	} else {
		return &Statement{
			EnumDecl: &EnumDeclStmt{
				IsPublic: false,
				Name:     enumName,
				Values:   values,
			},
		}, nextLine, nil
	}
}

func parseStatement(lines []string, lineNum, currentIndent int) (*Statement, int, error) {
	line := strings.TrimSpace(lines[lineNum])

	if !isStandardLibraryFile(CurrentSourceFile) {
		if hasReserved, reservedCmd := containsReserved(line); hasReserved {
			return nil, lineNum + 1, fmt.Errorf("unknown statement type '%s' at line %d", reservedCmd, lineNum+1)
		}
	}

	if strings.HasPrefix(line, "pub enum ") || strings.HasPrefix(line, "enum ") {
		return parseEnumDeclaration(lines, lineNum, currentIndent)
	}

	if strings.HasPrefix(line, "macro ") {
		return parseMacroDeclaration(lines, lineNum, currentIndent)
	}

	if strings.HasPrefix(line, "pub macro ") {
		return parsePubMacroDeclaration(lines, lineNum, currentIndent)
	}

	if strings.HasPrefix(line, "alias ") {
		return parseAliasDeclaration(line, lineNum)
	}

	if strings.HasPrefix(line, "new ") {
		return parseNewExprStatement(line, lineNum)
	}

	if strings.HasPrefix(line, "catlist!(") && strings.HasSuffix(line, ")") {
		var (
			argsStr = strings.TrimSpace(line[9 : len(line)-1])
			args    = splitRespectingQuotes(argsStr)
		)
		if len(args) < 2 {
			return nil, lineNum + 1, fmt.Errorf("catlist! statement requires at least 2 arguments at line %d", lineNum+1)
		}
		return &Statement{CatList: &CatListStmt{
			Target: "",
			Lists:  args,
		}}, lineNum + 1, nil
	}

	if strings.Contains(line, "=") && strings.Contains(line, "catlist!(") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			leftSide := strings.TrimSpace(parts[0])
			rightSide := strings.TrimSpace(parts[1])

			if strings.HasPrefix(rightSide, "catlist!(") && strings.HasSuffix(rightSide, ")") {
				argsStr := strings.TrimSpace(rightSide[9 : len(rightSide)-1])
				args := splitRespectingQuotes(argsStr)

				if len(args) < 2 {
					return nil, lineNum + 1, fmt.Errorf("catlist! assignment requires at least 2 list arguments at line %d", lineNum+1)
				}

				var targetVar string
				if strings.HasPrefix(leftSide, "var ") {
					targetVar = strings.TrimSpace(leftSide[4:])
				} else {
					targetVar = leftSide
				}

				return &Statement{CatList: &CatListStmt{
					Target: targetVar,
					Lists:  args,
				}}, lineNum + 1, nil
			}
		}
	}

	if strings.HasPrefix(line, "list_of!(") && strings.HasSuffix(line, ")") {
		argsStr := strings.TrimSpace(line[9 : len(line)-1]) // Remove "list_of!(" and ")"
		if argsStr == "" {
			return nil, lineNum + 1, fmt.Errorf("list_of! statement requires exactly 1 argument at line %d", lineNum+1)
		}
		return &Statement{ListOf: &ListOfStmt{
			Value: argsStr,
		}}, lineNum + 1, nil
	}

	if strings.Contains(line, "=") && strings.Contains(line, "list_of!(") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			leftSide := strings.TrimSpace(parts[0])
			rightSide := strings.TrimSpace(parts[1])

			if strings.HasPrefix(rightSide, "list_of!(") && strings.HasSuffix(rightSide, ")") {
				argsStr := strings.TrimSpace(rightSide[9 : len(rightSide)-1])

				var targetVar string
				var listType string

				if strings.HasPrefix(leftSide, "list[") {
					typeEnd := strings.Index(leftSide, "]")
					if typeEnd != -1 {
						listType = leftSide[5:typeEnd]
						parts := strings.Fields(leftSide)
						if len(parts) >= 2 {
							targetVar = parts[1]
						}
					}
				}

				if targetVar != "" && listType != "" {
					return &Statement{ListOfDecl: &ListOfDeclStmt{
						Type:  listType,
						Name:  targetVar,
						Value: argsStr,
					}}, lineNum + 1, nil
				}
			}
		}
	}

	if strings.HasPrefix(line, "map[") && strings.Contains(line, "]") && strings.Contains(line, "=") {
		// Find the end of the map type declaration, handling nested brackets
		typeEnd := findMapTypeEnd(line)
		if typeEnd == -1 {
			return nil, lineNum + 1, fmt.Errorf("invalid map type declaration at line %d", lineNum+1)
		}
		mapType := line[:typeEnd+1]
		if !strings.Contains(mapType, ":") {
			return nil, lineNum + 1, fmt.Errorf("map type must specify key:value types at line %d", lineNum+1)
		}
		var (
			typeStart = strings.Index(mapType, "[")
			typeDecl  = strings.TrimSpace(mapType[typeStart+1 : typeEnd])
		)

		// Parse complex map types using the new parsing logic
		colonPos := findMapColonPosition(typeDecl)
		if colonPos == -1 {
			return nil, lineNum + 1, fmt.Errorf("map type must specify key:value types at line %d", lineNum+1)
		}

		var (
			keyType   = strings.TrimSpace(typeDecl[:colonPos])
			valueType = strings.TrimSpace(typeDecl[colonPos+1:])
		)
		if keyType == "" || valueType == "" {
			return nil, lineNum + 1, fmt.Errorf("map type must specify valid key and value types at line %d", lineNum+1)
		}

		// Validate the complex types
		if _, valid := parseComplexType(keyType); !valid {
			return nil, lineNum + 1, fmt.Errorf("invalid key type '%s' at line %d", keyType, lineNum+1)
		}
		if _, valid := parseComplexType(valueType); !valid {
			return nil, lineNum + 1, fmt.Errorf("invalid value type '%s' at line %d", valueType, lineNum+1)
		}

		restOfLine := strings.TrimSpace(line[typeEnd+1:])
		parts := strings.Fields(restOfLine)
		if len(parts) < 3 || parts[1] != "=" {
			return nil, lineNum + 1, fmt.Errorf("map declaration format error at line %d (expected: map[keyType: valueType] name = [key: value, ...])", lineNum+1)
		}
		var (
			mapName    = parts[0]
			pairsStart = strings.Index(line, "=") + 1
			pairsEnd   = strings.LastIndex(line, "]")
		)
		if pairsStart == -1 || pairsEnd == -1 || pairsEnd <= pairsStart {
			return nil, lineNum + 1, fmt.Errorf("map declaration missing initialization at line %d", lineNum+1)
		}

		pairsStr := strings.TrimSpace(line[pairsStart:pairsEnd])
		var pairs []MapPair

		if pairsStr == "[]" {
			return &Statement{MapDecl: &MapDeclStmt{
				KeyType:   keyType,
				ValueType: valueType,
				Name:      mapName,
				Pairs:     pairs,
			}}, lineNum + 1, nil
		}

		if strings.HasPrefix(pairsStr, "[") {
			pairsStr = strings.TrimSpace(pairsStr[1:])
		}

		if pairsStr != "" {
			if strings.Contains(pairsStr, ":") {
				pairsList := splitMapPairs(pairsStr)
				for _, pairStr := range pairsList {
					pairStr = strings.TrimSpace(pairStr)
					if pairStr != "" {
						colonIdx := findPairColonPosition(pairStr)
						if colonIdx == -1 {
							return nil, lineNum + 1, fmt.Errorf("invalid map pair format at line %d", lineNum+1)
						}
						key := strings.TrimSpace(pairStr[:colonIdx])
						value := strings.TrimSpace(pairStr[colonIdx+1:])

						if strings.HasPrefix(key, "\"") && strings.HasSuffix(key, "\"") {
							key = key[1 : len(key)-1]
						}
						if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
							value = value[1 : len(value)-1]
						}

						pairs = append(pairs, MapPair{Key: key, Value: value})
					}
				}
			} else if strings.TrimSpace(pairsStr) != "" {
				valuesList := strings.Split(pairsStr, ",")
				for i, valueStr := range valuesList {
					valueStr = strings.TrimSpace(valueStr)
					if valueStr != "" {
						if strings.HasPrefix(valueStr, "\"") && strings.HasSuffix(valueStr, "\"") {
							valueStr = valueStr[1 : len(valueStr)-1]
						}
						pairs = append(pairs, MapPair{
							Key:   fmt.Sprintf("%d", i),
							Value: valueStr,
						})
					}
				}
			}
		}

		return &Statement{MapDecl: &MapDeclStmt{
			KeyType:   keyType,
			ValueType: valueType,
			Name:      mapName,
			Pairs:     pairs,
		}}, lineNum + 1, nil
	}

	parts := strings.Fields(strings.TrimSuffix(line, ":"))

	if strings.HasPrefix(parts[0], "list[") && strings.Contains(parts[0], "]") {
		if len(parts) < 4 || parts[2] != "=" {
			return nil, lineNum + 1, fmt.Errorf("list declaration format error at line %d (expected: list[type] name = [elements] or list[type] name = function_call())", lineNum+1)
		}

		// Find the end of the list type, handling nested brackets
		typeEnd := findListTypeEnd(parts[0])
		if typeEnd == -1 {
			return nil, lineNum + 1, fmt.Errorf("invalid list type declaration at line %d", lineNum+1)
		}

		// Extract the full list type including "list[...]"
		listType := parts[0][:typeEnd+1]

		// Validate the complex type
		if _, valid := parseComplexType(listType); !valid {
			return nil, lineNum + 1, fmt.Errorf("invalid list type '%s' at line %d", listType, lineNum+1)
		}

		listName := parts[1]
		value := strings.Join(parts[3:], " ")

		if strings.Contains(value, "(") && strings.Contains(value, ")") && !strings.HasPrefix(value, "[") {
			return &Statement{ListDeclFunctionCall: &ListDeclFunctionCallStmt{
				Type:         listType,
				Name:         listName,
				FunctionCall: value,
			}}, lineNum + 1, nil
		}

		// Handle list assignment from another variable (e.g., list[int] sorted_list = input_list)
		if !strings.HasPrefix(value, "[") && !strings.Contains(value, "(") && !strings.Contains(value, ")") {
			// This is a simple variable assignment, create a list declaration with reference to the source variable
			return &Statement{ListDecl: &ListDeclStmt{
				Type:     listType,
				Name:     listName,
				Elements: []string{value}, // Store the source variable name as the only element
			}}, lineNum + 1, nil
		}

		// Find the actual list elements after the "=" sign
		equalPos := strings.Index(line, "=")
		if equalPos == -1 {
			return nil, lineNum + 1, fmt.Errorf("list declaration missing '=' at line %d", lineNum+1)
		}

		// Find the opening bracket of the list elements
		elementsStart := strings.Index(line[equalPos:], "[")
		if elementsStart == -1 {
			return nil, lineNum + 1, fmt.Errorf("list declaration missing elements at line %d", lineNum+1)
		}
		elementsStart += equalPos // Adjust to absolute position

		// Find the closing bracket that matches the opening bracket
		elementsEnd := findMatchingClosingBracket(line, elementsStart)
		if elementsEnd == -1 {
			return nil, lineNum + 1, fmt.Errorf("list declaration missing closing bracket at line %d", lineNum+1)
		}

		elementsStr := strings.TrimSpace(line[elementsStart+1 : elementsEnd])
		var elements []string

		if elementsStr != "" {
			// Use bracket-aware parsing for complex nested elements
			elementsList := splitListElementsRespectingBrackets(elementsStr)
			for _, elem := range elementsList {
				elem = strings.TrimSpace(elem)
				if elem != "" {
					// Don't strip quotes from complex elements like ["a", "b"]
					if strings.HasPrefix(elem, "\"") && strings.HasSuffix(elem, "\"") && !strings.Contains(elem, "[") {
						elem = elem[1 : len(elem)-1]
					}
					elements = append(elements, elem)
				}
			}
		}

		return &Statement{ListDecl: &ListDeclStmt{Type: listType, Name: listName, Elements: elements}}, lineNum + 1, nil
	}

	if len(parts) == 0 {
		return nil, lineNum + 1, fmt.Errorf("empty statement at line %d", lineNum+1)
	}

	if strings.HasPrefix(line, "cat!(") && strings.HasSuffix(line, ")") {
		argsStr := strings.TrimSpace(line[5 : len(line)-1])
		args := splitRespectingQuotes(argsStr)
		if len(args) != 2 {
			return nil, lineNum + 1, fmt.Errorf("cat! statement requires exactly 2 arguments at line %d", lineNum+1)
		}
		var (
			target = strings.TrimSpace(args[0])
			value  = strings.TrimSpace(args[1])
		)
		return &Statement{CatString: &CatStringStmt{
			Target: target,
			Value:  value,
		}}, lineNum + 1, nil
	} else if strings.HasPrefix(line, "put!(") && strings.HasSuffix(line, ")") {
		argsStr := strings.TrimSpace(line[5 : len(line)-1])
		args := splitRespectingQuotes(argsStr)
		if len(args) != 3 {
			return nil, lineNum + 1, fmt.Errorf("put! statement requires exactly 3 arguments at line %d", lineNum+1)
		}
		var (
			mapName = strings.TrimSpace(args[0])
			key     = strings.TrimSpace(args[1])
			value   = strings.TrimSpace(args[2])
		)
		return &Statement{PutMap: &PutMapStmt{
			MapName: mapName,
			Key:     key,
			Value:   value,
		}}, lineNum + 1, nil
	} else if strings.HasPrefix(line, "get!(") && strings.HasSuffix(line, ")") {
		argsStr := strings.TrimSpace(line[5 : len(line)-1])
		args := splitRespectingQuotes(argsStr)
		if len(args) != 2 {
			return nil, lineNum + 1, fmt.Errorf("get! statement requires exactly 2 arguments at line %d (mapName, key)", lineNum+1)
		}
		var (
			mapName = strings.TrimSpace(args[0])
			key     = strings.TrimSpace(args[1])
		)
		return &Statement{GetMap: &GetMapStmt{
			MapName: mapName,
			Key:     key,
		}}, lineNum + 1, nil
	}
	switch parts[0] {
	case "u16", "u32", "u64", "i16", "i32", "i64", "f32", "f64":
		if len(parts) < 4 || parts[2] != "=" {
			return nil, lineNum + 1, fmt.Errorf("numeric type declaration format error at line %d (expected: %s name = value)", lineNum+1, parts[0])
		}
		varType := parts[0]
		varName := parts[1]
		value := strings.Join(parts[3:], " ")

		return &Statement{VarDecl: &VarDeclStmt{
			Type:  varType,
			Name:  varName,
			Value: value,
			IsRef: false,
		}}, lineNum + 1, nil

	case "parallel":
		if len(parts) >= 5 && parts[1] == "for" && parts[3] == "=" && strings.Contains(line, "to") && strings.HasSuffix(line, ":") {
			var (
				equalsIndex   = strings.Index(line, "=")
				toIndex       = strings.Index(line, "to")
				colonIndex    = strings.LastIndex(line, ":")
				stepIndex     = strings.Index(line, "step")
				stepClauseEnd int
			)
			if stepIndex != -1 && stepIndex > toIndex && stepIndex < colonIndex {
				stepClauseEnd = stepIndex
			} else {
				stepClauseEnd = colonIndex
			}

			reduceIndex := strings.Index(line, "reduce(")
			var reduceClauseStart int
			if reduceIndex != -1 && reduceIndex < colonIndex {
				reduceClauseStart = reduceIndex
				if stepClauseEnd == stepIndex {
					// If we have both step and reduce, step ends where reduce starts
					stepClauseEnd = reduceIndex
				}
			} else {
				reduceClauseStart = colonIndex
			}

			if equalsIndex == -1 || toIndex == -1 || colonIndex == -1 ||
				!(equalsIndex > strings.Index(line, "for") && equalsIndex < toIndex && toIndex < stepClauseEnd) {
				return nil, lineNum + 1, fmt.Errorf("parallel for statement format error at line %d", lineNum+1)
			}

			var (
				varName    = strings.TrimSpace(line[strings.Index(line, "for")+len("for") : equalsIndex])
				start      = strings.TrimSpace(line[equalsIndex+1 : toIndex])
				end        string
				step       string
				reductions []*ReductionClause
			)

			// Parse end value (up to step or reduce clause)
			if stepIndex != -1 && stepIndex > toIndex && stepIndex < reduceClauseStart {
				end = strings.TrimSpace(line[toIndex+len("to") : stepIndex])
				// Parse step value
				stepEnd := reduceClauseStart
				step = strings.TrimSpace(line[stepIndex+len("step") : stepEnd])
			} else {
				end = strings.TrimSpace(line[toIndex+len("to") : reduceClauseStart])
			}

			if reduceIndex != -1 {
				reduceEnd := strings.Index(line[reduceIndex:], ")")
				if reduceEnd == -1 {
					return nil, lineNum + 1, fmt.Errorf("parallel for statement missing closing ')' for reduce clause at line %d", lineNum+1)
				}
				reduceContent := strings.TrimSpace(line[reduceIndex+len("reduce(") : reduceIndex+reduceEnd])
				clauses := strings.SplitSeq(reduceContent, ",")
				for clause := range clauses {
					clause = strings.TrimSpace(clause)
					parts := strings.Split(clause, ":")
					if len(parts) != 2 {
						return nil, lineNum + 1, fmt.Errorf("parallel for statement invalid reduce clause format at line %d (expected 'operation: variable')", lineNum+1)
					}
					operation := strings.TrimSpace(parts[0])
					variable := strings.TrimSpace(parts[1])
					reductions = append(reductions, &ReductionClause{Operation: operation, Variable: variable})
				}
			}

			if varName == "" || start == "" || end == "" {
				return nil, lineNum + 1, fmt.Errorf("parallel for statement missing variable, start, or end expression at line %d", lineNum+1)
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

			body, err := parseStatements(lines, lineNum+1, expectedBodyIndent)
			if err != nil {
				return nil, lineNum + 1, err
			}

			nextLine := findEndOfBlock(lines, lineNum+1, expectedBodyIndent)

			return &Statement{ParallelFor: &ParallelForStmt{
				Var:        varName,
				Start:      start,
				End:        end,
				Step:       step,
				Reductions: reductions,
				Body:       body,
			}}, nextLine, nil
		} else if len(parts) >= 3 && parts[1] == "while" && strings.HasSuffix(line, ":") {
			whileIndex := strings.Index(line, "while")
			colonIndex := strings.LastIndex(line, ":")
			if whileIndex == -1 || colonIndex == -1 || whileIndex >= colonIndex {
				return nil, lineNum + 1, fmt.Errorf("parallel while statement format error at line %d", lineNum+1)
			}

			condition := strings.TrimSpace(line[whileIndex+len("while") : colonIndex])
			if condition == "" {
				return nil, lineNum + 1, fmt.Errorf("parallel while statement missing condition at line %d", lineNum+1)
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

			body, err := parseStatements(lines, lineNum+1, expectedBodyIndent)
			if err != nil {
				return nil, lineNum + 1, err
			}

			nextLine := findEndOfBlock(lines, lineNum+1, expectedBodyIndent)

			return &Statement{ParallelWhile: &ParallelWhileStmt{Condition: condition, Body: body}}, nextLine, nil
		} else if strings.HasSuffix(line, ":") {
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

			return &Statement{ParallelBlock: &ParallelBlockStmt{Body: body}}, nextLine, nil
		} else {
			return nil, lineNum + 1, fmt.Errorf("parallel statement format error at line %d (expected: 'parallel:' for block, 'parallel for var = start to end:' for loop, or 'parallel while condition:' for while loop)", lineNum+1)
		}

	case "import":
		if len(parts) < 2 {
			return nil, lineNum + 1, fmt.Errorf("import statement requires a module name at line %d", lineNum+1)
		}

		if strings.Contains(line, ",") {
			importLine := strings.TrimSpace(line[6:])
			moduleNames := strings.Split(importLine, ",")

			var imports []*ImportStmt
			for _, moduleName := range moduleNames {
				moduleName = strings.TrimSpace(strings.Trim(moduleName, "\""))
				if moduleName != "" {
					imports = append(imports, &ImportStmt{Module: moduleName})
				}
			}
			if len(imports) > 0 {
				return &Statement{Import: imports[0]}, lineNum + 1, nil
			}
		} else {
			nextLine := lineNum + 1
			if nextLine < len(lines) {
				nextTrimmed := strings.TrimSpace(lines[nextLine])
				if nextTrimmed != "" && getIndentation(lines[nextLine]) > 0 &&
					!strings.HasPrefix(nextTrimmed, "import ") && nextTrimmed != "import" {
					return parseBulkImport(lines, lineNum)
				}
			}
			moduleName := strings.Trim(strings.Join(parts[1:], " "), "\"")
			return &Statement{Import: &ImportStmt{Module: moduleName}}, lineNum + 1, nil
		}
	case "external":
		if len(parts) >= 3 && parts[1] == "import" {
			if len(parts) < 3 {
				return nil, lineNum + 1, fmt.Errorf("external import statement requires a header name at line %d", lineNum+1)
			}
			headerName := strings.Trim(strings.Join(parts[2:], " "), "\"")
			return &Statement{ExternalImport: &ExternalImportStmt{Header: headerName}}, lineNum + 1, nil
		}
		return nil, lineNum + 1, fmt.Errorf("unknown external statement at line %d", lineNum+1)
	case "local":
		if len(parts) >= 3 && parts[1] == "import" {
			if len(parts) < 3 {
				return nil, lineNum + 1, fmt.Errorf("local import statement requires a header name at line %d", lineNum+1)
			}
			headerName := strings.Trim(strings.Join(parts[2:], " "), "\"")
			return &Statement{LocalImport: &LocalImportStmt{Header: headerName}}, lineNum + 1, nil
		}
		return nil, lineNum + 1, fmt.Errorf("unknown local statement at line %d", lineNum+1)
	case "ref":
		if len(parts) < 5 || parts[3] != "=" {
			return nil, lineNum + 1, fmt.Errorf("ref declaration format error at line %d (expected: ref type name = value)", lineNum+1)
		}

		varType := parts[1]
		varName := parts[2]
		value := strings.Join(parts[4:], " ")

		// Handle ref declarations for class fields
		if strings.HasPrefix(varName, "this.") {
			return &Statement{VarDecl: &VarDeclStmt{
				Type:  varType,
				Name:  varName,
				Value: value,
				IsRef: true,
			}}, lineNum + 1, nil
		}

		// Handle local ref declarations
		return &Statement{VarDecl: &VarDeclStmt{
			Type:  varType,
			Name:  varName,
			Value: value,
			IsRef: true,
		}}, lineNum + 1, nil

	case "var":
		if len(parts) < 4 || parts[2] != "=" {
			return nil, lineNum + 1, fmt.Errorf("var declaration format error at line %d (expected: var name = value)", lineNum+1)
		}

		isRef := false
		varName := parts[1]
		value := strings.Join(parts[3:], " ")
		varType := ""

		if parts[1] == "ref" && len(parts) >= 5 {
			isRef = true
			varType = parts[2]
			varName = parts[3]
			if parts[4] == "=" {
				value = strings.Join(parts[5:], " ")
			}
		}

		if strings.HasPrefix(varName, "this.") && isRef {
			return &Statement{VarDecl: &VarDeclStmt{
				Type:  varType,
				Name:  varName,
				Value: value,
				IsRef: true,
			}}, lineNum + 1, nil
		}
		if strings.HasPrefix(value, "new ") {
			newPart := strings.TrimSpace(value[4:]) // Remove "new "
			parenStart := strings.Index(newPart, "(")
			if parenStart == -1 {
				return nil, lineNum + 1, fmt.Errorf("object declaration missing parentheses at line %d", lineNum+1)
			}

			className := strings.TrimSpace(newPart[:parenStart])

			var constructorArgs []string
			argsStart := strings.Index(value, "(")
			argsEnd := strings.LastIndex(value, ")")
			if argsStart != -1 && argsEnd != -1 && argsEnd > argsStart+1 {
				constructorArgsStr := strings.TrimSpace(value[argsStart+1 : argsEnd])
				if constructorArgsStr != "" {
					constructorArgsList := strings.Split(constructorArgsStr, ",")
					for _, arg := range constructorArgsList {
						constructorArgs = append(constructorArgs, strings.TrimSpace(arg))
					}
				}
			}

			// Handle module-qualified types
			typeName := className
			var args []string
			if strings.Contains(className, ".") {
				parts := strings.Split(className, ".")
				if len(parts) == 2 {
					args = append(args, parts[0]) // module name
					args = append(args, parts[1]) // class name
					typeName = className          // Keep full qualified name as type
				} else {
					return nil, lineNum + 1, fmt.Errorf("invalid module-qualified class name at line %d", lineNum+1)
				}
			} else {
				args = append(args, className)
			}
			args = append(args, constructorArgs...)

			return &Statement{ObjectDecl: &ObjectDeclStmt{Type: typeName, Name: varName, Args: args}}, lineNum + 1, nil
		}

		if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
			// Maybe do something here later, idk.
		} else if strings.Contains(value, " ") && !strings.Contains(value, "\"") && !strings.HasPrefix(value, "new ") {
			value = fmt.Sprintf("\"%s\"", value)
		}

		return &Statement{VarDeclInferred: &VarDeclInferredStmt{Name: varName, Value: value}}, lineNum + 1, nil

	case "val":
		if len(parts) < 4 {
			return nil, lineNum + 1, fmt.Errorf("val declaration format error at line %d (expected: val name = value or val type name = value)", lineNum+1)
		}

		if len(parts) >= 5 && parts[3] == "=" {
			varType := parts[1]
			varName := parts[2]
			value := strings.Join(parts[4:], " ")

			return &Statement{VarDecl: &VarDeclStmt{
				Type:    varType,
				Name:    varName,
				Value:   value,
				IsConst: true,
			}}, lineNum + 1, nil
		}

		if len(parts) >= 6 && parts[1] == "ref" && parts[4] == "=" {
			varType := parts[2]
			varName := parts[3]
			value := strings.Join(parts[5:], " ")
			return &Statement{VarDecl: &VarDeclStmt{
				Type:    varType,
				Name:    varName,
				Value:   value,
				IsRef:   true,
				IsConst: true,
			}}, lineNum + 1, nil
		}

		if parts[2] != "=" {
			return nil, lineNum + 1, fmt.Errorf("val declaration format error at line %d (expected: val name = value)", lineNum+1)
		}

		isRef := false
		varName := parts[1]
		value := strings.Join(parts[3:], " ")
		varType := ""

		if parts[1] == "ref" && len(parts) >= 5 {
			isRef = true
			varType = parts[2]
			varName = parts[3]
			if parts[4] == "=" {
				value = strings.Join(parts[5:], " ")
			}
		}

		if strings.HasPrefix(varName, "this.") && isRef {
			return &Statement{VarDecl: &VarDeclStmt{
				Type:    varType,
				Name:    varName,
				Value:   value,
				IsRef:   true,
				IsConst: true,
			}}, lineNum + 1, nil
		}
		if strings.HasPrefix(value, "new ") {
			newPart := strings.TrimSpace(value[4:]) // Remove "new "
			parenStart := strings.Index(newPart, "(")
			if parenStart == -1 {
				return nil, lineNum + 1, fmt.Errorf("object declaration missing parentheses at line %d", lineNum+1)
			}

			className := strings.TrimSpace(newPart[:parenStart])

			var constructorArgs []string
			argsStart := strings.Index(value, "(")
			argsEnd := strings.LastIndex(value, ")")
			if argsStart != -1 && argsEnd != -1 && argsEnd > argsStart+1 {
				constructorArgsStr := strings.TrimSpace(value[argsStart+1 : argsEnd])
				if constructorArgsStr != "" {
					constructorArgsList := strings.SplitSeq(constructorArgsStr, ",")
					for arg := range constructorArgsList {
						constructorArgs = append(constructorArgs, strings.TrimSpace(arg))
					}
				}
			}

			typeName := className
			var args []string
			if strings.Contains(className, ".") {
				parts := strings.Split(className, ".")
				if len(parts) == 2 {
					args = append(args, parts[0])
					args = append(args, parts[1])
					typeName = className
				} else {
					return nil, lineNum + 1, fmt.Errorf("invalid module-qualified class name at line %d", lineNum+1)
				}
			} else {
				args = append(args, className)
			}
			args = append(args, constructorArgs...)

			return &Statement{ObjectDecl: &ObjectDeclStmt{Type: typeName, Name: varName, Args: args}}, lineNum + 1, nil
		}

		if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
		} else if strings.Contains(value, " ") && !strings.Contains(value, "\"") && !strings.HasPrefix(value, "new ") {
			value = fmt.Sprintf("\"%s\"", value)
		}

		// For inferred 'val', mark as const so codegen can emit C 'const'.
		return &Statement{VarDeclInferred: &VarDeclInferredStmt{Name: varName, Value: value, IsConst: true}}, lineNum + 1, nil

	case "pub":
		return parsePubStatement(lines, lineNum, currentIndent)

	case "return":
		value := ""
		if len(parts) >= 2 {
			value = strings.Join(parts[1:], " ")
			if strings.HasPrefix(value, "this.") {
				fieldName := value[5:]
				value = "this->" + fieldName
			}
		}
		return &Statement{Return: &ReturnStmt{Value: value}}, lineNum + 1, nil

	case "class":
		return parseClassStatement(lines, lineNum, currentIndent)

	case "struct":
		return parseStructStatement(lines, lineNum, currentIndent)

	case "fn":
		return parseTopLevelFunctionStatement(lines, lineNum, currentIndent)

	case "print":
		if len(parts) < 2 {
			return nil, lineNum + 1, fmt.Errorf("print statement requires a string at line %d", lineNum+1)
		}

		if strings.Contains(line, "|") {
			var (
				pipeIndex  = strings.Index(line, "|")
				formatPart = strings.TrimSpace(line[5:pipeIndex])
				varPart    = strings.TrimSpace(line[pipeIndex+1:])
			)
			if strings.HasPrefix(formatPart, "\"") && strings.HasSuffix(formatPart, "\"") {
				formatPart = formatPart[1 : len(formatPart)-1]
			}

			var variables []string
			if varPart != "" {
				// Use smart comma splitting that respects parentheses
				varList := splitRespectingParens(varPart)
				for _, v := range varList {
					variables = append(variables, strings.TrimSpace(v))
				}
			}

			return &Statement{Print: &PrintStmt{Format: formatPart, Variables: variables}}, lineNum + 1, nil
		} else if strings.Contains(line, ",") && strings.Contains(line, "\"") {
			quoteStart := strings.Index(line, "\"")
			quoteEnd := strings.LastIndex(line, "\"")
			if quoteStart != -1 && quoteEnd != -1 && quoteEnd > quoteStart {
				afterQuote := strings.TrimSpace(line[quoteEnd+1:])
				if strings.HasPrefix(afterQuote, ",") {
					formatPart := strings.TrimSpace(line[quoteStart+1 : quoteEnd])
					varPart := strings.TrimSpace(line[quoteEnd+1:])

					var variables []string
					if varPart != "" && strings.HasPrefix(varPart, ",") {
						varPart = strings.TrimSpace(varPart[1:])
						// Use smart comma splitting that respects parentheses
						varList := splitRespectingParens(varPart)
						for _, v := range varList {
							variables = append(variables, strings.TrimSpace(v))
						}
					}

					return &Statement{Print: &PrintStmt{Format: formatPart, Variables: variables}}, lineNum + 1, nil
				}
			}
		}

		str := strings.TrimSpace(line[5:])
		if strings.HasPrefix(str, "\"") && strings.HasSuffix(str, "\"") {
			str = str[1 : len(str)-1]
			return &Statement{Print: &PrintStmt{Print: str}}, lineNum + 1, nil
		} else {
			return nil, lineNum + 1, fmt.Errorf("print statement requires a quoted string at line %d", lineNum+1)
		}

	case "sleep":
		if len(parts) < 2 {
			return nil, lineNum + 1, fmt.Errorf("sleep statement requires a number at line %d", lineNum+1)
		}
		return &Statement{Sleep: &SleepStmt{Duration: parts[1]}}, lineNum + 1, nil

	case "break":
		return &Statement{Break: &BreakStmt{Break: "break"}}, lineNum + 1, nil

	case "pass":
		return &Statement{Pass: &PassStmt{Pass: "pass"}}, lineNum + 1, nil

	case "continue":
		return &Statement{Continue: &ContinueStmt{Continue: "continue"}}, lineNum + 1, nil

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
			return nil, lineNum + 1, fmt.Errorf("allocate statement format error at line %d (expected: allocate [ref] type name = size)", lineNum+1)
		}

		if len(parts) >= 5 && parts[1] == "ref" {
			// allocate ref type name = size
			if equalIndex != 4 {
				return nil, lineNum + 1, fmt.Errorf("allocate statement format error at line %d (expected: allocate ref type name = size)", lineNum+1)
			}
			varType = "ref " + parts[2]
			varName = parts[3]
		} else {
			// allocate type name = size
			if equalIndex != 3 {
				return nil, lineNum + 1, fmt.Errorf("allocate statement format error at line %d (expected: allocate type name = size)", lineNum+1)
			}
			varType = parts[1]
			varName = parts[2]
		}

		size = strings.Join(parts[equalIndex+1:], " ")
		return &Statement{Allocate: &AllocateStmt{Type: varType, Name: varName, Size: size}}, lineNum + 1, nil

	case "stallocate":
		var varType, varName, size string
		var equalIndex int

		for i, part := range parts {
			if part == "=" {
				equalIndex = i
				break
			}
		}

		if equalIndex == 0 || equalIndex >= len(parts)-1 {
			return nil, lineNum + 1, fmt.Errorf("stallocate statement format error at line %d (expected: stallocate [ref] type name = size)", lineNum+1)
		}

		// Check if we have "ref" modifier
		if len(parts) >= 5 && parts[1] == "ref" {
			// stallocate ref type name = size
			if equalIndex != 4 {
				return nil, lineNum + 1, fmt.Errorf("stallocate statement format error at line %d (expected: stallocate ref type name = size)", lineNum+1)
			}
			varType = "ref " + parts[2]
			varName = parts[3]
		} else {
			// stallocate type name = size
			if equalIndex != 3 {
				return nil, lineNum + 1, fmt.Errorf("stallocate statement format error at line %d (expected: stallocate type name = size)", lineNum+1)
			}
			varType = parts[1]
			varName = parts[2]
		}

		size = strings.Join(parts[equalIndex+1:], " ")
		return &Statement{StackAllocate: &StackAllocateStmt{Type: varType, Name: varName, Size: size}}, lineNum + 1, nil

	case "free":
		if len(parts) < 2 {
			return nil, lineNum + 1, fmt.Errorf("free statement requires a variable at line %d", lineNum+1)
		}
		variable := parts[1]
		return &Statement{Free: &FreeStmt{Variable: variable}}, lineNum + 1, nil

	case "run":
		if len(parts) < 2 {
			return nil, lineNum + 1, fmt.Errorf("run statement requires a function call at line %d", lineNum+1)
		}
		funcCall := strings.TrimSpace(line[3:])
		if !strings.Contains(funcCall, "(") || !strings.HasSuffix(funcCall, ")") {
			return nil, lineNum + 1, fmt.Errorf("run statement requires a function call with parentheses at line %d", lineNum+1)
		}
		return &Statement{Run: &RunStmt{FunctionCall: funcCall}}, lineNum + 1, nil

	case "while":
		if len(parts) < 2 || !strings.HasSuffix(line, ":") {
			return nil, lineNum + 1, fmt.Errorf("while statement format error at line %d", lineNum+1)
		}
		var (
			colonIndex    = strings.LastIndex(line, ":")
			conditionPart = strings.TrimSpace(line[5:colonIndex])
			condition     = conditionPart
		)
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

		return &Statement{While: &WhileStmt{Condition: condition, Body: body}}, nextLine, nil

	case "foreach":
		if !strings.HasSuffix(line, ":") {
			return nil, lineNum + 1, fmt.Errorf("foreach statement must end with ':' at line %d", lineNum+1)
		}
		content := strings.TrimSpace(line[7 : len(line)-1])
		if !strings.HasPrefix(content, "(") || !strings.HasSuffix(content, ")") {
			return nil, lineNum + 1, fmt.Errorf("foreach statement format error at line %d (expected: foreach (type var in collection):)", lineNum+1)
		}

		content = content[1 : len(content)-1] // Remove parentheses
		inIndex := strings.Index(content, " in ")
		if inIndex == -1 {
			return nil, lineNum + 1, fmt.Errorf("foreach statement missing 'in' keyword at line %d", lineNum+1)
		}

		varPart := strings.TrimSpace(content[:inIndex])
		collection := strings.TrimSpace(content[inIndex+4:])
		varParts := strings.Fields(varPart)
		if len(varParts) != 2 {
			return nil, lineNum + 1, fmt.Errorf("foreach statement variable format error at line %d (expected: type varname)", lineNum+1)
		}

		varType := varParts[0]
		varName := varParts[1]

		// Check if it's a valid collection type: map.keys, map.values, or string variable
		isMapCollection := strings.HasSuffix(collection, ".keys") || strings.HasSuffix(collection, ".values")
		isStringCollection := !isMapCollection // For now, assume any other identifier is a string variable

		if !isMapCollection && !isStringCollection {
			return nil, lineNum + 1, fmt.Errorf("foreach statement collection must be 'mapname.keys', 'mapname.values', or a string variable at line %d", lineNum+1)
		}

		// For string iteration, ensure the variable type is char
		if isStringCollection && varType != "char" {
			return nil, lineNum + 1, fmt.Errorf("foreach statement over string must use 'char' variable type at line %d", lineNum+1)
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

		body, err := parseStatements(lines, lineNum+1, expectedBodyIndent)
		if err != nil {
			return nil, lineNum + 1, err
		}

		nextLine := findEndOfBlock(lines, lineNum+1, expectedBodyIndent)

		return &Statement{Foreach: &ForeachStmt{
			VarType:    varType,
			VarName:    varName,
			Collection: collection,
			Body:       body,
		}}, nextLine, nil

	case "for":
		semicolonCount := strings.Count(line, ";")
		if semicolonCount == 2 {
			colonIndex := strings.LastIndex(line, ":")
			if colonIndex == -1 {
				return nil, lineNum + 1, fmt.Errorf("for statement missing colon at line %d", lineNum+1)
			}

			forContent := strings.TrimSpace(line[3:colonIndex]) // Remove "for" and ":"
			parts := strings.Split(forContent, ";")
			if len(parts) != 3 {
				return nil, lineNum + 1, fmt.Errorf("for statement format error at line %d", lineNum+1)
			}

			initPart := strings.TrimSpace(parts[0])
			condition := strings.TrimSpace(parts[1])
			increment := strings.TrimSpace(parts[2])
			initParts := strings.Fields(initPart)
			if len(initParts) < 4 || initParts[2] != "=" {
				return nil, lineNum + 1, fmt.Errorf("for statement init format error at line %d", lineNum+1)
			}

			varType := initParts[0]
			varName := initParts[1]
			init := strings.Join(initParts[3:], " ")

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

			return &Statement{VerboseFor: &VerboseForStmt{
				VarType:   varType,
				VarName:   varName,
				Init:      init,
				Condition: condition,
				Increment: increment,
				Body:      body,
			}}, nextLine, nil
		}

		equalsIndex := strings.Index(line, "=")
		toIndex := strings.Index(line, "to")
		colonIndex := strings.LastIndex(line, ":")
		if equalsIndex == -1 || toIndex == -1 || colonIndex == -1 ||
			!(equalsIndex > strings.Index(line, "for") && equalsIndex < toIndex && toIndex < colonIndex) {
			return nil, lineNum + 1, fmt.Errorf("for statement format error at line %d", lineNum+1)
		}

		varName := strings.TrimSpace(line[strings.Index(line, "for")+len("for") : equalsIndex])
		start := strings.TrimSpace(line[equalsIndex+1 : toIndex])
		end := strings.TrimSpace(line[toIndex+len("to") : colonIndex])

		if varName == "" || start == "" || end == "" {
			return nil, lineNum + 1, fmt.Errorf("for statement missing variable, start, or end expression at line %d", lineNum+1)
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

		body, err := parseStatements(lines, lineNum+1, expectedBodyIndent)
		if err != nil {
			return nil, lineNum + 1, err
		}

		nextLine := findEndOfBlock(lines, lineNum+1, expectedBodyIndent)

		return &Statement{For: &ForStmt{Var: varName, Start: start, End: end, Body: body}}, nextLine, nil

	case "reverse":
		if len(parts) < 2 || parts[1] != "for" {
			return nil, lineNum + 1, fmt.Errorf("reverse statement must be followed by 'for' at line %d", lineNum+1)
		}

		// Parse reverse for: reverse for i = 10 to 50:
		equalsIndex := strings.Index(line, "=")
		toIndex := strings.Index(line, "to")
		colonIndex := strings.LastIndex(line, ":")
		if equalsIndex == -1 || toIndex == -1 || colonIndex == -1 ||
			!(equalsIndex > strings.Index(line, "for") && equalsIndex < toIndex && toIndex < colonIndex) {
			return nil, lineNum + 1, fmt.Errorf("reverse for statement format error at line %d", lineNum+1)
		}

		varName := strings.TrimSpace(line[strings.Index(line, "for")+len("for") : equalsIndex])
		start := strings.TrimSpace(line[equalsIndex+1 : toIndex])
		end := strings.TrimSpace(line[toIndex+len("to") : colonIndex])

		if varName == "" || start == "" || end == "" {
			return nil, lineNum + 1, fmt.Errorf("reverse for statement missing variable, start, or end expression at line %d", lineNum+1)
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

		body, err := parseStatements(lines, lineNum+1, expectedBodyIndent)
		if err != nil {
			return nil, lineNum + 1, err
		}

		nextLine := findEndOfBlock(lines, lineNum+1, expectedBodyIndent)

		return &Statement{ReverseFor: &ReverseForStmt{Var: varName, Start: start, End: end, Body: body}}, nextLine, nil

	case "if":
		if len(parts) < 2 || !strings.HasSuffix(line, ":") {
			return nil, lineNum + 1, fmt.Errorf("if statement format error at line %d", lineNum+1)
		}
		var (
			colonIndex         = strings.LastIndex(line, ":")
			conditionPart      = strings.TrimSpace(line[2:colonIndex])
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

		var elseIfs []*ElifStmt
		for nextLine < len(lines) {
			nextTrimmed := strings.TrimSpace(lines[nextLine])
			if nextTrimmed == "" || strings.HasPrefix(nextTrimmed, "#") {
				nextLine++
				continue
			}

			nextIndent := getIndentation(lines[nextLine])
			if nextIndent != currentIndent {
				break
			}

			if !strings.HasPrefix(nextTrimmed, "elif ") {
				break
			}

			elifStmt, newNextLine, err := parseElifStatement(lines, nextLine, currentIndent)
			if err != nil {
				return nil, nextLine, err
			}

			elseIfs = append(elseIfs, elifStmt)
			nextLine = newNextLine
		}

		var elseStmt *ElseStmt
		if nextLine < len(lines) {
			nextTrimmed := strings.TrimSpace(lines[nextLine])
			if nextTrimmed != "" && !strings.HasPrefix(nextTrimmed, "#") {
				nextIndent := getIndentation(lines[nextLine])
				if nextIndent == currentIndent && strings.HasPrefix(nextTrimmed, "else:") {
					var err error
					elseStmt, nextLine, err = parseElseStatement(lines, nextLine, currentIndent)
					if err != nil {
						return nil, nextLine, err
					}
				}
			}
		}

		return &Statement{If: &IfStmt{Condition: condition, Body: body, ElseIfs: elseIfs, Else: elseStmt}}, nextLine, nil

	case "platform":
		open := strings.Index(line, "(")
		close := strings.LastIndex(line, ")")
		if open == -1 || close == -1 || close < open || !strings.HasSuffix(strings.TrimSpace(line), ":") {
			return nil, lineNum + 1, fmt.Errorf("platform statement format error at line %d", lineNum+1)
		}
		platformName := strings.TrimSpace(line[open+1 : close])
		if platformName == "" {
			return nil, lineNum + 1, fmt.Errorf("platform name missing at line %d", lineNum+1)
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

		body, err := parseStatements(lines, lineNum+1, expectedBodyIndent)
		if err != nil {
			return nil, lineNum + 1, err
		}
		nextLine := findEndOfBlock(lines, lineNum+1, expectedBodyIndent)
		return &Statement{Platform: &PlatformStmt{Platform: platformName, Body: body}}, nextLine, nil

	case "put":
		if len(parts) < 2 {
			return nil, lineNum + 1, fmt.Errorf("put statement requires a string at line %d", lineNum+1)
		}

		if strings.Contains(line, "|") {
			var (
				pipeIndex  = strings.Index(line, "|")
				formatPart = strings.TrimSpace(line[3:pipeIndex])
				varPart    = strings.TrimSpace(line[pipeIndex+1:])
			)
			if strings.HasPrefix(formatPart, "\"") && strings.HasSuffix(formatPart, "\"") {
				formatPart = formatPart[1 : len(formatPart)-1]
			}

			var variables []string
			if varPart != "" {
				varList := splitRespectingParens(varPart)
				for _, v := range varList {
					variables = append(variables, strings.TrimSpace(v))
				}
			}

			return &Statement{Put: &PutStmt{Format: formatPart, Variables: variables}}, lineNum + 1, nil
		} else if strings.Contains(line, ",") && strings.Contains(line, "\"") {
			quoteStart := strings.Index(line, "\"")
			quoteEnd := strings.LastIndex(line, "\"")
			if quoteStart != -1 && quoteEnd != -1 && quoteEnd > quoteStart {
				afterQuote := strings.TrimSpace(line[quoteEnd+1:])
				if strings.HasPrefix(afterQuote, ",") {
					formatPart := strings.TrimSpace(line[quoteStart+1 : quoteEnd])
					varPart := strings.TrimSpace(line[quoteEnd+1:])

					var variables []string
					if varPart != "" && strings.HasPrefix(varPart, ",") {
						varPart = strings.TrimSpace(varPart[1:])
						varList := splitRespectingParens(varPart)
						for _, v := range varList {
							variables = append(variables, strings.TrimSpace(v))
						}
					}

					return &Statement{Put: &PutStmt{Format: formatPart, Variables: variables}}, lineNum + 1, nil
				}
			}
		}

		str := strings.TrimSpace(line[3:])
		if strings.HasPrefix(str, "\"") && strings.HasSuffix(str, "\"") {
			str = str[1 : len(str)-1]
		}
		return &Statement{Put: &PutStmt{Put: str}}, lineNum + 1, nil
	case "try":
		if !strings.HasSuffix(line, ":") {
			return nil, lineNum + 1, fmt.Errorf("try statement must end with ':' at line %d", lineNum+1)
		}
		return parseTryCatchStatement(lines, lineNum, currentIndent)
	case "throw":
		if len(parts) < 2 {
			return nil, lineNum + 1, fmt.Errorf("throw statement requires a value at line %d", lineNum+1)
		}
		value := strings.Join(parts[1:], " ")
		return &Statement{Throw: &ThrowStmt{Value: value}}, lineNum + 1, nil

	// Handle standard assignment (var = expr)
	case "elif":
		return nil, lineNum + 1, fmt.Errorf("elif statement must follow an if statement at line %d", lineNum+1)

	case "else":
		return nil, lineNum + 1, fmt.Errorf("else statement must follow an if statement at line %d", lineNum+1)

	case "catch":
		return nil, lineNum + 1, fmt.Errorf("catch statement must follow a try statement at line %d", lineNum+1)

	case "$raw":
		if !isStandardLibraryFile(CurrentSourceFile) {
			return nil, lineNum + 1, fmt.Errorf("$raw blocks are only allowed in the standard library at line %d", lineNum+1)
		}

		if !strings.HasSuffix(line, "(") {
			return nil, lineNum + 1, fmt.Errorf("$raw block must start with '(' at line %d", lineNum+1)
		}

		var (
			rawCode     strings.Builder
			currentLine = lineNum + 1
			parenCount  = 1
		)

		for currentLine < len(lines) && parenCount > 0 {
			line := lines[currentLine]

			for _, char := range line {
				switch char {
				case '(':
					parenCount++
				case ')':
					parenCount--
				}

				if parenCount > 0 {
					rawCode.WriteRune(char)
				}
			}

			if parenCount > 0 {
				rawCode.WriteString("\n")
			}
			currentLine++
		}

		if parenCount > 0 {
			return nil, lineNum + 1, fmt.Errorf("unclosed $raw block starting at line %d", lineNum+1)
		}

		code := strings.TrimSpace(rawCode.String())

		if !isStandardLibraryFile(CurrentSourceFile) {
			if hasReserved, reservedCmd := containsReserved(code); hasReserved {
				return nil, lineNum + 1, fmt.Errorf("unknown statement type '%s' at line %d", reservedCmd, lineNum+1)
			}
		}

		return &Statement{RawCode: &RawCodeStmt{Code: code}}, currentLine, nil

	default:
		if len(parts) >= 3 && parts[1] == "=" {
			var (
				varName = parts[0]
				eqIndex = strings.Index(line, "=")
			)

			if eqIndex == -1 {
				return nil, lineNum, fmt.Errorf("malformed assignment")
			}

			value := strings.TrimSpace(line[eqIndex+1:])

			if strings.HasSuffix(value, ";") {
				value = strings.TrimSpace(value[:len(value)-1])
			}

			if strings.Contains(value, ".") && strings.Contains(value, "(") && strings.Contains(value, ")") && !strings.HasPrefix(value, "new ") {
				dotIndex := strings.Index(value, ".")
				parenIndex := strings.Index(value, "(")
				if dotIndex < parenIndex {
					var (
						objectName     = strings.TrimSpace(value[:dotIndex])
						methodPart     = strings.TrimSpace(value[dotIndex+1:])
						methodEndIndex = strings.Index(methodPart, "(")
						methodName     = strings.TrimSpace(methodPart[:methodEndIndex])
						argsStart      = strings.Index(value, "(")
						argsEnd        = strings.LastIndex(value, ")")
						args           []string
					)
					if argsEnd > argsStart+1 {
						argsStr := strings.TrimSpace(value[argsStart+1 : argsEnd])
						if argsStr != "" {
							argsList := strings.SplitSeq(argsStr, ",")
							for arg := range argsList {
								args = append(args, strings.TrimSpace(arg))
							}
						}
					}
					return &Statement{VarAssignMethodCall: &VarAssignMethodCallStmt{
						Name:   varName,
						Object: objectName,
						Method: methodName,
						Args:   args,
					}}, lineNum + 1, nil
				}
			}
			if strings.Contains(varName, "[") && strings.Contains(varName, "]") {
				bracketStart := strings.Index(varName, "[")
				bracketEnd := strings.LastIndex(varName, "]")

				if bracketStart == -1 || bracketEnd == -1 || bracketEnd <= bracketStart {
					return nil, lineNum + 1, fmt.Errorf("invalid index assignment format at line %d", lineNum+1)
				}

				afterBracket := strings.TrimSpace(varName[bracketEnd+1:])
				if strings.HasPrefix(afterBracket, ".") {
					return &Statement{VarAssign: &VarAssignStmt{Name: varName, Value: value}}, lineNum + 1, nil
				}

				listName := strings.TrimSpace(varName[:bracketStart])
				index := strings.TrimSpace(varName[bracketStart+1 : bracketEnd])

				return &Statement{IndexAssign: &IndexAssignStmt{
					ListName: listName,
					Index:    index,
					Value:    value,
				}}, lineNum + 1, nil
			}

			return &Statement{VarAssign: &VarAssignStmt{Name: varName, Value: value}}, lineNum + 1, nil
		}
		if strings.Contains(line, "=") && (strings.Contains(line, "[") && strings.Contains(line, "]")) {
			eqIndex := strings.Index(line, "=")
			if eqIndex > 0 && eqIndex < len(line)-1 {
				varName := strings.TrimSpace(line[:eqIndex])
				value := strings.TrimSpace(line[eqIndex+1:])
				if strings.HasSuffix(value, ";") {
					value = strings.TrimSpace(value[:len(value)-1])
				}
				if strings.Contains(varName, "[") && strings.Contains(varName, "]") {
					bracketStart := strings.Index(varName, "[")
					bracketEnd := strings.LastIndex(varName, "]")

					if bracketStart != -1 && bracketEnd != -1 && bracketEnd > bracketStart {
						listName := strings.TrimSpace(varName[:bracketStart])
						index := strings.TrimSpace(varName[bracketStart+1 : bracketEnd])

						return &Statement{IndexAssign: &IndexAssignStmt{
							ListName: listName,
							Index:    index,
							Value:    value,
						}}, lineNum + 1, nil
					}
				}
			}
		}

		if strings.HasPrefix(parts[0], "this.") && len(parts) >= 3 && parts[1] == "=" {
			fieldName := parts[0][5:]
			value := strings.Join(parts[2:], " ")
			return &Statement{VarAssign: &VarAssignStmt{Name: "this." + fieldName, Value: value}}, lineNum + 1, nil
		}

		if strings.Contains(line, "(") && strings.Contains(line, ")") && !strings.Contains(line, "=") && !strings.Contains(line, ".") {
			parenStart := strings.Index(line, "(")
			parenEnd := strings.LastIndex(line, ")")

			if parenStart > 0 {
				funcName := strings.TrimSpace(line[:parenStart])

				if target, exists := Aliases[funcName]; exists {
					funcName = target
				}

				var args []string

				if parenEnd > parenStart+1 {
					argsStr := strings.TrimSpace(line[parenStart+1 : parenEnd])
					if argsStr != "" {
						args = parseArgumentsRespectingNesting(argsStr)
					}
				}

				return &Statement{FunctionCall: &FunctionCallStmt{Name: funcName, Args: args}}, lineNum + 1, nil
			}
		}

		// Check for static method call (ClassName::methodName(...))
		if strings.Contains(line, "::") && strings.Contains(line, "(") && strings.Contains(line, ")") && !strings.Contains(line, "=") {
			doubleColonIndex := strings.Index(line, "::")
			parenIndex := strings.Index(line, "(")
			if doubleColonIndex < parenIndex {
				className := strings.TrimSpace(line[:doubleColonIndex])
				methodPart := strings.TrimSpace(line[doubleColonIndex+2:])
				methodEndIndex := strings.Index(methodPart, "(")

				if methodEndIndex == -1 {
					return nil, lineNum + 1, fmt.Errorf("invalid static method call syntax at line %d", lineNum+1)
				}

				var (
					methodName = strings.TrimSpace(methodPart[:methodEndIndex])
					argsStart  = strings.Index(line, "(")
					argsEnd    = strings.LastIndex(line, ")")
				)

				fullCall := strings.TrimSpace(line[:parenIndex])
				if target, exists := Aliases[fullCall]; exists {
					if strings.Contains(target, "::") {
						targetParts := strings.SplitN(target, "::", 2)
						if len(targetParts) == 2 {
							className = strings.TrimSpace(targetParts[0])
							methodName = strings.TrimSpace(targetParts[1])
						}
					}
				}

				var args []string
				if argsEnd > argsStart+1 {
					argsStr := strings.TrimSpace(line[argsStart+1 : argsEnd])
					if argsStr != "" {
						argsList := strings.SplitSeq(argsStr, ",")
						for arg := range argsList {
							args = append(args, strings.TrimSpace(arg))
						}
					}
				}

				return &Statement{StaticMethodCall: &StaticMethodCallStmt{Class: className, Method: methodName, Args: args}}, lineNum + 1, nil
			}
		}

		if strings.Contains(line, ".") && strings.Contains(line, "(") && strings.Contains(line, ")") && !strings.Contains(line, "=") {
			parenIndex := strings.Index(line, "(")
			lastDotIndex := -1
			for i := parenIndex - 1; i >= 0; i-- {
				if line[i] == '.' {
					lastDotIndex = i
					break
				}
			}

			if lastDotIndex != -1 {
				var (
					objectName     = strings.TrimSpace(line[:lastDotIndex])
					methodPart     = strings.TrimSpace(line[lastDotIndex+1:])
					methodEndIndex = strings.Index(methodPart, "(")
				)

				if methodEndIndex == -1 {
					return nil, lineNum + 1, fmt.Errorf("invalid method call syntax at line %d", lineNum+1)
				}

				var (
					methodName = strings.TrimSpace(methodPart[:methodEndIndex])
					argsStart  = strings.Index(line, "(")
					argsEnd    = strings.LastIndex(line, ")")
				)

				fullCall := strings.TrimSpace(line[:parenIndex])
				if target, exists := Aliases[fullCall]; exists {
					if strings.Contains(target, ".") {
						targetParts := strings.SplitN(target, ".", 2)
						if len(targetParts) == 2 {
							objectName = strings.TrimSpace(targetParts[0])
							methodName = strings.TrimSpace(targetParts[1])
						}
					}
				}

				var args []string
				if argsEnd > argsStart+1 {
					argsStr := strings.TrimSpace(line[argsStart+1 : argsEnd])
					if argsStr != "" {
						argsList := strings.SplitSeq(argsStr, ",")
						for arg := range argsList {
							args = append(args, strings.TrimSpace(arg))
						}
					}
				}

				return &Statement{MethodCall: &MethodCallStmt{Object: objectName, Method: methodName, Args: args}}, lineNum + 1, nil
			}
		}

		if len(parts) >= 5 && parts[2] == "=" && parts[3] == "new" {
			var (
				typeName   = parts[0]
				varName    = parts[1]
				newPart    = strings.TrimSpace(line[strings.Index(line, "new")+3:])
				parenStart = strings.Index(newPart, "(")
			)

			if parenStart == -1 {
				return nil, lineNum + 1, fmt.Errorf("object declaration missing parentheses at line %d", lineNum+1)
			}

			className := strings.TrimSpace(newPart[:parenStart])
			var args []string
			if strings.Contains(className, ".") {
				parts := strings.Split(className, ".")
				if len(parts) == 2 {
					args = []string{parts[0], parts[1]}
				} else {
					return nil, lineNum + 1, fmt.Errorf("invalid module-qualified class name at line %d", lineNum+1)
				}
			} else {
				args = []string{className}
			}

			argsStart := strings.Index(line, "(")
			argsEnd := strings.LastIndex(line, ")")
			if argsStart != -1 && argsEnd != -1 && argsEnd > argsStart+1 {
				constructorArgsStr := strings.TrimSpace(line[argsStart+1 : argsEnd])
				if constructorArgsStr != "" {
					constructorArgsList := strings.SplitSeq(constructorArgsStr, ",")
					for arg := range constructorArgsList {
						args = append(args, strings.TrimSpace(arg))
					}
				}
			}

			return &Statement{ObjectDecl: &ObjectDeclStmt{Type: typeName, Name: varName, Args: args}}, lineNum + 1, nil
		}

		if strings.HasPrefix(parts[0], "list[") && strings.Contains(parts[0], "]") {
			if len(parts) < 4 || parts[2] != "=" {
				return nil, lineNum + 1, fmt.Errorf("list declaration format error at line %d (expected: list[type] name = [elements])", lineNum+1)
			}

			typeStart := strings.Index(parts[0], "[")
			typeEnd := strings.Index(parts[0], "]")
			if typeStart == -1 || typeEnd == -1 || typeEnd <= typeStart {
				return nil, lineNum + 1, fmt.Errorf("invalid list type declaration at line %d", lineNum+1)
			}

			listType := parts[0][typeStart+1 : typeEnd]
			listName := parts[1]

			elementsStart := strings.Index(line, "[")
			secondBracketPos := strings.Index(line[elementsStart+1:], "[")
			if secondBracketPos != -1 {
				elementsStart = elementsStart + 1 + secondBracketPos
			} else {
				return nil, lineNum + 1, fmt.Errorf("list declaration missing elements at line %d", lineNum+1)
			}

			elementsEnd := strings.LastIndex(line, "]")
			if elementsEnd == -1 || elementsEnd <= elementsStart {
				return nil, lineNum + 1, fmt.Errorf("list declaration missing closing bracket at line %d", lineNum+1)
			}

			elementsStr := strings.TrimSpace(line[elementsStart+1 : elementsEnd])
			var elements []string

			if elementsStr != "" {
				elementsList := strings.Split(elementsStr, ",")
				for _, elem := range elementsList {
					elem = strings.TrimSpace(elem)
					if elem != "" {
						if strings.HasPrefix(elem, "\"") && strings.HasSuffix(elem, "\"") {
							elem = elem[1 : len(elem)-1]
						}
						elements = append(elements, elem)
					}
				}
			}

			return &Statement{ListDecl: &ListDeclStmt{Type: listType, Name: listName, Elements: elements}}, lineNum + 1, nil
		}

		if len(parts) >= 4 && parts[2] == "=" && isValidType(parts[0]) {
			varType := parts[0]
			varName := parts[1]
			value := strings.Join(parts[3:], " ")
			if strings.HasPrefix(value, "new ") {
				newPart := strings.TrimSpace(value[4:]) // Remove "new "
				parenStart := strings.Index(newPart, "(")
				if parenStart == -1 {
					return nil, lineNum + 1, fmt.Errorf("object declaration missing parentheses at line %d", lineNum+1)
				}

				className := strings.TrimSpace(newPart[:parenStart])

				var constructorArgs []string
				argsStart := strings.Index(value, "(")
				argsEnd := strings.LastIndex(value, ")")
				if argsStart != -1 && argsEnd != -1 && argsEnd > argsStart+1 {
					constructorArgsStr := strings.TrimSpace(value[argsStart+1 : argsEnd])
					if constructorArgsStr != "" {
						constructorArgsList := strings.Split(constructorArgsStr, ",")
						for _, arg := range constructorArgsList {
							constructorArgs = append(constructorArgs, strings.TrimSpace(arg))
						}
					}
				}

				// Handle module-qualified types
				typeName := className
				var args []string
				if strings.Contains(className, "::") {
					parts := strings.Split(className, "::")
					if len(parts) == 2 {
						args = append(args, parts[0]) // module name
						args = append(args, parts[1]) // class name
						typeName = className          // Keep full qualified name as type
					} else {
						return nil, lineNum + 1, fmt.Errorf("invalid module-qualified class name at line %d", lineNum+1)
					}
				} else {
					args = append(args, className)
				}
				args = append(args, constructorArgs...)

				return &Statement{ObjectDecl: &ObjectDeclStmt{Type: typeName, Name: varName, Args: args}}, lineNum + 1, nil
			}
			if strings.Contains(value, ".") && strings.Contains(value, "(") && strings.Contains(value, ")") && !strings.HasPrefix(value, "new ") {
				dotIndex := strings.Index(value, ".")
				parenIndex := strings.Index(value, "(")
				if dotIndex < parenIndex {
					objectName := strings.TrimSpace(value[:dotIndex])
					methodPart := strings.TrimSpace(value[dotIndex+1:])
					methodEndIndex := strings.Index(methodPart, "(")
					methodName := strings.TrimSpace(methodPart[:methodEndIndex])

					argsStart := strings.Index(value, "(")
					argsEnd := strings.LastIndex(value, ")")
					var args []string
					if argsEnd > argsStart+1 {
						argsStr := strings.TrimSpace(value[argsStart+1 : argsEnd])
						if argsStr != "" {
							argsList := strings.Split(argsStr, ",")
							for _, arg := range argsList {
								args = append(args, strings.TrimSpace(arg))
							}
						}
					}

					return &Statement{VarDeclMethodCall: &VarDeclMethodCallStmt{
						Type:   varType,
						Name:   varName,
						Object: objectName,
						Method: methodName,
						Args:   args,
					}}, lineNum + 1, nil
				}
			}

			if strings.Contains(value, "read(") {
				start := strings.Index(value, "read(")
				end := strings.LastIndex(value, ")")
				if start != -1 && end != -1 {
					filePath := value[start+5 : end]
					return &Statement{VarDeclRead: &VarDeclReadStmt{Type: varType, Name: varName, FilePath: filePath}}, lineNum + 1, nil
				}
			}

			return &Statement{VarDecl: &VarDeclStmt{Type: varType, Name: varName, Value: value}}, lineNum + 1, nil
		}

		if strings.HasPrefix(line, "write(") && strings.HasSuffix(line, ")") {
			start := strings.Index(line, "(")
			end := strings.LastIndex(line, ")")
			if start != -1 && end != -1 && end > start {
				argsStr := strings.TrimSpace(line[start+1 : end])
				if argsStr != "" {
					args := strings.Split(argsStr, ",")
					if len(args) >= 2 {
						content := strings.TrimSpace(args[0])
						filePath := strings.TrimSpace(args[1])
						mode := "overwrite!"
						if len(args) >= 3 {
							mode = strings.TrimSpace(args[2])
						}
						if strings.HasPrefix(filePath, "\"") && strings.HasSuffix(filePath, "\"") {
							filePath = filePath[1 : len(filePath)-1]
						}
						if strings.HasPrefix(mode, "\"") && strings.HasSuffix(mode, "\"") {
							mode = mode[1 : len(mode)-1]
						}
						return &Statement{VarDeclWrite: &VarDeclWriteStmt{
							Content:  content,
							FilePath: filePath,
							Mode:     mode,
						}}, lineNum + 1, nil
					}
				}
			}
		}
	}

	if strings.Contains(line, "(") && strings.HasSuffix(line, ")") && !strings.HasSuffix(line, ":") &&
		!strings.Contains(line, "*") {
		firstWord := strings.Fields(line)[0]
		isKeyword := false
		keywords := []string{"if", "for", "while", "fn", "class",
			"var", "return", "import", "pub", "ref", "u16", "u32", "u64",
			"i16", "i32", "i64", "f32", "f64", "print", "sleep", "break",
			"continue", "foreach", "parallel", "char*", "allocate", "stallocate", "free", "new"}
		if slices.Contains(keywords, firstWord) {
			isKeyword = true
		}
		if !isKeyword {
			return &Statement{Run: &RunStmt{FunctionCall: line}}, lineNum + 1, nil
		}

	}
	if parts[0] == "char*" {
		return &Statement{VarDecl: &VarDeclStmt{Type: "char*", Name: parts[1], Value: parts[3]}}, lineNum + 1, nil
	}
	return nil, lineNum + 1, fmt.Errorf("unknown statement type '%s' at line %d", parts[0], lineNum+1)
}

func splitMapPairs(input string) []string {
	var pairs []string
	var currentPair strings.Builder
	inQuotes := false
	parenCount := 0
	bracketCount := 0

	for _, char := range input {
		switch char {
		case '"':
			if inQuotes && currentPair.Len() > 0 && currentPair.String()[currentPair.Len()-1] != '\\' {
				inQuotes = false
			} else if !inQuotes {
				inQuotes = true
			}
			currentPair.WriteRune(char)
		case ',':
			if !inQuotes && parenCount == 0 && bracketCount == 0 {
				pair := strings.TrimSpace(currentPair.String())
				if pair != "" {
					pairs = append(pairs, pair)
				}
				currentPair.Reset()
				continue
			}
			currentPair.WriteRune(char)
		case '(':
			if !inQuotes {
				parenCount++
			}
			currentPair.WriteRune(char)
		case ')':
			if !inQuotes {
				parenCount--
			}
			currentPair.WriteRune(char)
		case '[':
			if !inQuotes {
				bracketCount++
			}
			currentPair.WriteRune(char)
		case ']':
			if !inQuotes {
				bracketCount--
			}
			currentPair.WriteRune(char)
		default:
			currentPair.WriteRune(char)
		}
	}
	pair := strings.TrimSpace(currentPair.String())
	if pair != "" {
		pairs = append(pairs, pair)
	}
	return pairs
}

func parseArgumentsRespectingNesting(argsStr string) []string {
	if strings.TrimSpace(argsStr) == "" {
		return []string{}
	}

	var (
		args            []string
		current         strings.Builder
		parenDepth      = 0
		inString        = false
		stringDelimiter rune
		escapeNext      = false
	)

	for _, char := range argsStr {
		if inString {
			if escapeNext {
				escapeNext = false
				current.WriteRune(char)
				continue
			}
			if char == '\\' {
				escapeNext = true
				current.WriteRune(char)
				continue
			}
			if char == stringDelimiter {
				inString = false
				current.WriteRune(char)
				continue
			}
			current.WriteRune(char)
			continue
		}

		switch char {
		case '\'', '"':
			inString = true
			stringDelimiter = char
			current.WriteRune(char)
		case '(':
			parenDepth++
			current.WriteRune(char)
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
			current.WriteRune(char)
		case ',':
			if parenDepth == 0 {
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

func parseNewExprStatement(line string, lineNum int) (*Statement, int, error) {
	expr := strings.TrimSpace(line[4:])
	parenPos := strings.Index(expr, "(")

	if parenPos == -1 {
		return nil, lineNum, fmt.Errorf("invalid new expression: %s", line)
	}

	parenDepth := 0
	closingParenPos := -1
	for i := parenPos; i < len(expr); i++ {
		if expr[i] == '(' {
			parenDepth++
		} else if expr[i] == ')' {
			parenDepth--
			if parenDepth == 0 {
				closingParenPos = i
				break
			}
		}
	}

	if closingParenPos == -1 {
		return nil, lineNum, fmt.Errorf("invalid new expression, missing closing parenthesis: %s", line)
	}

	className := strings.TrimSpace(expr[:parenPos])
	if className == "" {
		return nil, lineNum, fmt.Errorf("missing class name in new expression: %s", line)
	}
	argsStr := expr[parenPos+1 : closingParenPos]
	args := []string{}
	if strings.TrimSpace(argsStr) != "" {
		args = splitRespectingQuotes(argsStr)
	}

	remaining := strings.TrimSpace(expr[closingParenPos+1:])
	if strings.HasPrefix(remaining, ".") {
		methodCall := remaining[1:] // Remove the dot
		methodParenPos := strings.Index(methodCall, "(")
		if methodParenPos == -1 {
			return nil, lineNum, fmt.Errorf("invalid method call after new expression: %s", line)
		}

		methodName := strings.TrimSpace(methodCall[:methodParenPos])
		if methodName == "" {
			return nil, lineNum, fmt.Errorf("missing method name in chained call: %s", line)
		}

		methodParenDepth := 0
		methodClosingParenPos := -1
		for i := methodParenPos; i < len(methodCall); i++ {
			if methodCall[i] == '(' {
				methodParenDepth++
			} else if methodCall[i] == ')' {
				methodParenDepth--
				if methodParenDepth == 0 {
					methodClosingParenPos = i
					break
				}
			}
		}

		if methodClosingParenPos == -1 {
			return nil, lineNum, fmt.Errorf("invalid method call, missing closing parenthesis: %s", line)
		}

		methodArgsStr := methodCall[methodParenPos+1 : methodClosingParenPos]
		methodArgs := []string{}
		if strings.TrimSpace(methodArgsStr) != "" {
			methodArgs = splitRespectingQuotes(methodArgsStr)
		}

		tempObjectName := fmt.Sprintf("new_%s(%s)", className, strings.Join(args, ", "))

		return &Statement{MethodCall: &MethodCallStmt{
			Object: tempObjectName,
			Method: methodName,
			Args:   methodArgs,
		}}, lineNum + 1, nil
	}

	return &Statement{NewExpr: &NewExprStmt{
		ClassName: className,
		Args:      args,
	}}, lineNum + 1, nil
}

func findMapTypeEnd(line string) int {
	if !strings.HasPrefix(line, "map[") {
		return -1
	}

	bracketDepth := 0
	start := strings.Index(line, "[")
	if start == -1 {
		return -1
	}

	for i := start; i < len(line); i++ {
		switch line[i] {
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
			if bracketDepth == 0 {
				return i
			}
		}
	}
	return -1
}

func findListTypeEnd(typeDecl string) int {
	if !strings.HasPrefix(typeDecl, "list[") {
		return -1
	}

	bracketDepth := 0
	start := strings.Index(typeDecl, "[")
	if start == -1 {
		return -1
	}

	for i := start; i < len(typeDecl); i++ {
		switch typeDecl[i] {
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
			if bracketDepth == 0 {
				return i
			}
		}
	}
	return -1
}

// Finds the colon that separates key:value in a map pair,
// ignoring colons inside nested brackets, parentheses, or quotes
func findPairColonPosition(pairStr string) int {
	bracketDepth := 0
	parenDepth := 0
	inQuotes := false

	for i, char := range pairStr {
		switch char {
		case '"':
			if i == 0 || pairStr[i-1] != '\\' {
				inQuotes = !inQuotes
			}
		case '[':
			if !inQuotes {
				bracketDepth++
			}
		case ']':
			if !inQuotes {
				bracketDepth--
			}
		case '(':
			if !inQuotes {
				parenDepth++
			}
		case ')':
			if !inQuotes {
				parenDepth--
			}
		case ':':
			if !inQuotes && bracketDepth == 0 && parenDepth == 0 {
				return i
			}
		}
	}
	return -1
}

// Splits list elements by comma while respecting nested brackets and quotes
func splitListElementsRespectingBrackets(elementsStr string) []string {
	var elements []string
	var current strings.Builder
	bracketDepth := 0
	inQuotes := false

	for i, char := range elementsStr {
		switch char {
		case '"':
			if i == 0 || elementsStr[i-1] != '\\' {
				inQuotes = !inQuotes
			}
			current.WriteRune(char)
		case '[':
			if !inQuotes {
				bracketDepth++
			}
			current.WriteRune(char)
		case ']':
			if !inQuotes {
				bracketDepth--
			}
			current.WriteRune(char)
		case ',':
			if !inQuotes && bracketDepth == 0 {
				element := strings.TrimSpace(current.String())
				if element != "" {
					elements = append(elements, element)
				}
				current.Reset()
			} else {
				current.WriteRune(char)
			}
		default:
			current.WriteRune(char)
		}
	}

	element := strings.TrimSpace(current.String())
	if element != "" {
		elements = append(elements, element)
	}

	return elements
}

// Finds the closing bracket that matches the opening bracket at the given position
func findMatchingClosingBracket(str string, openPos int) int {
	if openPos >= len(str) || str[openPos] != '[' {
		return -1
	}

	bracketDepth := 0
	inQuotes := false

	for i := openPos; i < len(str); i++ {
		switch str[i] {
		case '"':
			if i == 0 || str[i-1] != '\\' {
				inQuotes = !inQuotes
			}
		case '[':
			if !inQuotes {
				bracketDepth++
			}
		case ']':
			if !inQuotes {
				bracketDepth--
				if bracketDepth == 0 {
					return i
				}
			}
		}
	}

	return -1
}

func parseAliasDeclaration(line string, lineNum int) (*Statement, int, error) {
	aliasPart := strings.TrimSpace(line[6:])

	parts := strings.SplitN(aliasPart, "=", 2)
	if len(parts) != 2 {
		return nil, lineNum + 1, fmt.Errorf("invalid alias syntax at line %d: expected 'alias name = target'", lineNum+1)
	}

	aliasName := strings.TrimSpace(parts[0])
	target := strings.TrimSpace(parts[1])

	if aliasName == "" || target == "" {
		return nil, lineNum + 1, fmt.Errorf("alias name and target cannot be empty at line %d", lineNum+1)
	}

	Aliases[aliasName] = target

	return &Statement{Alias: &AliasStmt{
		AliasName: aliasName,
		Target:    target,
	}}, lineNum + 1, nil
}
