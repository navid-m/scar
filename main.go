package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
)

func main() {
	if len(os.Args) <= 1 {
		fmt.Println("usage: scar [program.x]")
		return
	}
	var input string
	if len(os.Args) > 1 {
		wd, _ := os.Getwd()
		ptf := path.Join(wd, os.Args[1])
		data, err := os.ReadFile(ptf)
		if err != nil {
			log.Fatal("Could not find file.")
		}
		input = string(data)
	}

	program, err := parseWithIndentation(input)
	if err != nil {
		log.Fatal(err)
	}

	cCode := renderC(program)
	fmt.Println(cCode)
	tmpCPath := "temp_program.c"
	err = os.WriteFile(tmpCPath, []byte(cCode), 0644)
	if err != nil {
		log.Fatalf("Failed to write temp C file: %v", err)
	}
	defer os.Remove(tmpCPath)
	outputBinary := "./output_program"
	compilers := []string{"clang", "gcc"}
	var success bool

	for _, compiler := range compilers {
		cmd := exec.Command(compiler, tmpCPath, "-o", outputBinary)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		fmt.Printf("Trying to compile with %s...\n", compiler)
		err := cmd.Run()
		if err == nil {
			fmt.Printf("Compiled successfully with %s. Executable: %s\n", compiler, outputBinary)
			success = true
			break
		} else {
			fmt.Printf("%s failed.\n", compiler)
		}
	}

	if !success {
		log.Fatal("Failed to compile with clang, tcc, and gcc.")
	}
}
