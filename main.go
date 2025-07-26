package main

import (
	"fmt"
	"log"
	"os"
	"path"
)

func main() {
	var input string
	if len(os.Args) > 1 {
		var (
			wd, _     = os.Getwd()
			ptf       = path.Join(wd, os.Args[1])
			data, err = os.ReadFile(ptf)
		)
		if err != nil {
			log.Fatal("Could not find file.")
		}
		input = string(data)
	}

	program, err := parseWithIndentation(input)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(renderC(program))
}
