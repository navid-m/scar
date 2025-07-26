package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

var dslLexer = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Comment", Pattern: `#[^\n]*`},
	{Name: "Number", Pattern: `\d+`},
	{Name: "String", Pattern: `"[^"]*"`},
	{Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`},
	{Name: "Colon", Pattern: `:`},
	{Name: "Newline", Pattern: `\n`},
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
	Body      []*Statement `{ @@ Newline+ }`
}

type ForStmt struct {
	Var   string       `"for" @Ident`
	Start string       `"=" @Number`
	End   string       `"to" @Number ":" Newline+`
	Body  []*Statement `{ @@ Newline+ }`
}

var parser = participle.MustBuild[Program](
	participle.Lexer(dslLexer),
	participle.Elide("Whitespace", "Comment"),
	participle.Unquote("String"),
)

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
			fmt.Fprintf(b, "%sfprintf(stdout, \"%s\\n\");\n", indent, stmt.Print.Print)
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

	program, err := parser.ParseString("", input)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(renderC(program))
}
