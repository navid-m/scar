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

func ShowUsage() {
	fmt.Println("Usage: scar [-asm | -c] [program.x]")
	flag.PrintDefaults()
	fmt.Println("\nBy Navid M (c) 2025")
}
