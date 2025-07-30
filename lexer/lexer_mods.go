package lexer

import (
	"fmt"

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
