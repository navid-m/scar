package meta

import (
	"flag"
	"fmt"
)

func ShowUsage() {
	fmt.Println("usage: scar [-asm] [program.x]")
	flag.PrintDefaults()
	fmt.Println("By Navid M (c) 2025")
}
