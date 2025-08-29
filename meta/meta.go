// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Contains meta information and usage instructions for the scar compiler.

package meta

import (
	"fmt"
	"runtime"
)

const Version = "v0.0.1-alpha.3"

func ShowUsage() {
	fmt.Printf("Scar %v - By Navid M (c) 2025\n\n", Version)
	fmt.Println("Usage:")
	fmt.Println("  scar [options] [program]")
	fmt.Println()
	fmt.Println("Options:")

	fmt.Printf("  %-10s %s\n", "-asm", "Show assembly output")
	fmt.Printf("  %-10s %s\n", "-c", "Emit C output")
	fmt.Printf("  %-10s %s\n", "-d", "Enable verbose logging")
	fmt.Printf("  %-10s %s\n", "-dll", "Compile as dynamic link library")
	fmt.Printf("  %-10s %s\n", "-gc", "Use garbage collector")
	fmt.Printf("  %-10s %s\n", "-keepc", "Keep generated C file")
	fmt.Printf("  %-10s %s\n", "-l <opts>", "Additional linker options (e.g. -lm -lpthread)")
	fmt.Printf("  %-10s %s\n", "-o <file>", "Output binary name")
	fmt.Printf("  %-10s %s\n", "-opt", "Optimize for performance")
	fmt.Printf("  %-10s %s\n", "-repl", "Run in REPL mode")
	fmt.Printf("  %-10s %s\n", "-v", "Show version")

	if runtime.GOOS == "windows" {
		fmt.Printf("  %-10s %s\n", "-nowin", "Disable native windows headers")
	} else {
		fmt.Printf("  %-10s %s\n", "-asan", "Enable address/undefined sanitizers")
	}

	fmt.Printf("  %-10s %s\n\n", "-h, --help", "Show this help message")
}
