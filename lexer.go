package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
)

var dslLexer = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Comment", Pattern: `#[^\n]*`},
	{Name: "Number", Pattern: `\d+`},
	{Name: "String", Pattern: `"[^"]*"`},
	{Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`},
	{Name: "Colon", Pattern: `:`},
	{Name: "Newline", Pattern: `\n`},
	{Name: "Indent", Pattern: `^[ \t]+`},
	{Name: "Whitespace", Pattern: `[ \t\r]+`},
	{Name: "Assign", Pattern: `=`},
	{Name: "To", Pattern: `to`},
	{Name: "If", Pattern: `if`},
	{Name: "Elif", Pattern: `elif`},
	{Name: "Else", Pattern: `else`},
	{Name: "Reassign", Pattern: `reassign`},
	{Name: "Break", Pattern: `break`},
	{Name: "LeftBracket", Pattern: `\[`},
	{Name: "RightBracket", Pattern: `\]`},
	{Name: "LeftParen", Pattern: `\(`},
	{Name: "RightParen", Pattern: `\)`},
	{Name: "Comma", Pattern: `,`},
	{Name: "Dot", Pattern: `\.`},
	{Name: "Arrow", Pattern: `->`},
	{Name: "Class", Pattern: `class`},
	{Name: "Init", Pattern: `init`},
	{Name: "Fn", Pattern: `fn`},
	{Name: "This", Pattern: `this`},
	{Name: "New", Pattern: `new`},
	{Name: "Void", Pattern: `void`},
})

type Program struct {
	Statements []*Statement `{ @@ ( Newline+ @@ )* Newline* }`
}

type Statement struct {
	Print             *PrintStmt             `  @@`
	Sleep             *SleepStmt             `| @@`
	While             *WhileStmt             `| @@`
	For               *ForStmt               `| @@`
	If                *IfStmt                `| @@`
	Break             *BreakStmt             `| @@`
	VarDecl           *VarDeclStmt           `| @@`
	VarAssign         *VarAssignStmt         `| @@`
	ListDecl          *ListDeclStmt          `| @@`
	ClassDecl         *ClassDeclStmt         `| @@`
	MethodCall        *MethodCallStmt        `| @@`
	ObjectDecl        *ObjectDeclStmt        `| @@`
	Return            *ReturnStmt            `| @@`
	VarDeclMethodCall *VarDeclMethodCallStmt `| @@`
}

type VarDeclMethodCallStmt struct {
	Type   string   `@Ident`
	Name   string   `@Ident`
	Object string   `"=" @Ident`
	Method string   `"." @Ident`
	Args   []string `"(" ( @(Ident | Number | String) ( "," @(Ident | Number | String) )* )? ")"`
}

type ReturnStmt struct {
	Value string `"return" @(Ident | Number | String)`
}

type PrintStmt struct {
	Print     string   `"print" @String`
	Format    string   `| "print" @String`
	Variables []string `"|" @Ident ( "," @Ident )*`
}

type SleepStmt struct {
	Duration string `"sleep" @Number`
}

type WhileStmt struct {
	Condition string       `"while" @(Ident | Number) ":" Newline+`
	Body      []*Statement `@@*`
}

type ForStmt struct {
	Var   string       `"for" @Ident`
	Start string       `"=" @Number`
	End   string       `"to" @Number ":" Newline+`
	Body  []*Statement `@@*`
}

type IfStmt struct {
	Condition string       `"if" @(Ident | Number | String) ":" Newline+`
	Body      []*Statement `@@*`
	ElseIfs   []*ElifStmt  `@@*`
	Else      *ElseStmt    `@@?`
}

type ElifStmt struct {
	Condition string       `"elif" @(Ident | Number | String) ":" Newline+`
	Body      []*Statement `@@*`
}

type ElseStmt struct {
	Body []*Statement `"else" ":" Newline+ @@*`
}

type BreakStmt struct {
	Break string `"break"`
}

type VarDeclStmt struct {
	Type  string `@Ident`
	Name  string `@Ident`
	Value string `"=" @(Number | String | Ident)`
}

type VarAssignStmt struct {
	Name  string `@Ident`
	Value string `"=" @(Number | String | Ident | Expression)`
}

type ListDeclStmt struct {
	Type     string   `"list" "[" @Ident "]"`
	Name     string   `@Ident`
	Elements []string `"=" "[" ( @(Number | String) ( "," @(Number | String) )* )? "]"`
}

type ClassDeclStmt struct {
	Name        string            `"class" @Ident ":"`
	Constructor *ConstructorStmt  `@@?`
	Methods     []*MethodDeclStmt `@@*`
}

type ConstructorStmt struct {
	Fields []*Statement `"init" ":" Newline+ @@*`
}

type MethodParameter struct {
	Type string `@Ident`
	Name string `@Ident`
}

type MethodDeclStmt struct {
	Name       string             `"fn" @Ident`
	Parameters []*MethodParameter `"(" ( @@ ( "," @@ )* )? ")"`
	ReturnType string             `"->" @(Ident | "void") ":"`
	Body       []*Statement       `Newline+ @@*`
}

type MethodCallStmt struct {
	Object string   `@Ident`
	Method string   `"." @Ident`
	Args   []string `"(" ( @(Ident | Number | String) ( "," @(Ident | Number | String) )* )? ")"`
}

type ObjectDeclStmt struct {
	Type string   `@Ident`
	Name string   `@Ident`
	Args []string `"=" "new" @Ident "(" ( @(Ident | Number | String) ( "," @(Ident | Number | String) )* )? ")"`
}

type Expression struct {
	Left     string `@(Ident | Number)`
	Operator string `@("+" | "-" | "*" | "/" | "%")`
	Right    string `@(Ident | Number)`
}

type IndexAccess struct {
	ListName string `@Ident`
	Index    string `"[" @(Number | Ident) "]"`
}

func parseWithIndentation(input string) (*Program, error) {
	lines := strings.Split(input, "\n")
	statements, err := parseStatements(lines, 0, 0)
	if err != nil {
		return nil, err
	}
	return &Program{Statements: statements}, nil
}

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

func parseStatement(lines []string, lineNum, currentIndent int) (*Statement, int, error) {
	line := strings.TrimSpace(lines[lineNum])
	parts := strings.Fields(line)

	if len(parts) == 0 {
		return nil, lineNum + 1, fmt.Errorf("empty statement at line %d", lineNum+1)
	}

	switch parts[0] {
	case "return":
		if len(parts) < 2 {
			return nil, lineNum + 1, fmt.Errorf("return statement requires a value at line %d", lineNum+1)
		}
		value := strings.Join(parts[1:], " ")
		if strings.HasPrefix(value, "this.") {
			fieldName := value[5:]
			value = "this->" + fieldName
		}
		return &Statement{Return: &ReturnStmt{Value: value}}, lineNum + 1, nil

	case "class":
		return parseClassStatement(lines, lineNum, currentIndent)

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
				varList := strings.Split(varPart, ",")
				for _, v := range varList {
					variables = append(variables, strings.TrimSpace(v))
				}
			}

			return &Statement{Print: &PrintStmt{Format: formatPart, Variables: variables}}, lineNum + 1, nil
		} else {
			str := strings.Join(parts[1:], " ")
			if strings.HasPrefix(str, "\"") && strings.HasSuffix(str, "\"") {
				str = str[1 : len(str)-1]
			}
			return &Statement{Print: &PrintStmt{Print: str}}, lineNum + 1, nil
		}

	case "sleep":
		if len(parts) < 2 {
			return nil, lineNum + 1, fmt.Errorf("sleep statement requires a number at line %d", lineNum+1)
		}
		return &Statement{Sleep: &SleepStmt{Duration: parts[1]}}, lineNum + 1, nil

	case "break":
		return &Statement{Break: &BreakStmt{Break: "break"}}, lineNum + 1, nil

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

	case "for":
		if len(parts) < 6 || parts[2] != "=" || parts[4] != "to" || !strings.HasSuffix(line, ":") {
			return nil, lineNum + 1, fmt.Errorf("for statement format error at line %d", lineNum+1)
		}

		varName := parts[1]
		start := parts[3]
		end := parts[5]
		if strings.HasSuffix(end, ":") {
			end = end[:len(end)-1]
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

	case "reassign":
		if len(parts) < 4 || parts[2] != "=" {
			return nil, lineNum + 1, fmt.Errorf("reassign statement format error at line %d (expected: reassign var = value)", lineNum+1)
		}

		varName := parts[1]
		value := strings.Join(parts[3:], " ")

		if strings.Contains(varName, "[") && strings.Contains(varName, "]") {
			value = handleIndexAssignment(line, varName, value)
		} else if len(parts) >= 6 && isOperator(parts[4]) {
			left := parts[3]
			operator := parts[4]
			right := parts[5]
			value = left + " " + operator + " " + right
		} else if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
			value = value[1 : len(value)-1]
		}

		return &Statement{VarAssign: &VarAssignStmt{Name: varName, Value: value}}, lineNum + 1, nil

	case "elif":
		return nil, lineNum + 1, fmt.Errorf("elif statement must follow an if statement at line %d", lineNum+1)

	case "else":
		return nil, lineNum + 1, fmt.Errorf("else statement must follow an if statement at line %d", lineNum+1)

	default:
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
						argsList := strings.Split(argsStr, ",")
						for _, arg := range argsList {
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
			argsStart := strings.Index(line, "(")
			argsEnd := strings.LastIndex(line, ")")

			var args []string
			if argsStart != -1 && argsEnd != -1 && argsEnd > argsStart+1 {
				argsStr := strings.TrimSpace(line[argsStart+1 : argsEnd])
				if argsStr != "" {
					argsList := strings.Split(argsStr, ",")
					for _, arg := range argsList {
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
			if strings.Contains(value, ".") && strings.Contains(value, "(") && strings.Contains(value, ")") {
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

			if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
				value = value[1 : len(value)-1]
			}

			return &Statement{VarDecl: &VarDeclStmt{Type: varType, Name: varName, Value: value}}, lineNum + 1, nil
		}

		return nil, lineNum + 1, fmt.Errorf("unknown statement type '%s' at line %d", parts[0], lineNum+1)
	}
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

		if strings.HasPrefix(trimmed, "init:") {
			initBodyIndent := expectedBodyIndent + 4
			initStartLine := nextLine + 1
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

			constructor = &ConstructorStmt{Fields: initBody}
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

var vdt = []string{"int", "float", "double", "char", "string", "bool"}

func isValidType(s string) bool {
	return slices.Contains(vdt, s)
}

func isOperator(s string) bool {
	operators := []string{"+", "-", "*", "/", "%"}
	return slices.Contains(operators, s)
}

func getIndentation(line string) int {
	indent := 0
	shouldBreak := false
	for _, char := range line {
		switch char {
		case ' ':
			indent++
		case '\t':
			indent += 4
		default:
			shouldBreak = true
		}
		if shouldBreak {
			break
		}
	}
	return indent
}

func findEndOfBlock(lines []string, startLine, blockIndent int) int {
	for i := startLine; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if getIndentation(line) < blockIndent {
			return i
		}
	}
	return len(lines)
}
