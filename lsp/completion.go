package main

import (
	"log"
	"scar/lexer"
	"strings"
)

func (ls *LanguageServer) handleAdvancedCompletion(message *Message) *Message {
	var params CompletionParams
	if err := mapToStruct(message.Params, &params); err != nil {
		log.Printf("Error parsing completion params: %v", err)
		return ls.handleCompletion(message)
	}

	doc := ls.documents[params.TextDocument.URI]
	if doc == nil {
		return ls.handleCompletion(message)
	}
	lines := strings.Split(doc.Text, "\n")
	if params.Position.Line >= len(lines) {
		return ls.handleCompletion(message)
	}

	currentLine := lines[params.Position.Line]
	prefix := ""
	if params.Position.Character <= len(currentLine) {
		prefix = currentLine[:params.Position.Character]
	}

	completionItems := []CompletionItem{}
	context := analyzeCompletionContext(prefix, lines, params.Position.Line)

	switch context.Type {
	case "import":
		completionItems = append(completionItems, getImportCompletions()...)
	case "type":
		completionItems = append(completionItems, getTypeCompletions()...)
	case "function_body":
		completionItems = append(completionItems, getFunctionBodyCompletions()...)
	case "class_body":
		completionItems = append(completionItems, getClassBodyCompletions()...)
	case "after_dot":
		completionItems = append(completionItems, getMethodCompletions(context.Object)...)
	default:
		completionItems = append(completionItems, getGeneralCompletions()...)
	}
	if doc.Program != nil {
		completionItems = append(completionItems, getSymbolCompletions(doc.Program)...)
	}

	result := CompletionList{
		IsIncomplete: false,
		Items:        completionItems,
	}

	return &Message{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result:  result,
	}
}

type CompletionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	Context      *CompletionContext     `json:"context,omitempty"`
}

type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

type CompletionContext struct {
	TriggerKind      int    `json:"triggerKind"`
	TriggerCharacter string `json:"triggerCharacter,omitempty"`
}

type AnalysisContext struct {
	Type   string
	Object string
	Scope  string
}

func analyzeCompletionContext(prefix string, lines []string, lineNum int) AnalysisContext {
	trimmedPrefix := strings.TrimSpace(prefix)

	if strings.HasPrefix(trimmedPrefix, "import") {
		return AnalysisContext{Type: "import"}
	}

	if strings.Contains(prefix, "->") || strings.Contains(prefix, ":") {
		return AnalysisContext{Type: "type"}
	}

	if strings.Contains(prefix, ".") {
		parts := strings.Split(prefix, ".")
		if len(parts) >= 2 {
			object := strings.TrimSpace(parts[len(parts)-2])
			return AnalysisContext{Type: "after_dot", Object: object}
		}
	}

	for i := lineNum - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "fn ") || strings.HasPrefix(line, "pub fn ") {
			return AnalysisContext{Type: "function_body"}
		}
		if strings.HasPrefix(line, "class ") || strings.HasPrefix(line, "pub class ") {
			return AnalysisContext{Type: "class_body"}
		}

		break
	}

	return AnalysisContext{Type: "general"}
}

func getImportCompletions() []CompletionItem {
	return []CompletionItem{
		{Label: "\"std/io\"", Kind: 9, Detail: "Standard I/O module"},
		{Label: "\"std/os\"", Kind: 9, Detail: "Operating system module"},
		{Label: "\"std/strings\"", Kind: 9, Detail: "String manipulation module"},
		{Label: "\"std/fs\"", Kind: 9, Detail: "File system module"},
		{Label: "\"std/collections\"", Kind: 9, Detail: "Collections module"},
		{Label: "\"std/regex\"", Kind: 9, Detail: "Regular expressions module"},
		{Label: "\"std/math\"", Kind: 9, Detail: "Mathematics module"},
		{Label: "\"std/time\"", Kind: 9, Detail: "Time utilities module"},
		{Label: "\"std/net\"", Kind: 9, Detail: "Networking module"},
		{Label: "\"std/json\"", Kind: 9, Detail: "JSON handling module"},
		{Label: "\"std/sort\"", Kind: 9, Detail: "Sort algorithms module"},
		{Label: "\"std/random\"", Kind: 9, Detail: "Random module"},
		{Label: "\"std/test\"", Kind: 9, Detail: "Unit testing module"},
		{Label: "\"std/path\"", Kind: 9, Detail: "Path handling module"},
		{Label: "\"std/threads\"", Kind: 9, Detail: "Multithreading module"},
		{Label: "\"std/uri\"", Kind: 9, Detail: "URI handling module"},
		{Label: "\"std/hash\"", Kind: 9, Detail: "Hashing module"},
		{Label: "\"std/crypto\"", Kind: 9, Detail: "Cryptography module"},
	}
}

