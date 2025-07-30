package lexer

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
)

var dslLexer = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Comment", Pattern: `#[^\n]*`},
	{Name: "Number", Pattern: `\d+`},
	{Name: "String", Pattern: `"[^"]*"`},
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
	{Name: "Pub", Pattern: `pub`},
	{Name: "Import", Pattern: `import`},
	{Name: "Var", Pattern: `var`},
	{Name: "Print", Pattern: `print`},
	{Name: "Sleep", Pattern: `sleep`},
	{Name: "While", Pattern: `while`},
	{Name: "For", Pattern: `for`},
	{Name: "Return", Pattern: `return`},
	{Name: "List", Pattern: `list`},
	{Name: "Plus", Pattern: `\+`},
	{Name: "Minus", Pattern: `-`},
	{Name: "Multiply", Pattern: `\*`},
	{Name: "Divide", Pattern: `/`},
	{Name: "Modulo", Pattern: `%`},
	{Name: "Pipe", Pattern: `\|`},
	{Name: "Less", Pattern: `<`},
	{Name: "Greater", Pattern: `>`},
	{Name: "LessEqual", Pattern: `<=`},
	{Name: "GreaterEqual", Pattern: `>=`},
	{Name: "Equal", Pattern: `==`},
	{Name: "NotEqual", Pattern: `!=`},
	{Name: "Not", Pattern: `!`},
	{Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`},
})

// Token represents a lexical token
type Token struct {
	Type  string
	Value string
	Line  int
	Col   int
}

// TokenStream represents a stream of tokens with current position
type TokenStream struct {
	tokens   []Token
	position int
}

// NewTokenStream creates a new token stream from input text
func NewTokenStream(input string) (*TokenStream, error) {
	tokens, err := tokenize(input)
	if err != nil {
		return nil, err
	}
	return &TokenStream{tokens: tokens, position: 0}, nil
}

// Current returns the current token
func (ts *TokenStream) Current() *Token {
	if ts.position >= len(ts.tokens) {
		return nil
	}
	return &ts.tokens[ts.position]
}

// Next advances to the next token
func (ts *TokenStream) Next() *Token {
	if ts.position < len(ts.tokens) {
		ts.position++
	}
	return ts.Current()
}

// Peek returns the token at offset positions ahead without advancing
func (ts *TokenStream) Peek(offset int) *Token {
	pos := ts.position + offset
	if pos >= len(ts.tokens) {
		return nil
	}
	return &ts.tokens[pos]
}

// Consume advances if the current token matches the expected type
func (ts *TokenStream) Consume(tokenType string) (*Token, error) {
	current := ts.Current()
	if current == nil {
		return nil, fmt.Errorf("unexpected end of input, expected %s", tokenType)
	}
	if current.Type != tokenType {
		return nil, fmt.Errorf("expected %s, got %s at line %d", tokenType, current.Type, current.Line)
	}
	ts.Next()
	return current, nil
}

// Match checks if current token matches any of the given types
func (ts *TokenStream) Match(tokenTypes ...string) bool {
	current := ts.Current()
	if current == nil {
		return false
	}
	for _, tokenType := range tokenTypes {
		if current.Type == tokenType {
			return true
		}
	}
	return false
}

// SkipWhitespaceAndComments skips whitespace and comment tokens
func (ts *TokenStream) SkipWhitespaceAndComments() {
	for ts.Current() != nil && (ts.Current().Type == "Whitespace" || ts.Current().Type == "Comment" || ts.Current().Type == "Indent") {
		ts.Next()
	}
}

// IsAtEnd checks if we're at the end of the token stream
func (ts *TokenStream) IsAtEnd() bool {
	return ts.position >= len(ts.tokens)
}

// tokenTypeToName converts a lexer.TokenType to its corresponding rule name.
func tokenTypeToName(tt lexer.TokenType) string {
	for name, t := range dslLexer.Symbols() {
		if t == tt {
			return name
		}
	}
	// Fallback to the numeric value if no symbol found (should not happen)
	return fmt.Sprintf("%d", tt)
}

func tokenize(input string) ([]Token, error) {
	lex, err := dslLexer.LexString("", input)
	if err != nil {
		return nil, err
	}

	var tokens []Token
	for {
		token, err := lex.Next()
		if err != nil {
			return nil, err
		}
		if token.EOF() {
			break
		}

		tokens = append(tokens, Token{
			Type:  tokenTypeToName(token.Type),
			Value: token.Value,
			Line:  token.Pos.Line,
			Col:   token.Pos.Column,
		})
	}

	return tokens, nil
}

// Import and module system types
type ImportStmt struct {
	Module string
}

type ModuleInfo struct {
	Name          string
	FilePath      string
	PublicVars    map[string]*VarDeclStmt
	PublicClasses map[string]*ClassDeclStmt
	PublicFuncs   map[string]*MethodDeclStmt
}

type Program struct {
	Imports    []*ImportStmt
	Statements []*Statement
}

type Statement struct {
	Import            *ImportStmt
	Print             *PrintStmt
	Sleep             *SleepStmt
	While             *WhileStmt
	For               *ForStmt
	If                *IfStmt
	Break             *BreakStmt
	VarDecl           *VarDeclStmt
	VarAssign         *VarAssignStmt
	ListDecl          *ListDeclStmt
	ClassDecl         *ClassDeclStmt
	MethodCall        *MethodCallStmt
	ObjectDecl        *ObjectDeclStmt
	Return            *ReturnStmt
	VarDeclMethodCall *VarDeclMethodCallStmt
	VarDeclInferred   *VarDeclInferredStmt
	PubVarDecl        *PubVarDeclStmt
	PubClassDecl      *PubClassDeclStmt
	TopLevelFuncDecl  *TopLevelFuncDeclStmt
	FunctionCall      *FunctionCallStmt
}

type PubVarDeclStmt struct {
	Type  string
	Name  string
	Value string
}

type PubClassDeclStmt struct {
	Name        string
	Constructor *ConstructorStmt
	Methods     []*MethodDeclStmt
}

type VarDeclMethodCallStmt struct {
	Type   string
	Name   string
	Object string
	Method string
	Args   []string
}

type VarDeclInferredStmt struct {
	Name  string
	Value string
}

type ReturnStmt struct {
	Value string
}

type PrintStmt struct {
	Print     string
	Format    string
	Variables []string
}

type SleepStmt struct {
	Duration string
}

type WhileStmt struct {
	Condition string
	Body      []*Statement
}

type ForStmt struct {
	Var   string
	Start string
	End   string
	Body  []*Statement
}

type IfStmt struct {
	Condition string
	Body      []*Statement
	ElseIfs   []*ElifStmt
	Else      *ElseStmt
}

type ElifStmt struct {
	Condition string
	Body      []*Statement
}

type ElseStmt struct {
	Body []*Statement
}

type BreakStmt struct {
	Break string
}

type VarDeclStmt struct {
	Type  string
	Name  string
	Value string
}

type VarAssignStmt struct {
	Name  string
	Value string
}

type ListDeclStmt struct {
	Type     string
	Name     string
	Elements []string
}

type ClassDeclStmt struct {
	Name        string
	Constructor *ConstructorStmt
	Methods     []*MethodDeclStmt
}

type ConstructorStmt struct {
	Parameters []*MethodParameter
	Fields     []*Statement
}

type MethodParameter struct {
	Type string
	Name string
}

type MethodDeclStmt struct {
	Name       string
	Parameters []*MethodParameter
	ReturnType string
	Body       []*Statement
}

type MethodCallStmt struct {
	Object string
	Method string
	Args   []string
}

type ObjectDeclStmt struct {
	Type string
	Name string
	Args []string
}

type Expression struct {
	Left     string
	Operator string
	Right    string
}

type IndexAccess struct {
	ListName string
	Index    string
}

type TopLevelFuncDeclStmt struct {
	Name       string
	Parameters []*MethodParameter
	ReturnType string
	Body       []*Statement
}

type FunctionCallStmt struct {
	Name string
	Args []string
}

var LoadedModules = make(map[string]*ModuleInfo)

// Parser represents the main parser
type Parser struct {
	tokens *TokenStream
}

// NewParser creates a new parser with the given input
func NewParser(input string) (*Parser, error) {
	tokens, err := NewTokenStream(input)
	if err != nil {
		return nil, err
	}
	return &Parser{tokens: tokens}, nil
}

// ParseProgram parses the entire program
func (p *Parser) ParseProgram() (*Program, error) {
	var imports []*ImportStmt
	var statements []*Statement

	for !p.tokens.IsAtEnd() {
		p.tokens.SkipWhitespaceAndComments()
		if p.tokens.IsAtEnd() {
			break
		}

		// Skip newlines at top level
		if p.tokens.Match("Newline") {
			p.tokens.Next()
			continue
		}

		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}

		if stmt.Import != nil {
			imports = append(imports, stmt.Import)
		} else {
			statements = append(statements, stmt)
		}

		// Skip trailing newlines
		for p.tokens.Match("Newline") {
			p.tokens.Next()
		}
	}

	return &Program{Imports: imports, Statements: statements}, nil
}

// parseStatement parses a single statement
func (p *Parser) parseStatement() (*Statement, error) {
	p.tokens.SkipWhitespaceAndComments()

	if p.tokens.IsAtEnd() {
		return nil, fmt.Errorf("unexpected end of input")
	}

	current := p.tokens.Current()
	switch current.Type {
	case "Import":
		return p.parseImport()
	case "Print":
		return p.parsePrint()
	case "Sleep":
		return p.parseSleep()
	case "While":
		return p.parseWhile()
	case "For":
		return p.parseFor()
	case "If":
		return p.parseIf()
	case "Break":
		return p.parseBreak()
	case "Var":
		return p.parseVarInferred()
	case "Pub":
		return p.parsePub()
	case "Return":
		return p.parseReturn()
	case "Class":
		return p.parseClass()
	case "Fn":
		return p.parseTopLevelFunction()
	case "Reassign":
		return p.parseVarAssign()
	case "List":
		return p.parseListDecl()
	case "Ident":
		return p.parseIdentStatement()
	default:
		return nil, fmt.Errorf("unexpected token %s at line %d", current.Type, current.Line)
	}
}

func (p *Parser) parseImport() (*Statement, error) {
	_, err := p.tokens.Consume("Import")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	moduleToken, err := p.tokens.Consume("String")
	if err != nil {
		return nil, err
	}

	moduleName := strings.Trim(moduleToken.Value, "\"")
	return &Statement{Import: &ImportStmt{Module: moduleName}}, nil
}

func (p *Parser) parsePrint() (*Statement, error) {
	_, err := p.tokens.Consume("Print")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()

	// Handle empty print statement - just print a newline
	if p.tokens.Match("Newline") || p.tokens.IsAtEnd() {
		return &Statement{Print: &PrintStmt{Print: ""}}, nil
	}

	// Check for string literal
	if p.tokens.Match("String") {
		stringToken := p.tokens.Current()
		p.tokens.Next()
		p.tokens.SkipWhitespaceAndComments()

		// Check for format string with variables (using pipe or comma)
		if p.tokens.Match("Pipe") || p.tokens.Match("Comma") {
			// Consume the pipe or comma
			delimiter := p.tokens.Current().Type
			p.tokens.Next()
			p.tokens.SkipWhitespaceAndComments()

			formatStr := strings.Trim(stringToken.Value, "\"")
			var variables []string

			// Parse variables until end of line or statement
			for !p.tokens.IsAtEnd() && !p.tokens.Match("Newline") {
				if p.tokens.Match("Comma") && delimiter == "Comma" {
					p.tokens.Next()
					p.tokens.SkipWhitespaceAndComments()
				}

				if p.tokens.Match("Ident") {
					varToken := p.tokens.Current()
					variables = append(variables, varToken.Value)
					p.tokens.Next()
					p.tokens.SkipWhitespaceAndComments()
				} else if p.tokens.Match("Number", "String") {
					// Allow direct values in print statements
					valToken := p.tokens.Current()
					variables = append(variables, valToken.Value)
					p.tokens.Next()
					p.tokens.SkipWhitespaceAndComments()
				} else {
					break
				}

				// After first variable, only allow comma if that's our delimiter
				if p.tokens.Match("Comma") && delimiter == "Pipe" {
					break
				}
			}

			return &Statement{Print: &PrintStmt{Format: formatStr, Variables: variables}}, nil
		}

		// Simple print statement with just a string
		printStr := strings.Trim(stringToken.Value, "\"")
		return &Statement{Print: &PrintStmt{Print: printStr}}, nil
	}

	// Handle print with just variables (no string literal)
	var variables []string
	for !p.tokens.IsAtEnd() && !p.tokens.Match("Newline") {
		if p.tokens.Match("Comma") {
			p.tokens.Next()
			p.tokens.SkipWhitespaceAndComments()
		}

		if p.tokens.Match("Ident", "Number", "String") {
			token := p.tokens.Current()
			variables = append(variables, token.Value)
			p.tokens.Next()
			p.tokens.SkipWhitespaceAndComments()
		} else {
			break
		}
	}

	if len(variables) > 0 {
		// For multiple variables, join with spaces in the format string
		formatStr := strings.Repeat("%v ", len(variables))
		formatStr = strings.TrimSpace(formatStr)
		return &Statement{Print: &PrintStmt{Format: formatStr, Variables: variables}}, nil
	}

	// If we get here, it's an empty print statement
	return &Statement{Print: &PrintStmt{Print: ""}}, nil
}

func (p *Parser) parseSleep() (*Statement, error) {
	_, err := p.tokens.Consume("Sleep")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	durationToken, err := p.tokens.Consume("Number")
	if err != nil {
		return nil, err
	}

	return &Statement{Sleep: &SleepStmt{Duration: durationToken.Value}}, nil
}

func (p *Parser) parseWhile() (*Statement, error) {
	_, err := p.tokens.Consume("While")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	condition, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("Colon")
	if err != nil {
		return nil, err
	}

	// Ensure we have a newline after the colon
	p.tokens.SkipWhitespaceAndComments()
	if p.tokens.Match("Newline") {
		p.tokens.Next()
	}

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	return &Statement{While: &WhileStmt{Condition: condition, Body: body}}, nil
}

func (p *Parser) parseFor() (*Statement, error) {
	_, err := p.tokens.Consume("For")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	varToken, err := p.tokens.Consume("Ident")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("Assign")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	startToken, err := p.tokens.Consume("Number")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("To")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	endToken, err := p.tokens.Consume("Number")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("Colon")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	if p.tokens.Match("Newline") {
		p.tokens.Next()
	}

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	return &Statement{For: &ForStmt{
		Var:   varToken.Value,
		Start: startToken.Value,
		End:   endToken.Value,
		Body:  body,
	}}, nil
}

func (p *Parser) parseIf() (*Statement, error) {
	_, err := p.tokens.Consume("If")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	condition, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("Colon")
	if err != nil {
		return nil, err
	}

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	var elseIfs []*ElifStmt
	for p.tokens.Match("Elif") {
		elifStmt, err := p.parseElif()
		if err != nil {
			return nil, err
		}
		elseIfs = append(elseIfs, elifStmt)
	}

	var elseStmt *ElseStmt
	if p.tokens.Match("Else") {
		elseStmt, err = p.parseElse()
		if err != nil {
			return nil, err
		}
	}

	return &Statement{If: &IfStmt{
		Condition: condition,
		Body:      body,
		ElseIfs:   elseIfs,
		Else:      elseStmt,
	}}, nil
}

func (p *Parser) parseElif() (*ElifStmt, error) {
	_, err := p.tokens.Consume("Elif")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	condition, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("Colon")
	if err != nil {
		return nil, err
	}

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	return &ElifStmt{Condition: condition, Body: body}, nil
}

func (p *Parser) parseElse() (*ElseStmt, error) {
	_, err := p.tokens.Consume("Else")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("Colon")
	if err != nil {
		return nil, err
	}

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	return &ElseStmt{Body: body}, nil
}

func (p *Parser) parseBreak() (*Statement, error) {
	_, err := p.tokens.Consume("Break")
	if err != nil {
		return nil, err
	}

	return &Statement{Break: &BreakStmt{Break: "break"}}, nil
}

func (p *Parser) parseVarInferred() (*Statement, error) {
	_, err := p.tokens.Consume("Var")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	nameToken, err := p.tokens.Consume("Ident")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("Assign")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	value, err := p.parseValue()
	if err != nil {
		return nil, err
	}

	return &Statement{VarDeclInferred: &VarDeclInferredStmt{
		Name:  nameToken.Value,
		Value: value,
	}}, nil
}

func (p *Parser) parsePub() (*Statement, error) {
	_, err := p.tokens.Consume("Pub")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	if p.tokens.Match("Class") {
		return p.parsePubClass()
	}

	// Parse pub var declaration
	typeToken, err := p.tokens.Consume("Ident")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	nameToken, err := p.tokens.Consume("Ident")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("Assign")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	value, err := p.parseValue()
	if err != nil {
		return nil, err
	}

	return &Statement{PubVarDecl: &PubVarDeclStmt{
		Type:  typeToken.Value,
		Name:  nameToken.Value,
		Value: value,
	}}, nil
}

func (p *Parser) parseReturn() (*Statement, error) {
	_, err := p.tokens.Consume("Return")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	value, err := p.parseValue()
	if err != nil {
		return nil, err
	}

	// Handle this.field references
	if strings.HasPrefix(value, "this.") {
		fieldName := value[5:]
		value = "this->" + fieldName
	}

	return &Statement{Return: &ReturnStmt{Value: value}}, nil
}

func (p *Parser) parseClass() (*Statement, error) {
	_, err := p.tokens.Consume("Class")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	nameToken, err := p.tokens.Consume("Ident")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("Colon")
	if err != nil {
		return nil, err
	}

	var constructor *ConstructorStmt
	var methods []*MethodDeclStmt

	// Parse class body
	for !p.tokens.IsAtEnd() {
		p.tokens.SkipWhitespaceAndComments()
		if p.tokens.IsAtEnd() {
			break
		}

		// Check for indentation (simplified - in a real implementation you'd track indentation levels)
		if p.tokens.Match("Newline") {
			p.tokens.Next()
			continue
		}

		if p.tokens.Match("Init") {
			constructor, err = p.parseConstructor()
			if err != nil {
				return nil, err
			}
		} else if p.tokens.Match("Fn") {
			method, err := p.parseMethod()
			if err != nil {
				return nil, err
			}
			methods = append(methods, method)
		} else {
			// End of class body
			break
		}
	}

	return &Statement{ClassDecl: &ClassDeclStmt{
		Name:        nameToken.Value,
		Constructor: constructor,
		Methods:     methods,
	}}, nil
}

func (p *Parser) parsePubClass() (*Statement, error) {
	_, err := p.tokens.Consume("Class")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	nameToken, err := p.tokens.Consume("Ident")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("Colon")
	if err != nil {
		return nil, err
	}

	var constructor *ConstructorStmt
	var methods []*MethodDeclStmt

	// Parse class body (similar to parseClass)
	for !p.tokens.IsAtEnd() {
		p.tokens.SkipWhitespaceAndComments()
		if p.tokens.IsAtEnd() {
			break
		}

		if p.tokens.Match("Newline") {
			p.tokens.Next()
			continue
		}

		if p.tokens.Match("Init") {
			constructor, err = p.parseConstructor()
			if err != nil {
				return nil, err
			}
		} else if p.tokens.Match("Fn") {
			method, err := p.parseMethod()
			if err != nil {
				return nil, err
			}
			methods = append(methods, method)
		} else {
			break
		}
	}

	return &Statement{PubClassDecl: &PubClassDeclStmt{
		Name:        nameToken.Value,
		Constructor: constructor,
		Methods:     methods,
	}}, nil
}

func (p *Parser) parseConstructor() (*ConstructorStmt, error) {
	_, err := p.tokens.Consume("Init")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("LeftParen")
	if err != nil {
		return nil, err
	}

	parameters, err := p.parseParameterList()
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("RightParen")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("Colon")
	if err != nil {
		return nil, err
	}

	fields, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	return &ConstructorStmt{
		Parameters: parameters,
		Fields:     fields,
	}, nil
}

func (p *Parser) parseMethod() (*MethodDeclStmt, error) {
	_, err := p.tokens.Consume("Fn")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	nameToken, err := p.tokens.Consume("Ident")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("LeftParen")
	if err != nil {
		return nil, err
	}

	parameters, err := p.parseParameterList()
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("RightParen")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	returnType := "void"
	if p.tokens.Match("Arrow") {
		p.tokens.Next()
		p.tokens.SkipWhitespaceAndComments()
		if p.tokens.Match("Ident", "Void") {
			returnTypeToken := p.tokens.Current()
			returnType = returnTypeToken.Value
			p.tokens.Next()
		}
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("Colon")
	if err != nil {
		return nil, err
	}

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	return &MethodDeclStmt{
		Name:       nameToken.Value,
		Parameters: parameters,
		ReturnType: returnType,
		Body:       body,
	}, nil
}

func (p *Parser) parseTopLevelFunction() (*Statement, error) {
	_, err := p.tokens.Consume("Fn")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	nameToken, err := p.tokens.Consume("Ident")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("LeftParen")
	if err != nil {
		return nil, err
	}

	parameters, err := p.parseParameterList()
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("RightParen")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	returnType := "void"
	if p.tokens.Match("Arrow") {
		p.tokens.Next()
		p.tokens.SkipWhitespaceAndComments()
		if p.tokens.Match("Ident", "Void") {
			returnTypeToken := p.tokens.Current()
			returnType = returnTypeToken.Value
			p.tokens.Next()
		}
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("Colon")
	if err != nil {
		return nil, err
	}

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	return &Statement{TopLevelFuncDecl: &TopLevelFuncDeclStmt{
		Name:       nameToken.Value,
		Parameters: parameters,
		ReturnType: returnType,
		Body:       body,
	}}, nil
}

func (p *Parser) parseVarAssign() (*Statement, error) {
	_, err := p.tokens.Consume("Reassign")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	nameToken, err := p.tokens.Consume("Ident")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("Assign")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	value, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	return &Statement{VarAssign: &VarAssignStmt{
		Name:  nameToken.Value,
		Value: value,
	}}, nil
}

func (p *Parser) parseListDecl() (*Statement, error) {
	_, err := p.tokens.Consume("List")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("LeftBracket")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	typeToken, err := p.tokens.Consume("Ident")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("RightBracket")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	nameToken, err := p.tokens.Consume("Ident")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("Assign")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("LeftBracket")
	if err != nil {
		return nil, err
	}

	var elements []string
	p.tokens.SkipWhitespaceAndComments()
	if !p.tokens.Match("RightBracket") {
		for {
			p.tokens.SkipWhitespaceAndComments()
			if p.tokens.Match("String", "Number") {
				valueToken := p.tokens.Current()
				value := valueToken.Value
				if valueToken.Type == "String" {
					value = strings.Trim(value, "\"")
				}
				elements = append(elements, value)
				p.tokens.Next()
			} else {
				return nil, fmt.Errorf("expected string or number in list")
			}

			p.tokens.SkipWhitespaceAndComments()
			if p.tokens.Match("Comma") {
				p.tokens.Next()
				continue
			} else {
				break
			}
		}
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("RightBracket")
	if err != nil {
		return nil, err
	}

	return &Statement{ListDecl: &ListDeclStmt{
		Type:     typeToken.Value,
		Name:     nameToken.Value,
		Elements: elements,
	}}, nil
}

func (p *Parser) parseIdentStatement() (*Statement, error) {
	// This handles various statements that start with an identifier
	first := p.tokens.Current()

	// Look ahead to determine what kind of statement this is
	if p.tokens.Peek(1) != nil && p.tokens.Peek(1).Type == "Ident" &&
		p.tokens.Peek(2) != nil && p.tokens.Peek(2).Type == "Assign" {
		// Type name = value (variable declaration)
		return p.parseVarDecl()
	}

	if p.tokens.Peek(1) != nil && p.tokens.Peek(1).Type == "Assign" {
		// name = value (assignment or this.field assignment)
		if strings.HasPrefix(first.Value, "this.") {
			return p.parseThisFieldAssign()
		}
		return p.parseAssignment()
	}

	if p.tokens.Peek(1) != nil && p.tokens.Peek(1).Type == "Dot" {
		// object.method() (method call)
		return p.parseMethodCall()
	}

	if p.tokens.Peek(1) != nil && p.tokens.Peek(1).Type == "LeftParen" {
		// function() (function call)
		return p.parseFunctionCall()
	}

	// Check for object declaration: Type name = new ClassName()
	if p.tokens.Peek(1) != nil && p.tokens.Peek(1).Type == "Ident" &&
		p.tokens.Peek(2) != nil && p.tokens.Peek(2).Type == "Assign" &&
		p.tokens.Peek(3) != nil && p.tokens.Peek(3).Type == "New" {
		return p.parseObjectDecl()
	}

	return nil, fmt.Errorf("unexpected identifier statement at line %d", first.Line)
}

func (p *Parser) parseVarDecl() (*Statement, error) {
	typeToken, err := p.tokens.Consume("Ident")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	nameToken, err := p.tokens.Consume("Ident")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("Assign")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()

	// Check if this is a method call assignment
	if p.tokens.Match("Ident") && p.tokens.Peek(1) != nil && p.tokens.Peek(1).Type == "Dot" {
		objectToken := p.tokens.Current()
		p.tokens.Next()
		p.tokens.SkipWhitespaceAndComments()
		_, err = p.tokens.Consume("Dot")
		if err != nil {
			return nil, err
		}
		p.tokens.SkipWhitespaceAndComments()
		methodToken, err := p.tokens.Consume("Ident")
		if err != nil {
			return nil, err
		}
		p.tokens.SkipWhitespaceAndComments()
		_, err = p.tokens.Consume("LeftParen")
		if err != nil {
			return nil, err
		}

		args, err := p.parseArgumentList()
		if err != nil {
			return nil, err
		}

		p.tokens.SkipWhitespaceAndComments()
		_, err = p.tokens.Consume("RightParen")
		if err != nil {
			return nil, err
		}

		return &Statement{VarDeclMethodCall: &VarDeclMethodCallStmt{
			Type:   typeToken.Value,
			Name:   nameToken.Value,
			Object: objectToken.Value,
			Method: methodToken.Value,
			Args:   args,
		}}, nil
	}

	value, err := p.parseValue()
	if err != nil {
		return nil, err
	}

	return &Statement{VarDecl: &VarDeclStmt{
		Type:  typeToken.Value,
		Name:  nameToken.Value,
		Value: value,
	}}, nil
}

func (p *Parser) parseThisFieldAssign() (*Statement, error) {
	nameToken, err := p.tokens.Consume("Ident")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("Assign")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	value, err := p.parseValue()
	if err != nil {
		return nil, err
	}

	return &Statement{VarAssign: &VarAssignStmt{
		Name:  nameToken.Value,
		Value: value,
	}}, nil
}

func (p *Parser) parseAssignment() (*Statement, error) {
	nameToken, err := p.tokens.Consume("Ident")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("Assign")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	value, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	return &Statement{VarAssign: &VarAssignStmt{
		Name:  nameToken.Value,
		Value: value,
	}}, nil
}

func (p *Parser) parseMethodCall() (*Statement, error) {
	objectToken, err := p.tokens.Consume("Ident")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("Dot")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	methodToken, err := p.tokens.Consume("Ident")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("LeftParen")
	if err != nil {
		return nil, err
	}

	args, err := p.parseArgumentList()
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("RightParen")
	if err != nil {
		return nil, err
	}

	return &Statement{MethodCall: &MethodCallStmt{
		Object: objectToken.Value,
		Method: methodToken.Value,
		Args:   args,
	}}, nil
}

func (p *Parser) parseFunctionCall() (*Statement, error) {
	nameToken, err := p.tokens.Consume("Ident")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("LeftParen")
	if err != nil {
		return nil, err
	}

	args, err := p.parseArgumentList()
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("RightParen")
	if err != nil {
		return nil, err
	}

	return &Statement{FunctionCall: &FunctionCallStmt{
		Name: nameToken.Value,
		Args: args,
	}}, nil
}

func (p *Parser) parseObjectDecl() (*Statement, error) {
	typeToken, err := p.tokens.Consume("Ident")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	nameToken, err := p.tokens.Consume("Ident")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("Assign")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("New")
	if err != nil {
		return nil, err
	}

	p.tokens.SkipWhitespaceAndComments()
	classToken, err := p.tokens.Consume("Ident")
	if err != nil {
		return nil, err
	}

	// Handle module-qualified class names (module.ClassName)
	var args []string
	if p.tokens.Match("Dot") {
		p.tokens.Next()
		p.tokens.SkipWhitespaceAndComments()
		actualClassToken, err := p.tokens.Consume("Ident")
		if err != nil {
			return nil, err
		}
		args = []string{classToken.Value, actualClassToken.Value}
	} else {
		args = []string{classToken.Value}
	}

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("LeftParen")
	if err != nil {
		return nil, err
	}

	constructorArgs, err := p.parseArgumentList()
	if err != nil {
		return nil, err
	}
	args = append(args, constructorArgs...)

	p.tokens.SkipWhitespaceAndComments()
	_, err = p.tokens.Consume("RightParen")
	if err != nil {
		return nil, err
	}

	return &Statement{ObjectDecl: &ObjectDeclStmt{
		Type: typeToken.Value,
		Name: nameToken.Value,
		Args: args,
	}}, nil
}

// Helper functions

func (p *Parser) parseParameterList() ([]*MethodParameter, error) {
	var parameters []*MethodParameter

	p.tokens.SkipWhitespaceAndComments()
	if p.tokens.Match("RightParen") {
		return parameters, nil
	}

	for {
		p.tokens.SkipWhitespaceAndComments()
		typeToken, err := p.tokens.Consume("Ident")
		if err != nil {
			return nil, err
		}

		p.tokens.SkipWhitespaceAndComments()
		nameToken, err := p.tokens.Consume("Ident")
		if err != nil {
			// If only one token, assume it's the name and type is "int"
			parameters = append(parameters, &MethodParameter{
				Type: "int",
				Name: typeToken.Value,
			})
		} else {
			parameters = append(parameters, &MethodParameter{
				Type: typeToken.Value,
				Name: nameToken.Value,
			})
		}

		p.tokens.SkipWhitespaceAndComments()
		if p.tokens.Match("Comma") {
			p.tokens.Next()
			continue
		} else {
			break
		}
	}

	return parameters, nil
}

func (p *Parser) parseArgumentList() ([]string, error) {
	var args []string

	p.tokens.SkipWhitespaceAndComments()
	if p.tokens.Match("RightParen") {
		return args, nil
	}

	for {
		p.tokens.SkipWhitespaceAndComments()
		if p.tokens.Match("String", "Number", "Ident") {
			argToken := p.tokens.Current()
			args = append(args, argToken.Value)
			p.tokens.Next()
		} else {
			return nil, fmt.Errorf("expected argument")
		}

		p.tokens.SkipWhitespaceAndComments()
		if p.tokens.Match("Comma") {
			p.tokens.Next()
			continue
		} else {
			break
		}
	}

	return args, nil
}

func (p *Parser) parseBlock() ([]*Statement, error) {
	var statements []*Statement

	p.tokens.SkipWhitespaceAndComments()

	if p.tokens.Match("Newline") {
		p.tokens.Next()
	}

	// Get the base indentation level of the first line
	baseIndent := 0
	if p.tokens.Match("Indent") {
		baseIndent = len(p.tokens.Current().Value)
		p.tokens.Next() // consume the indent
	}

	for !p.tokens.IsAtEnd() {
		// Skip whitespace and comments but NOT indent tokens
		for p.tokens.Current() != nil && (p.tokens.Current().Type == "Whitespace" || p.tokens.Current().Type == "Comment") {
			p.tokens.Next()
		}

		// Check for end of input
		if p.tokens.IsAtEnd() {
			break
		}

		// Check for newlines
		if p.tokens.Match("Newline") {
			p.tokens.Next()
			continue
		}

		// Check for dedent (less indentation than base) or no indentation
		if p.tokens.Match("Indent") {
			currentIndent := len(p.tokens.Current().Value)
			if currentIndent < baseIndent {
				// Dedent detected - end of block
				break
			} else if currentIndent == baseIndent {
				// Skip indentation if it matches exactly
				p.tokens.Next()
			}
			// If currentIndent > baseIndent, do not consume the indent here; let parseStatement handle nested blocks.
		} else if baseIndent > 0 {
			// No indentation but we expect some - this is a dedent to column 0
			break
		}

		// Check for block-ending keywords at this indentation level
		if p.tokens.Match("Elif", "Else", "Except", "Finally") {
			break
		}

		// Parse the statement
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}

		if stmt != nil {
			statements = append(statements, stmt)
		}

		// Skip trailing whitespace and comments
		for p.tokens.Current() != nil && (p.tokens.Current().Type == "Whitespace" || p.tokens.Current().Type == "Comment") {
			p.tokens.Next()
		}

		// Expect newline after statement
		if !p.tokens.Match("Newline") && !p.tokens.IsAtEnd() {
			// If there's no newline but more tokens, it might be a syntax error
			// or the end of the block. Let the next parse attempt handle it.
			break
		}
		if p.tokens.Match("Newline") {
			p.tokens.Next()
		}
	}

	return statements, nil
}

func (p *Parser) parseExpression() (string, error) {
	// Handle simple literals and identifiers
	if p.tokens.Match("Number", "Ident") {
		token := p.tokens.Current()
		p.tokens.Next()
		p.tokens.SkipWhitespaceAndComments()

		// Check if there's an operator following the token
		if p.tokens.Match("Plus", "Minus", "Multiply", "Divide", "Modulo",
			"Less", "Greater", "LessEqual", "GreaterEqual", "Equal", "NotEqual") {

			opToken := p.tokens.Current()
			p.tokens.Next()
			p.tokens.SkipWhitespaceAndComments()

			// Parse the right-hand side of the expression
			right, err := p.parseExpression()
			if err != nil {
				return "", fmt.Errorf("invalid expression after operator: %v", err)
			}

			// Return the combined expression
			return fmt.Sprintf("%s %s %s", token.Value, opToken.Value, right), nil
		}

		// No operator, just return the token value
		return token.Value, nil
	}

	// Handle string literals
	if p.tokens.Match("String") {
		token := p.tokens.Current()
		p.tokens.Next()
		return token.Value, nil
	}

	// Handle unary operators (like -1, !condition)
	if p.tokens.Match("Minus", "Not") {
		opToken := p.tokens.Current()
		p.tokens.Next()
		p.tokens.SkipWhitespaceAndComments()

		expr, err := p.parseExpression()
		if err != nil {
			return "", fmt.Errorf("invalid expression after unary operator: %v", err)
		}
		return fmt.Sprintf("%s%s", opToken.Value, expr), nil
	}

	// Handle parenthesized expressions
	if p.tokens.Match("LeftParen") {
		p.tokens.Next()
		expr, err := p.parseExpression()
		if err != nil {
			return "", err
		}

		if !p.tokens.Match("RightParen") {
			return "", fmt.Errorf("expected ')' after expression")
		}
		p.tokens.Next()
		return fmt.Sprintf("(%s)", expr), nil
	}

	return "", fmt.Errorf("expected expression, got %s", p.tokens.Current().Type)
}

func (p *Parser) parseValue() (string, error) {
	p.tokens.SkipWhitespaceAndComments()

	if p.tokens.Match("String") {
		token := p.tokens.Current()
		p.tokens.Next()
		return strings.Trim(token.Value, "\""), nil
	}

	if p.tokens.Match("Number", "Ident") {
		token := p.tokens.Current()
		p.tokens.Next()
		return token.Value, nil
	}

	return "", fmt.Errorf("expected value")
}

// Updated main parsing function to use the new parser
func ParseWithIndentation(input string) (*Program, error) {
	parser, err := NewParser(input)
	if err != nil {
		return nil, err
	}
	return parser.ParseProgram()
}

// Utility functions (keeping the existing ones that are still needed)

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

func ResolveSymbol(symbolName string, currentModule string) string {
	if strings.Contains(symbolName, ".") {
		parts := strings.SplitN(symbolName, ".", 2)
		moduleName := parts[0]
		symbol := parts[1]

		if module, exists := LoadedModules[moduleName]; exists {
			if _, exists := module.PublicVars[symbol]; exists {
				return fmt.Sprintf("%s_%s", moduleName, symbol)
			}
			if _, exists := module.PublicClasses[symbol]; exists {
				return fmt.Sprintf("%s_%s", moduleName, symbol)
			}
		}
	}

	return symbolName
}

func GenerateUniqueSymbol(originalName string, moduleName string) string {
	if moduleName == "" {
		return originalName
	}
	return fmt.Sprintf("%s_%s", moduleName, originalName)
}

var vdt = []string{"int", "float", "double", "char", "string", "bool"}

func isValidType(s string) bool {
	return slices.Contains(vdt, s)
}

func IsOperator(s string) bool {
	operators := []string{"+", "-", "*", "/", "%"}
	return slices.Contains(operators, s)
}
