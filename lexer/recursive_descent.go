// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Contains the recursive descent parser for the scar programming language.

package lexer

import (
	"fmt"
	"strings"
)

func parseStatements(lines []string, startLine, expectedIndent int) ([]*Statement, error) {
	var statements []*Statement
	i := startLine

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
		result   []string
		current  strings.Builder
		inQuotes = false
	)

	for i, char := range input {
		switch char {
		case '"':
			if i > 0 && input[i-1] == '\\' {
				current.WriteRune(char)
			} else {
				inQuotes = !inQuotes
				current.WriteRune(char)
			}
		case ',':
			if !inQuotes {
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

func parseStatement(lines []string, lineNum, currentIndent int) (*Statement, int, error) {
	line := strings.TrimSpace(lines[lineNum])

	if strings.HasPrefix(line, "map[") && strings.Contains(line, "]") && strings.Contains(line, "=") {
		typeEnd := strings.Index(line, "]")
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
			typeParts = strings.SplitN(typeDecl, ":", 2)
		)
		if len(typeParts) != 2 {
			return nil, lineNum + 1, fmt.Errorf("map type must specify key:value types at line %d", lineNum+1)
		}
		var (
			keyType   = strings.TrimSpace(typeParts[0])
			valueType = strings.TrimSpace(typeParts[1])
		)
		if keyType == "" || valueType == "" {
			return nil, lineNum + 1, fmt.Errorf("map type must specify valid key and value types at line %d", lineNum+1)
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
						colonIdx := strings.Index(pairStr, ":")
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

	if len(parts) == 0 {
		return nil, lineNum + 1, fmt.Errorf("empty statement at line %d", lineNum+1)
	}

	if strings.HasPrefix(line, "put!(") && strings.HasSuffix(line, ")") {
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
	case "parallel":
		if len(parts) < 5 || parts[1] != "for" || parts[3] != "=" || !strings.Contains(line, "to") || !strings.HasSuffix(line, ":") {
			return nil, lineNum + 1, fmt.Errorf("parallel for statement format error at line %d (expected: parallel for var = start to end:)", lineNum+1)
		}

		equalsIndex := strings.Index(line, "=")
		toIndex := strings.Index(line, "to")
		colonIndex := strings.LastIndex(line, ":")
		if equalsIndex == -1 || toIndex == -1 || colonIndex == -1 ||
			!(equalsIndex > strings.Index(line, "for") && equalsIndex < toIndex && toIndex < colonIndex) {
			return nil, lineNum + 1, fmt.Errorf("parallel for statement format error at line %d", lineNum+1)
		}

		var (
			varName = strings.TrimSpace(line[strings.Index(line, "for")+len("for") : equalsIndex])
			start   = strings.TrimSpace(line[equalsIndex+1 : toIndex])
			end     = strings.TrimSpace(line[toIndex+len("to") : colonIndex])
		)

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

		return &Statement{ParallelFor: &ParallelForStmt{Var: varName, Start: start, End: end, Body: body}}, nextLine, nil

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
				if nextTrimmed != "" && getIndentation(lines[nextLine]) > 0 {
					return parseBulkImport(lines, lineNum)
				}
			}
			moduleName := strings.Trim(strings.Join(parts[1:], " "), "\"")
			return &Statement{Import: &ImportStmt{Module: moduleName}}, lineNum + 1, nil
		}
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
		}
		return &Statement{Print: &PrintStmt{Print: str}}, lineNum + 1, nil

	case "sleep":
		if len(parts) < 2 {
			return nil, lineNum + 1, fmt.Errorf("sleep statement requires a number at line %d", lineNum+1)
		}
		return &Statement{Sleep: &SleepStmt{Duration: parts[1]}}, lineNum + 1, nil

	case "break":
		return &Statement{Break: &BreakStmt{Break: "break"}}, lineNum + 1, nil

	case "continue":
		return &Statement{Continue: &ContinueStmt{Continue: "continue"}}, lineNum + 1, nil

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
		if !strings.HasSuffix(collection, ".keys") && !strings.HasSuffix(collection, ".values") {
			return nil, lineNum + 1, fmt.Errorf("foreach statement collection must be 'mapname.keys' or 'mapname.values' at line %d", lineNum+1)
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
		if !strings.HasSuffix(line, "(") {
			return nil, lineNum + 1, fmt.Errorf("$raw block must start with '(' at line %d", lineNum+1)
		}

		var rawCode strings.Builder
		currentLine := lineNum + 1
		parenCount := 1

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
		return &Statement{RawCode: &RawCodeStmt{Code: code}}, currentLine, nil

	default:
		if len(parts) >= 3 && parts[1] == "=" {
			varName := parts[0]
			value := strings.Join(parts[2:], " ")

			// Check for method call assignment
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

					return &Statement{VarAssignMethodCall: &VarAssignMethodCallStmt{
						Name:   varName,
						Object: objectName,
						Method: methodName,
						Args:   args,
					}}, lineNum + 1, nil
				}
			}

			if strings.Contains(varName, "[") && strings.Contains(varName, "]") {
				value = handleIndexAssignment(line, varName, value)
			} else if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
				value = value[1 : len(value)-1]
			}

			// The simple operator parsing logic was removed as it was causing issues
			// with complex expressions. The full expression is now passed as-is to ResolveSymbol.
			//
			// May cause issues later, need to revisit.

			return &Statement{VarAssign: &VarAssignStmt{Name: varName, Value: value}}, lineNum + 1, nil
		}
		// If not an assignment, fall through to the error below
		if strings.HasPrefix(parts[0], "this.") && len(parts) >= 3 && parts[1] == "=" {
			fieldName := parts[0][5:] // Remove "this."
			value := strings.Join(parts[2:], " ")
			return &Statement{VarAssign: &VarAssignStmt{Name: "this." + fieldName, Value: value}}, lineNum + 1, nil
		}

		if strings.Contains(line, "(") && strings.Contains(line, ")") && !strings.Contains(line, "=") && !strings.Contains(line, ".") {
			parenStart := strings.Index(line, "(")
			parenEnd := strings.LastIndex(line, ")")

			if parenStart > 0 {
				funcName := strings.TrimSpace(line[:parenStart])
				var args []string

				if parenEnd > parenStart+1 {
					argsStr := strings.TrimSpace(line[parenStart+1 : parenEnd])
					if argsStr != "" {
						argsList := strings.Split(argsStr, ",")
						for _, arg := range argsList {
							args = append(args, strings.TrimSpace(arg))
						}
					}
				}

				return &Statement{FunctionCall: &FunctionCallStmt{Name: funcName, Args: args}}, lineNum + 1, nil
			}
		}

		if strings.Contains(line, ".") && strings.Contains(line, "(") && strings.Contains(line, ")") && !strings.Contains(line, "=") {
			dotIndex := strings.Index(line, ".")
			parenIndex := strings.Index(line, "(")
			if dotIndex < parenIndex {
				objectName := strings.TrimSpace(line[:dotIndex])
				methodPart := strings.TrimSpace(line[dotIndex+1:])

				methodEndIndex := strings.Index(methodPart, "(")
				if methodEndIndex == -1 {
					return nil, lineNum + 1, fmt.Errorf("invalid method call syntax at line %d", lineNum+1)
				}

				methodName := strings.TrimSpace(methodPart[:methodEndIndex])
				argsStart := strings.Index(line, "(")
				argsEnd := strings.LastIndex(line, ")")

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
			typeName := parts[0]
			varName := parts[1]
			newPart := strings.TrimSpace(line[strings.Index(line, "new")+3:])
			parenStart := strings.Index(newPart, "(")
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

			if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
				value = value[1 : len(value)-1]
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
	return nil, lineNum + 1, fmt.Errorf("unknown statement type '%s' at line %d", parts[0], lineNum+1)
}

func splitMapPairs(input string) []string {
	var pairs []string
	var currentPair strings.Builder
	inQuotes := false
	parenCount := 0

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
			if !inQuotes && parenCount == 0 {
				pair := strings.TrimSpace(currentPair.String())
				if pair != "" {
					pairs = append(pairs, pair)
				}
				currentPair.Reset()
				continue
			}
			currentPair.WriteRune(char)
		case '(':
			if inQuotes {
				currentPair.WriteRune(char)
			} else {
				parenCount++
				currentPair.WriteRune(char)
			}
		case ')':
			if inQuotes {
				currentPair.WriteRune(char)
			} else {
				parenCount--
				currentPair.WriteRune(char)
			}
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