func getTypeCompletions() []CompletionItem {
	return []CompletionItem{
		{Label: "void", Kind: 25, Detail: "No return value"},
		{Label: "string", Kind: 25, Detail: "Text string"},
		{Label: "int", Kind: 25, Detail: "Integer number"},
		{Label: "bool", Kind: 25, Detail: "Boolean value"},
		{Label: "float", Kind: 25, Detail: "Floating point number"},
		{Label: "char", Kind: 25, Detail: "Single character"},
		{Label: "u8", Kind: 25, Detail: "Unsigned 8-bit integer"},
		{Label: "ref", Kind: 14, Detail: "Reference type"},
		{Label: "collections::StringArrayList", Kind: 7, Detail: "Dynamic string array"},
		{Label: "collections::IntArrayList", Kind: 7, Detail: "Dynamic integer array"},
	}
}

func getFunctionBodyCompletions() []CompletionItem {
	return []CompletionItem{
		{Label: "print", Kind: 3, Detail: "Print to stdout"},
		{Label: "put", Kind: 3, Detail: "Print without newline"},
		{Label: "if", Kind: 14, Detail: "Conditional statement"},
		{Label: "while", Kind: 14, Detail: "While loop"},
		{Label: "for", Kind: 14, Detail: "For loop"},
		{Label: "parallel for", Kind: 14, Detail: "Parallel for loop"},
		{Label: "return", Kind: 14, Detail: "Return from function"},
		{Label: "var", Kind: 14, Detail: "Variable declaration"},
		{Label: "string", Kind: 25, Detail: "String type"},
		{Label: "int", Kind: 25, Detail: "Integer type"},
		{Label: "bool", Kind: 25, Detail: "Boolean type"},
		{Label: "new", Kind: 14, Detail: "Create new instance"},
		{Label: "true", Kind: 12, Detail: "Boolean true"},
		{Label: "false", Kind: 12, Detail: "Boolean false"},
	}
}

func getClassBodyCompletions() []CompletionItem {
	return []CompletionItem{
		{Label: "init", Kind: 3, Detail: "Constructor method"},
		{Label: "fn", Kind: 14, Detail: "Method declaration"},
		{Label: "pub fn", Kind: 14, Detail: "Public method declaration"},
		{Label: "var", Kind: 14, Detail: "Instance variable"},
		{Label: "string", Kind: 25, Detail: "String type"},
		{Label: "int", Kind: 25, Detail: "Integer type"},
		{Label: "bool", Kind: 25, Detail: "Boolean type"},
		{Label: "this.", Kind: 14, Detail: "Reference to current instance"},
	}
}

func getMethodCompletions(_ string) []CompletionItem {
	return []CompletionItem{
		{Label: "length()", Kind: 2, Detail: "Get length"},
		{Label: "append()", Kind: 2, Detail: "Append element"},
		{Label: "toString()", Kind: 2, Detail: "Convert to string"},
		{Label: "trim()", Kind: 2, Detail: "Trim whitespace"},
		{Label: "split()", Kind: 2, Detail: "Split string"},
		{Label: "replace()", Kind: 2, Detail: "Replace substring"},
	}
}

