package main

import (
	"log"
	"scar/lexer"
	"strings"
)

type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           int              `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

type DocumentSymbolParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

const (
	SymbolKindFile          = 1
	SymbolKindModule        = 2
	SymbolKindNamespace     = 3
	SymbolKindPackage       = 4
	SymbolKindClass         = 5
	SymbolKindMethod        = 6
	SymbolKindProperty      = 7
	SymbolKindField         = 8
	SymbolKindConstructor   = 9
	SymbolKindEnum          = 10
	SymbolKindInterface     = 11
	SymbolKindFunction      = 12
	SymbolKindVariable      = 13
	SymbolKindConstant      = 14
	SymbolKindString        = 15
	SymbolKindNumber        = 16
	SymbolKindBoolean       = 17
	SymbolKindArray         = 18
	SymbolKindObject        = 19
	SymbolKindKey           = 20
	SymbolKindNull          = 21
	SymbolKindEnumMember    = 22
	SymbolKindStruct        = 23
	SymbolKindEvent         = 24
	SymbolKindOperator      = 25
	SymbolKindTypeParameter = 26
)

func (ls *LanguageServer) handleDocumentSymbols(message *Message) *Message {
	var params DocumentSymbolParams
	if err := mapToStruct(message.Params, &params); err != nil {
		log.Printf("Error parsing document symbol params: %v", err)
		return nil
	}

	doc := ls.documents[params.TextDocument.URI]
	if doc == nil || doc.Program == nil {
		return &Message{
			JSONRPC: "2.0",
			ID:      message.ID,
			Result:  []DocumentSymbol{},
		}
	}

	symbols := extractDocumentSymbols(doc)

	return &Message{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result:  symbols,
	}
}

func extractDocumentSymbols(doc *Document) []DocumentSymbol {
	var symbols []DocumentSymbol
	lines := strings.Split(doc.Text, "\n")

	for _, stmt := range doc.Program.Statements {
		symbol := extractSymbolFromStatement(stmt, lines)
		if symbol != nil {
			symbols = append(symbols, *symbol)
		}
	}

	return symbols
}

func extractSymbolFromStatement(stmt *lexer.Statement, lines []string) *DocumentSymbol {
	if stmt.TopLevelFuncDecl != nil {
		return &DocumentSymbol{
			Name:           stmt.TopLevelFuncDecl.Name,
			Kind:           SymbolKindFunction,
			Detail:         "function",
			Range:          getStatementRange(lines, stmt.TopLevelFuncDecl.Name),
			SelectionRange: getStatementRange(lines, stmt.TopLevelFuncDecl.Name),
		}
	}

	if stmt.PubTopLevelFuncDecl != nil {
		return &DocumentSymbol{
			Name:           stmt.PubTopLevelFuncDecl.Name,
			Kind:           SymbolKindFunction,
			Detail:         "public function",
			Range:          getStatementRange(lines, stmt.PubTopLevelFuncDecl.Name),
			SelectionRange: getStatementRange(lines, stmt.PubTopLevelFuncDecl.Name),
		}
	}

	if stmt.ClassDecl != nil {
		symbol := &DocumentSymbol{
			Name:           stmt.ClassDecl.Name,
			Kind:           SymbolKindClass,
			Detail:         "class",
			Range:          getStatementRange(lines, stmt.ClassDecl.Name),
			SelectionRange: getStatementRange(lines, stmt.ClassDecl.Name),
		}

		for _, method := range stmt.ClassDecl.Methods {
			methodSymbol := DocumentSymbol{
				Name:           method.Name,
				Kind:           SymbolKindMethod,
				Detail:         "method",
				Range:          getStatementRange(lines, method.Name),
				SelectionRange: getStatementRange(lines, method.Name),
			}
			symbol.Children = append(symbol.Children, methodSymbol)
		}

		return symbol
	}

	if stmt.PubClassDecl != nil {
		symbol := &DocumentSymbol{
			Name:           stmt.PubClassDecl.Name,
			Kind:           SymbolKindClass,
			Detail:         "public class",
			Range:          getStatementRange(lines, stmt.PubClassDecl.Name),
			SelectionRange: getStatementRange(lines, stmt.PubClassDecl.Name),
		}
		for _, method := range stmt.PubClassDecl.Methods {
			methodSymbol := DocumentSymbol{
				Name:           method.Name,
				Kind:           SymbolKindMethod,
				Detail:         "method",
				Range:          getStatementRange(lines, method.Name),
				SelectionRange: getStatementRange(lines, method.Name),
			}
			symbol.Children = append(symbol.Children, methodSymbol)
		}

		return symbol
	}

	if stmt.StructDecl != nil {
		return &DocumentSymbol{
			Name:           stmt.StructDecl.Name,
			Kind:           SymbolKindStruct,
			Detail:         "struct",
			Range:          getStatementRange(lines, stmt.StructDecl.Name),
			SelectionRange: getStatementRange(lines, stmt.StructDecl.Name),
		}
	}

	if stmt.PubStructDecl != nil {
		return &DocumentSymbol{
			Name:           stmt.PubStructDecl.Name,
			Kind:           SymbolKindStruct,
			Detail:         "public struct",
			Range:          getStatementRange(lines, stmt.PubStructDecl.Name),
			SelectionRange: getStatementRange(lines, stmt.PubStructDecl.Name),
		}
	}

	if stmt.VarDecl != nil {
		return &DocumentSymbol{
			Name:           stmt.VarDecl.Name,
			Kind:           SymbolKindVariable,
			Detail:         "variable",
			Range:          getStatementRange(lines, stmt.VarDecl.Name),
			SelectionRange: getStatementRange(lines, stmt.VarDecl.Name),
		}
	}

	if stmt.PubVarDecl != nil {
		return &DocumentSymbol{
			Name:           stmt.PubVarDecl.Name,
			Kind:           SymbolKindVariable,
			Detail:         "public variable",
			Range:          getStatementRange(lines, stmt.PubVarDecl.Name),
			SelectionRange: getStatementRange(lines, stmt.PubVarDecl.Name),
		}
	}

	if stmt.EnumDecl != nil {
		return &DocumentSymbol{
			Name:           stmt.EnumDecl.Name,
			Kind:           SymbolKindEnum,
			Detail:         "enum",
			Range:          getStatementRange(lines, stmt.EnumDecl.Name),
			SelectionRange: getStatementRange(lines, stmt.EnumDecl.Name),
		}
	}

	if stmt.PubEnumDecl != nil {
		return &DocumentSymbol{
			Name:           stmt.PubEnumDecl.Name,
			Kind:           SymbolKindEnum,
			Detail:         "public enum",
			Range:          getStatementRange(lines, stmt.PubEnumDecl.Name),
			SelectionRange: getStatementRange(lines, stmt.PubEnumDecl.Name),
		}
	}

	return nil
}

func getStatementRange(lines []string, symbolName string) Range {
	for i, line := range lines {
		if strings.Contains(line, symbolName) {
			return Range{
				Start: Position{Line: i, Character: 0},
				End:   Position{Line: i, Character: len(line)},
			}
		}
	}

	return Range{
		Start: Position{Line: 0, Character: 0},
		End:   Position{Line: 0, Character: 0},
	}
}
