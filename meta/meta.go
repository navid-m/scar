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