func getGeneralCompletions() []CompletionItem {
	return []CompletionItem{
		{Label: "fn", Kind: 14, Detail: "Function declaration"},
		{Label: "pub fn", Kind: 14, Detail: "Public function declaration"},
		{Label: "class", Kind: 14, Detail: "Class declaration"},
		{Label: "pub class", Kind: 14, Detail: "Public class declaration"},
		{Label: "struct", Kind: 14, Detail: "Struct declaration"},
		{Label: "pub struct", Kind: 14, Detail: "Public struct declaration"},
		{Label: "import", Kind: 14, Detail: "Import module"},
		{Label: "if", Kind: 14, Detail: "Conditional statement"},
		{Label: "else", Kind: 14, Detail: "Else clause"},
		{Label: "while", Kind: 14, Detail: "While loop"},
		{Label: "for", Kind: 14, Detail: "For loop"},
		{Label: "parallel", Kind: 14, Detail: "Parallel execution"},
		{Label: "return", Kind: 14, Detail: "Return statement"},
		{Label: "print", Kind: 3, Detail: "Print to stdout"},
		{Label: "put", Kind: 3, Detail: "Print without newline"},
		{Label: "var", Kind: 14, Detail: "Variable declaration"},
		{Label: "string", Kind: 25, Detail: "String type"},
		{Label: "int", Kind: 25, Detail: "Integer type"},
		{Label: "bool", Kind: 25, Detail: "Boolean type"},
		{Label: "void", Kind: 25, Detail: "Void type"},
		{Label: "new", Kind: 14, Detail: "Create new instance"},
		{Label: "ref", Kind: 14, Detail: "Reference type"},
		{Label: "true", Kind: 12, Detail: "Boolean true"},
		{Label: "false", Kind: 12, Detail: "Boolean false"},
		{Label: "unsafe", Kind: 14, Detail: "Unsafe operation"},
		{Label: "alias", Kind: 14, Detail: "Type alias"},
		{Label: "macro", Kind: 14, Detail: "Macro declaration"},
		{Label: "enum", Kind: 14, Detail: "Enumeration declaration"},
	}
}

func getSymbolCompletions(program *lexer.Program) []CompletionItem {
	items := []CompletionItem{}

	for _, stmt := range program.Statements {
		if stmt.TopLevelFuncDecl != nil {
			items = append(items, CompletionItem{
				Label:  stmt.TopLevelFuncDecl.Name,
				Kind:   3, // Function
				Detail: "User-defined function",
			})
		}
		if stmt.PubTopLevelFuncDecl != nil {
			items = append(items, CompletionItem{
				Label:  stmt.PubTopLevelFuncDecl.Name,
				Kind:   3, // Function
				Detail: "Public user-defined function",
			})
		}
		if stmt.ClassDecl != nil {
			items = append(items, CompletionItem{
				Label:  stmt.ClassDecl.Name,
				Kind:   7, // Class
				Detail: "User-defined class",
			})
		}
		if stmt.PubClassDecl != nil {
			items = append(items, CompletionItem{
				Label:  stmt.PubClassDecl.Name,
				Kind:   7, // Class
				Detail: "Public user-defined class",
			})
		}
		if stmt.StructDecl != nil {
			items = append(items, CompletionItem{
				Label:  stmt.StructDecl.Name,
				Kind:   22, // Struct
				Detail: "User-defined struct",
			})
		}
		if stmt.PubStructDecl != nil {
			items = append(items, CompletionItem{
				Label:  stmt.PubStructDecl.Name,
				Kind:   22, // Struct
				Detail: "Public user-defined struct",
			})
		}
		if stmt.VarDecl != nil {
			items = append(items, CompletionItem{
				Label:  stmt.VarDecl.Name,
				Kind:   6, // Variable
				Detail: "Variable",
			})
		}
		if stmt.PubVarDecl != nil {
			items = append(items, CompletionItem{
				Label:  stmt.PubVarDecl.Name,
				Kind:   6, // Variable
				Detail: "Public variable",
			})
		}
	}

	return items
}
