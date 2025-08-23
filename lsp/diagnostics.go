package main

import (
	"log"
	"scar/lexer"
	"strings"
)

type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Source   string `json:"source"`
	Message  string `json:"message"`
}

type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

const (
	DiagnosticSeverityError       = 1
	DiagnosticSeverityWarning     = 2
	DiagnosticSeverityInformation = 3
	DiagnosticSeverityHint        = 4
)

func (ls *LanguageServer) updateDiagnostics(doc *Document) {
	diagnostics := []Diagnostic{}

	if doc.Program == nil {
		_, err := lexer.InnerParseWithIndentation(doc.Text, doc.URI)
		if err != nil {
			diagnostic := Diagnostic{
				Range: Range{
					Start: Position{Line: 0, Character: 0},
					End:   Position{Line: 0, Character: 0},
				},
				Severity: DiagnosticSeverityError,
				Source:   "scar",
				Message:  err.Error(),
			}
			diagnostics = append(diagnostics, diagnostic)
		}
	} else {
		validationErrors := lexer.ValidateProgram(doc.Program)
		for _, validationError := range validationErrors {
			diagnostic := Diagnostic{
				Range: Range{
					Start: Position{Line: 0, Character: 0},
					End:   Position{Line: 0, Character: 0},
				},
				Severity: DiagnosticSeverityError,
				Source:   "scar",
				Message:  validationError.Error(),
			}
			diagnostics = append(diagnostics, diagnostic)
		}
	}

	lines := strings.Split(doc.Text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(line, "\t") && strings.Contains(line, "    ") {
			diagnostic := Diagnostic{
				Range: Range{
					Start: Position{Line: i, Character: 0},
					End:   Position{Line: i, Character: len(line)},
				},
				Severity: DiagnosticSeverityWarning,
				Source:   "scar",
				Message:  "Mixed tabs and spaces for indentation",
			}
			diagnostics = append(diagnostics, diagnostic)
		}
		if strings.HasPrefix(trimmed, "fn ") && !strings.Contains(trimmed, "->") && strings.HasSuffix(trimmed, ":") {
			if !containsReturnType(trimmed) {
				diagnostic := Diagnostic{
					Range: Range{
						Start: Position{Line: i, Character: 0},
						End:   Position{Line: i, Character: len(line)},
					},
					Severity: DiagnosticSeverityHint,
					Source:   "scar",
					Message:  "Consider adding explicit return type (-> void)",
				}
				diagnostics = append(diagnostics, diagnostic)
			}
		}
	}

	params := PublishDiagnosticsParams{
		URI:         doc.URI,
		Diagnostics: diagnostics,
	}

	notification := Message{
		JSONRPC: "2.0",
		Method:  "textDocument/publishDiagnostics",
		Params:  params,
	}

	if err := writeMessage(ls.writer, &notification); err != nil {
		log.Printf("Error publishing diagnostics: %v", err)
	}
}

func containsReturnType(line string) bool {
	return strings.Contains(line, "-> void") ||
		strings.Contains(line, "-> string") ||
		strings.Contains(line, "-> int") ||
		strings.Contains(line, "-> bool") ||
		strings.Contains(line, "-> ref")
}

func (ls *LanguageServer) handleDidOpenWithDiagnostics(message *Message) *Message {
	var params DidOpenTextDocumentParams
	if err := mapToStruct(message.Params, &params); err != nil {
		log.Printf("Error parsing didOpen params: %v", err)
		return nil
	}

	doc := &Document{
		URI:     params.TextDocument.URI,
		Text:    params.TextDocument.Text,
		Version: params.TextDocument.Version,
	}

	if program, err := lexer.InnerParseWithIndentation(doc.Text, doc.URI); err == nil {
		doc.Program = program
	} else {
		log.Printf("Parse error for %s: %v", doc.URI, err)
	}

	ls.documents[doc.URI] = doc

	ls.updateDiagnostics(doc)

	return nil
}

func (ls *LanguageServer) handleDidChangeWithDiagnostics(message *Message) *Message {
	var params DidChangeTextDocumentParams
	if err := mapToStruct(message.Params, &params); err != nil {
		log.Printf("Error parsing didChange params: %v", err)
		return nil
	}

	doc := ls.documents[params.TextDocument.URI]
	if doc == nil {
		return nil
	}

	if len(params.ContentChanges) > 0 {
		doc.Text = params.ContentChanges[0].Text
		doc.Version = params.TextDocument.Version
		if program, err := lexer.InnerParseWithIndentation(doc.Text, doc.URI); err == nil {
			doc.Program = program
		} else {
			log.Printf("Parse error for %s: %v", doc.URI, err)
			doc.Program = nil
		}
		ls.updateDiagnostics(doc)
	}

	return nil
}
