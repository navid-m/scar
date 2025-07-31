package meta

import (
	"flag"
	"fmt"
)

func ShowUsage() error {
	fmt.Println("Usage: scar [-asm] [program.x]")
	flag.PrintDefaults()
	_, err := fmt.Println("\nBy Navid M (c) 2025")
	return err
}
