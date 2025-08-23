package main

import (
	"log"
	"scar/lexer"
	"strings"
)

type HoverParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// Enhanced hover with symbol information
func (ls *LanguageServer) handleEnhancedHover(message *Message) *Message {
	var params HoverParams
	if err := mapToStruct(message.Params, &params); err != nil {
		log.Printf("Error parsing hover params: %v", err)
		return ls.handleHover(message) // Fallback to basic hover
	}

	doc := ls.documents[params.TextDocument.URI]
	if doc == nil {
		return ls.handleHover(message)
	}

	lines := strings.Split(doc.Text, "\n")
	if params.Position.Line >= len(lines) {
		return ls.handleHover(message)
	}

	currentLine := lines[params.Position.Line]
	if params.Position.Character >= len(currentLine) {
		return ls.handleHover(message)
	}

	// Get word at cursor position
	word := getWordAtPosition(currentLine, params.Position.Character)
	if word == "" {
		return nil
	}

	// Get hover information based on word
	hoverInfo := getHoverInfo(word, doc.Program, lines, params.Position.Line)

	if hoverInfo == "" {
		return nil
	}

	hover := Hover{
		Contents: MarkupContent{
			Kind:  "markdown",
			Value: hoverInfo,
		},
	}

	return &Message{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result:  hover,
	}
}

func getWordAtPosition(line string, character int) string {
	if character >= len(line) {
		return ""
	}

	// Find word boundaries
	start := character
	end := character

	// Move start backward
	for start > 0 && isWordChar(rune(line[start-1])) {
		start--
	}

	// Move end forward
	for end < len(line) && isWordChar(rune(line[end])) {
		end++
	}

	if start == end {
		return ""
	}

	return line[start:end]
}

func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

func getHoverInfo(word string, program *lexer.Program, lines []string, lineNum int) string {
	// Check for built-in types
	if info := getBuiltinTypeInfo(word); info != "" {
		return info
	}

	// Check for keywords
	if info := getKeywordInfo(word); info != "" {
		return info
	}

	// Check for standard library functions
	if info := getStdLibInfo(word, lines, lineNum); info != "" {
		return info
	}

	// Check in parsed program
	if program != nil {
		if info := getUserDefinedSymbolInfo(word, program); info != "" {
			return info
		}
	}

	return ""
}

func getBuiltinTypeInfo(word string) string {
	switch word {
	case "string":
		return "**string** - Text string type\n\nA sequence of characters used to represent text."
	case "int":
		return "**int** - Integer type\n\nA whole number value (positive, negative, or zero)."
	case "bool":
		return "**bool** - Boolean type\n\nA logical value that can be either `true` or `false`."
	case "void":
		return "**void** - Void type\n\nRepresents the absence of a value, typically used for functions that don't return anything."
	case "float":
		return "**float** - Floating point type\n\nA number with decimal places."
	case "char":
		return "**char** - Character type\n\nA single character value."
	case "u8":
		return "**u8** - Unsigned 8-bit integer\n\nAn integer value from 0 to 255."
	case "ref":
		return "**ref** - Reference type\n\nA reference to another object or value."
	}
	return ""
}

func getKeywordInfo(word string) string {
	switch word {
	case "fn":
		return "**fn** - Function declaration keyword\n\nUsed to declare functions.\n\n```scar\nfn function_name(param: type) -> return_type:\n    // function body\n```"
	case "class":
		return "**class** - Class declaration keyword\n\nUsed to declare classes.\n\n```scar\nclass ClassName:\n    init:\n        // constructor\n    fn method_name():\n        // method body\n```"
	case "struct":
		return "**struct** - Struct declaration keyword\n\nUsed to declare structures.\n\n```scar\nstruct StructName:\n    field_type field_name\n```"
	case "if":
		return "**if** - Conditional statement\n\nExecutes code based on a condition.\n\n```scar\nif condition:\n    // code to execute if true\n```"
	case "else":
		return "**else** - Else clause\n\nExecutes code when the if condition is false.\n\n```scar\nif condition:\n    // if true\nelse:\n    // if false\n```"
	case "while":
		return "**while** - While loop\n\nRepeats code while a condition is true.\n\n```scar\nwhile condition:\n    // code to repeat\n```"
	case "for":
		return "**for** - For loop\n\nIterates over a range or collection.\n\n```scar\nfor i = 1 to 10:\n    // loop body\n```"
	case "parallel":
		return "**parallel** - Parallel execution keyword\n\nExecutes code in parallel.\n\n```scar\nparallel for i = 1 to 10:\n    // parallel execution\n```"
	case "return":
		return "**return** - Return statement\n\nReturns a value from a function.\n\n```scar\nreturn value\n```"
	case "print":
		return "**print** - Print function\n\nPrints a value to stdout with a newline.\n\n```scar\nprint \"Hello, World!\"\n```"
	case "put":
		return "**put** - Put function\n\nPrints a value to stdout without a newline.\n\n```scar\nput \"Hello\"\n```"
	case "var":
		return "**var** - Variable declaration\n\nDeclares a variable.\n\n```scar\nvar variable_name = value\n```"
	case "pub":
		return "**pub** - Public visibility modifier\n\nMakes functions, classes, or variables public.\n\n```scar\npub fn public_function():\n    // accessible from other modules\n```"
	case "import":
		return "**import** - Import statement\n\nImports modules or libraries.\n\n```scar\nimport \"std/io\"\n```"
	case "new":
		return "**new** - Object instantiation\n\nCreates a new instance of a class or struct.\n\n```scar\nvar obj = new ClassName()\n```"
	case "true", "false":
		return "**" + word + "** - Boolean literal\n\nA boolean value."
	case "unsafe":
		return "**unsafe** - Unsafe operation marker\n\nMarks operations that bypass safety checks.\n\n```scar\nunsafe alias name = module::function\n```"
	case "alias":
		return "**alias** - Type or function alias\n\nCreates an alias for a type or function.\n\n```scar\nalias name = original_name\n```"
	case "macro":
		return "**macro** - Macro declaration\n\nDeclares a macro for code generation.\n\n```scar\nmacro macro_name(args):\n    // macro body\n```"
	case "enum":
		return "**enum** - Enumeration declaration\n\nDeclares an enumeration type.\n\n```scar\nenum EnumName:\n    VALUE1\n    VALUE2\n```"
	}
	return ""
}

