// Scar Language Server
// Implements the Language Server Protocol (LSP) for the Scar programming language
//
// Author: Navid M
// Date: 2025
// License: GPL3

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"scar/lexer"
	"strconv"
	"strings"
)

// LSP message structures
type Message struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Method  string    `json:"method,omitempty"`
	Params  any       `json:"params,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *RPCError `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// LSP structures for initialization
type InitializeParams struct {
	ProcessID             int                `json:"processId"`
	RootPath              string             `json:"rootPath"`
	RootURI               string             `json:"rootUri"`
	InitializationOptions any                `json:"initializationOptions"`
	Capabilities          ClientCapabilities `json:"capabilities"`
	Trace                 string             `json:"trace"`
	WorkspaceFolders      []WorkspaceFolder  `json:"workspaceFolders"`
}

type ClientCapabilities struct {
	TextDocument *TextDocumentClientCapabilities `json:"textDocument,omitempty"`
}

type TextDocumentClientCapabilities struct {
	Synchronization *TextDocumentSyncClientCapabilities `json:"synchronization,omitempty"`
	Completion      *CompletionClientCapabilities       `json:"completion,omitempty"`
	Hover           *HoverClientCapabilities            `json:"hover,omitempty"`
}

type TextDocumentSyncClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	WillSave            bool `json:"willSave,omitempty"`
	WillSaveWaitUntil   bool `json:"willSaveWaitUntil,omitempty"`
	DidSave             bool `json:"didSave,omitempty"`
}

type CompletionClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type HoverClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type WorkspaceFolder struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

type ServerCapabilities struct {
	TextDocumentSync       int                `json:"textDocumentSync"`
	CompletionProvider     *CompletionOptions `json:"completionProvider,omitempty"`
	HoverProvider          bool               `json:"hoverProvider,omitempty"`
	DocumentSymbolProvider bool               `json:"documentSymbolProvider,omitempty"`
}

type CompletionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

// Text Document structures
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type TextDocumentContentChangeEvent struct {
	Range       *Range `json:"range,omitempty"`
	RangeLength int    `json:"rangeLength,omitempty"`
	Text        string `json:"text"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type LanguageServer struct {
	documents map[string]*Document
	writer    io.Writer
}

type Document struct {
	URI     string
	Text    string
	Version int
	Program *lexer.Program
}

func NewLanguageServer(writer io.Writer) *LanguageServer {
	return &LanguageServer{
		documents: make(map[string]*Document),
		writer:    writer,
	}
}

func (ls *LanguageServer) processMessage(message *Message) *Message {
	switch message.Method {
	case "initialize":
		return ls.handleInitialize(message)
	case "initialized":
		return nil
	case "textDocument/didOpen":
		return ls.handleDidOpenWithDiagnostics(message)
	case "textDocument/didChange":
		return ls.handleDidChangeWithDiagnostics(message)
	case "textDocument/completion":
		return ls.handleAdvancedCompletion(message)
	case "textDocument/hover":
		return ls.handleEnhancedHover(message)
	case "textDocument/documentSymbol":
		return ls.handleDocumentSymbols(message)
	case "shutdown":
		return &Message{JSONRPC: "2.0", ID: message.ID, Result: nil}
	case "exit":
		os.Exit(0)
	}
	return nil
}

func (ls *LanguageServer) handleInitialize(message *Message) *Message {
	result := InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync: 1, // Full sync
			CompletionProvider: &CompletionOptions{
				TriggerCharacters: []string{".", ":", " "},
			},
			HoverProvider:          true,
			DocumentSymbolProvider: true,
		},
	}

	return &Message{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result:  result,
	}
}

func (ls *LanguageServer) handleDidOpen(message *Message) *Message {
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

	// Parse the document
	if program, err := lexer.InnerParseWithIndentation(doc.Text, doc.URI); err == nil {
		doc.Program = program
	} else {
		log.Printf("Parse error for %s: %v", doc.URI, err)
	}

	ls.documents[doc.URI] = doc
	return nil
}

func (ls *LanguageServer) handleDidChange(message *Message) *Message {
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
	}

	return nil
}

func (ls *LanguageServer) handleCompletion(message *Message) *Message {
	// Basic completion - return Scar keywords and types
	completionItems := []CompletionItem{
		{Label: "fn", Kind: 3},        // Function
		{Label: "class", Kind: 7},     // Class
		{Label: "struct", Kind: 7},    // Class
		{Label: "if", Kind: 14},       // Keyword
		{Label: "else", Kind: 14},     // Keyword
		{Label: "while", Kind: 14},    // Keyword
		{Label: "for", Kind: 14},      // Keyword
		{Label: "return", Kind: 14},   // Keyword
		{Label: "print", Kind: 3},     // Function
		{Label: "parallel", Kind: 14}, // Keyword
		{Label: "string", Kind: 25},   // TypeParameter
		{Label: "int", Kind: 25},      // TypeParameter
		{Label: "bool", Kind: 25},     // TypeParameter
		{Label: "void", Kind: 25},     // TypeParameter
		{Label: "ref", Kind: 14},      // Keyword
		{Label: "var", Kind: 14},      // Keyword
		{Label: "pub", Kind: 14},      // Keyword
		{Label: "import", Kind: 14},   // Keyword
		{Label: "new", Kind: 14},      // Keyword
		{Label: "true", Kind: 12},     // Value
		{Label: "false", Kind: 12},    // Value
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

func (ls *LanguageServer) handleHover(message *Message) *Message {
	// Basic hover - could be enhanced with actual symbol information
	hover := Hover{
		Contents: MarkupContent{
			Kind:  "markdown",
			Value: "Scar programming language construct",
		},
	}

	return &Message{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result:  hover,
	}
}

// Completion structures
type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

type CompletionItem struct {
	Label  string `json:"label"`
	Kind   int    `json:"kind"`
	Detail string `json:"detail,omitempty"`
}

// Hover structures
type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// Utility functions
func mapToStruct(data interface{}, target interface{}) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, target)
}

// LSP Transport layer
func readMessage(reader *bufio.Reader) (*Message, error) {
	// Read headers
	contentLength := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length: ") {
			length, err := strconv.Atoi(strings.TrimPrefix(line, "Content-Length: "))
			if err != nil {
				return nil, err
			}
			contentLength = length
		}
	}

	// Read content
	if contentLength == 0 {
		return nil, fmt.Errorf("no Content-Length header")
	}

	content := make([]byte, contentLength)
	_, err := io.ReadFull(reader, content)
	if err != nil {
		return nil, err
	}

	var message Message
	if err := json.Unmarshal(content, &message); err != nil {
		return nil, err
	}

	return &message, nil
}

func writeMessage(writer io.Writer, message *Message) error {
	content, err := json.Marshal(message)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(content))
	_, err = writer.Write([]byte(header))
	if err != nil {
		return err
	}

	_, err = writer.Write(content)
	return err
}

func main() {
	// Setup logging to stderr
	logFile, err := os.OpenFile(filepath.Join(os.TempDir(), "scar-lsp.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.SetOutput(os.Stderr)
	} else {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	log.Println("Scar Language Server starting...")

	server := NewLanguageServer(os.Stdout)
	reader := bufio.NewReader(os.Stdin)

	for {
		message, err := readMessage(reader)
		if err != nil {
			log.Printf("Error reading message: %v", err)
			continue
		}

		log.Printf("Received message: %s", message.Method)

		response := server.processMessage(message)
		if response != nil {
			if err := writeMessage(os.Stdout, response); err != nil {
				log.Printf("Error writing response: %v", err)
			}
		}
	}
}
