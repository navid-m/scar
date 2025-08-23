# Scar Language Server

A Language Server Protocol (LSP) implementation for the Scar programming language, providing IDE features like syntax highlighting, autocompletion, diagnostics, and more.

## Features

- **Syntax Analysis**: Real-time parsing of Scar code using the existing lexer
- **Diagnostics**: Error reporting with syntax errors and validation warnings
- **Autocompletion**: Context-aware code completion for keywords, types, and symbols
- **Hover Information**: Detailed documentation for built-in types, keywords, and user-defined symbols
- **Document Symbols**: Outline view showing functions, classes, structs, and variables
- **Standard Library Support**: Completion and hover info for std library modules

## Supported LSP Features

### Text Document Synchronization
- `textDocument/didOpen`
- `textDocument/didChange`

### Language Features
- `textDocument/completion` - Code completion
- `textDocument/hover` - Hover information
- `textDocument/documentSymbol` - Document outline
- `textDocument/publishDiagnostics` - Error reporting

## Building

```bash
cd lsp
go build -o scar-lsp .
```

## Usage

The language server communicates via stdin/stdout using the LSP protocol. It's designed to be used with LSP-compatible editors.

### VS Code Integration

Create a VS Code extension that launches the language server:

```typescript
const serverOptions: ServerOptions = {
    command: 'path/to/scar-lsp',
    args: []
};

const clientOptions: LanguageClientOptions = {
    documentSelector: [{ scheme: 'file', language: 'scar' }]
};
```

### Vim/Neovim Integration

Using `nvim-lspconfig`:

```lua
require'lspconfig'.scar.setup{
    cmd = {'path/to/scar-lsp'},
    filetypes = {'scar'},
    root_dir = lspconfig.util.root_pattern('.git', 'go.mod'),
}
```

## Architecture

The language server consists of several modules:

- **main.go**: Core LSP server and message handling
- **diagnostics.go**: Syntax error detection and reporting
- **completion.go**: Context-aware autocompletion
- **hover.go**: Hover information for symbols and keywords
- **symbols.go**: Document symbol extraction

## Supported Scar Features

### Keywords
- `fn`, `pub fn` - Function declarations
- `class`, `pub class` - Class declarations  
- `struct`, `pub struct` - Struct declarations
- `if`, `else` - Conditionals
- `while`, `for` - Loops
- `parallel` - Parallel execution
- `return` - Return statements
- `var` - Variable declarations
- `import` - Module imports
- `new` - Object instantiation

### Types
- `string`, `int`, `bool`, `void`
- `float`, `char`, `u8`
- `ref` - Reference types
- Collections from `std/collections`

### Standard Library Modules
- `std/io` - File I/O operations
- `std/strings` - String manipulation
- `std/collections` - Data structures
- `std/os` - Operating system interface
- `std/fs` - File system operations
- `std/regex` - Regular expressions
- `std/math` - Mathematical functions
- `std/time` - Time utilities
- `std/net` - Networking
- `std/json` - JSON handling

## Diagnostics

The language server provides:

- **Syntax Errors**: Parse errors from the Scar compiler
- **Validation Errors**: Semantic validation issues
- **Style Warnings**: Mixed tabs/spaces, missing return types
- **Hints**: Code improvement suggestions

## Completion Context

The completion engine is context-aware:

- **Import Context**: Suggests standard library modules
- **Type Context**: Suggests type names after `->` or `:`
- **Method Context**: Suggests methods after `.` operator
- **Function Body**: Suggests statements and expressions
- **Class Body**: Suggests methods and instance variables

## Hover Information

Comprehensive hover documentation for:

- **Built-in Types**: Type descriptions and usage
- **Keywords**: Syntax and examples
- **Standard Library**: Function documentation
- **User Symbols**: Information about user-defined functions, classes, etc.

## Logging

The language server logs to `%TEMP%/scar-lsp.log` on Windows or `/tmp/scar-lsp.log` on Unix systems for debugging purposes.

## Future Enhancements

- **Go to Definition**: Navigate to symbol definitions
- **Find References**: Find all references to a symbol  
- **Rename Symbol**: Rename symbols across files
- **Code Formatting**: Format Scar code
- **Code Actions**: Quick fixes and refactoring
- **Semantic Highlighting**: Enhanced syntax highlighting
- **Workspace Symbols**: Global symbol search
- **Signature Help**: Parameter hints for function calls

## Contributing

The language server is designed to be easily extensible. To add new features:

1. Extend the message handling in `main.go`
2. Add new LSP capabilities to `ServerCapabilities`
3. Implement the feature logic in appropriate modules
4. Update completion and hover information as needed

## License

GPL3 - Same as the Scar compiler
