// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Contains meta information and usage instructions for the scar compiler.

package meta

import (
	"flag"
	"fmt"
)

const Version = "v0.0.1"

func ShowUsage() {
	fmt.Println("Usage: scar [-asm | -c] [program]")
	flag.PrintDefaults()
	fmt.Printf("\nScar %v - By Navid M (c) 2025", Version)
}
