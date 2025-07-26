package main

import (
	"fmt"
	"log"
	"os"
	"path"
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
	{Name: "Indent", Pattern: `^[ \t]+`}, // Indentation at start of line
	{Name: "Whitespace", Pattern: `[ \t\r]+`},
	{Name: "Assign", Pattern: `=`},
	{Name: "To", Pattern: `to`},
})

type Program struct {
	Statements []*Statement `{ @@ ( Newline+ @@ )* Newline* }`
}

type Statement struct {
	Print *PrintStmt `  @@`
	Sleep *SleepStmt `| @@`
	While *WhileStmt `| @@`
	For   *ForStmt   `| @@`
}

type PrintStmt struct {
	Print string `"print" @String`
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

// Custom parser that handles indentation
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

		// Skip empty lines and comments
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			i++
			continue
		}

		// Calculate indentation
		indent := getIndentation(line)

		// If indentation is less than expected, we've reached the end of this block
		if indent < expectedIndent {
			break
		}

		// If indentation is greater than expected, this is an error (unless we're in a block)
		if indent > expectedIndent {
			return nil, fmt.Errorf("unexpected indentation at line %d", i+1)
		}

		// Parse the statement
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
	case "print":
		if len(parts) < 2 {
			return nil, lineNum + 1, fmt.Errorf("print statement requires a string at line %d", lineNum+1)
		}
		// Extract string (remove quotes)
		str := strings.Join(parts[1:], " ")
		if strings.HasPrefix(str, "\"") && strings.HasSuffix(str, "\"") {
			str = str[1 : len(str)-1]
		}
		return &Statement{Print: &PrintStmt{Print: str}}, lineNum + 1, nil

	case "sleep":
		if len(parts) < 2 {
			return nil, lineNum + 1, fmt.Errorf("sleep statement requires a number at line %d", lineNum+1)
		}
		return &Statement{Sleep: &SleepStmt{Duration: parts[1]}}, lineNum + 1, nil

	case "while":
		if len(parts) < 2 || !strings.HasSuffix(line, ":") {
			return nil, lineNum + 1, fmt.Errorf("while statement format error at line %d", lineNum+1)
		}
		// Extract condition (everything between "while" and ":")
		colonIndex := strings.LastIndex(line, ":")
		conditionPart := strings.TrimSpace(line[5:colonIndex]) // Skip "while" (5 chars)
		condition := conditionPart

		// Parse body with increased indentation
		body, err := parseStatements(lines, lineNum+1, currentIndent+4)
		if err != nil {
			return nil, lineNum + 1, err
		}

		// Find where the body ends
		nextLine := findEndOfBlock(lines, lineNum+1, currentIndent+4)

		return &Statement{While: &WhileStmt{Condition: condition, Body: body}}, nextLine, nil

	case "for":
		// Parse "for i = 0 to 3:"
		if len(parts) < 6 || parts[2] != "=" || parts[4] != "to" || !strings.HasSuffix(line, ":") {
			return nil, lineNum + 1, fmt.Errorf("for statement format error at line %d", lineNum+1)
		}

		varName := parts[1]
		start := parts[3]
		end := parts[5]
		if strings.HasSuffix(end, ":") {
			end = end[:len(end)-1]
		}

		// Parse body with increased indentation
		body, err := parseStatements(lines, lineNum+1, currentIndent+4)
		if err != nil {
			return nil, lineNum + 1, err
		}

		// Find where the body ends
		nextLine := findEndOfBlock(lines, lineNum+1, currentIndent+4)

		return &Statement{For: &ForStmt{Var: varName, Start: start, End: end, Body: body}}, nextLine, nil

	default:
		return nil, lineNum + 1, fmt.Errorf("unknown statement type '%s' at line %d", parts[0], lineNum+1)
	}
}

func getIndentation(line string) int {
	indent := 0
	for _, char := range line {
		if char == ' ' {
			indent++
		} else if char == '\t' {
			indent += 4 // Treat tab as 4 spaces
		} else {
			break
		}
	}
	return indent
}

func findEndOfBlock(lines []string, startLine, blockIndent int) int {
	for i := startLine; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// If we find a line with less indentation, the block has ended
		if getIndentation(line) < blockIndent {
			return i
		}
	}
	return len(lines)
}

func renderC(program *Program) string {
	var b strings.Builder
	b.WriteString(`#include <stdio.h>
#include <unistd.h>

int main() {
`)
	renderStatements(&b, program.Statements, "    ")
	b.WriteString("    return 0;\n")
	b.WriteString("}\n")
	return b.String()
}

func renderStatements(b *strings.Builder, stmts []*Statement, indent string) {
	for _, stmt := range stmts {
		switch {
		case stmt.Print != nil:
			fmt.Fprintf(b, "%sprintf(\"%s\\n\");\n", indent, stmt.Print.Print)
		case stmt.Sleep != nil:
			fmt.Fprintf(b, "%ssleep(%s);\n", indent, stmt.Sleep.Duration)
		case stmt.While != nil:
			fmt.Fprintf(b, "%swhile (%s) {\n", indent, stmt.While.Condition)
			renderStatements(b, stmt.While.Body, indent+"    ")
			fmt.Fprintf(b, "%s}\n", indent)
		case stmt.For != nil:
			varName := stmt.For.Var
			start := stmt.For.Start
			end := stmt.For.End
			fmt.Fprintf(b, "%sfor (int %s = %s; %s <= %s; %s++) {\n", indent, varName, start, varName, end, varName)
			renderStatements(b, stmt.For.Body, indent+"    ")
			fmt.Fprintf(b, "%s}\n", indent)
		}
	}
}

func main() {
	var input string
	if len(os.Args) > 1 {
		var (
			wd, _     = os.Getwd()
			ptf       = path.Join(wd, os.Args[1])
			data, err = os.ReadFile(ptf)
		)
		if err != nil {
			log.Fatal("Could not find file.")
		}
		input = string(data)
	}

	program, err := parseWithIndentation(input)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(renderC(program))
}