func getStdLibInfo(word string, lines []string, lineNum int) string {
	// Check imports to see what standard library modules are available
	imports := extractImports(lines)
	
	for _, imp := range imports {
		if strings.Contains(imp, "std/") {
			module := strings.TrimPrefix(imp, "std/")
			module = strings.Trim(module, "\"")
			
			info := getStdLibFunctionInfo(word, module)
			if info != "" {
				return info
			}
		}
	}
	
	return ""
}

func extractImports(lines []string) []string {
	var imports []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import ") {
			import_part := strings.TrimPrefix(trimmed, "import ")
			imports = append(imports, import_part)
		}
	}
	return imports
}

func getStdLibFunctionInfo(word string, module string) string {
	switch module {
	case "io":
		switch word {
		case "read_file":
			return "**io::read_file** - Read file contents\n\nReads the entire contents of a file into a string.\n\n```scar\nstring content = io::read_file(\"path/to/file.txt\")\n```"
		case "write_file":
			return "**io::write_file** - Write to file\n\nWrites content to a file.\n\n```scar\nio::write_file(\"path/to/file.txt\", \"content\")\n```"
		}
	case "strings":
		switch word {
		case "trim":
			return "**strings::trim** - Trim whitespace\n\nRemoves leading and trailing whitespace.\n\n```scar\nstring result = strings::trim(\" hello \")\n```"
		case "split":
			return "**strings::split** - Split string\n\nSplits a string by delimiter.\n\n```scar\nStringArrayList parts = strings::split(\"a,b,c\", \",\")\n```"
		case "length":
			return "**strings::length** - Get string length\n\nReturns the length of a string.\n\n```scar\nint len = strings::length(\"hello\")\n```"
		}
	case "collections":
		switch word {
		case "StringArrayList":
			return "**collections::StringArrayList** - Dynamic string array\n\nA resizable array of strings.\n\n```scar\nvar list = new collections::StringArrayList(10)\n```"
		case "IntArrayList":
			return "**collections::IntArrayList** - Dynamic integer array\n\nA resizable array of integers.\n\n```scar\nvar list = new collections::IntArrayList(10)\n```"
		}
	case "os":
		switch word {
		case "cwd":
			return "**os::cwd** - Current working directory\n\nReturns the current working directory.\n\n```scar\nstring dir = os::cwd()\n```"
		case "exit":
			return "**os::exit** - Exit program\n\nExits the program with a status code.\n\n```scar\nos::exit(0)\n```"
		}
	}
	return ""
}

func getUserDefinedSymbolInfo(word string, program *lexer.Program) string {
	for _, stmt := range program.Statements {
		if stmt.TopLevelFuncDecl != nil && stmt.TopLevelFuncDecl.Name == word {
			return "**" + word + "** - User-defined function\n\nA function defined in this file."
		}
		if stmt.PubTopLevelFuncDecl != nil && stmt.PubTopLevelFuncDecl.Name == word {
			return "**" + word + "** - Public user-defined function\n\nA public function defined in this file."
		}
		if stmt.ClassDecl != nil && stmt.ClassDecl.Name == word {
			return "**" + word + "** - User-defined class\n\nA class defined in this file."
		}
		if stmt.PubClassDecl != nil && stmt.PubClassDecl.Name == word {
			return "**" + word + "** - Public user-defined class\n\nA public class defined in this file."
		}
		if stmt.StructDecl != nil && stmt.StructDecl.Name == word {
			return "**" + word + "** - User-defined struct\n\nA struct defined in this file."
		}
		if stmt.PubStructDecl != nil && stmt.PubStructDecl.Name == word {
			return "**" + word + "** - Public user-defined struct\n\nA public struct defined in this file."
		}
		if stmt.VarDecl != nil && stmt.VarDecl.Name == word {
			return "**" + word + "** - Variable\n\nA variable defined in this file."
		}
		if stmt.PubVarDecl != nil && stmt.PubVarDecl.Name == word {
			return "**" + word + "** - Public variable\n\nA public variable defined in this file."
		}
	}
	return ""
}
