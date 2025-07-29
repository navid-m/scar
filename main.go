package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
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

	cleanedName := strings.ReplaceAll(os.Args[1], ".x", "")
	program, err := parseWithIndentation(input)
	if err != nil {
		log.Fatal(err)
	}

	cCode := renderC(program)
	// fmt.Println(cCode)
	tmpCPath := cleanedName + ".c"
	err = os.WriteFile(tmpCPath, []byte(cCode), 0644)
	if err != nil {
		log.Fatalf("Failed to write temp C file: %v", err)
	}
	defer os.Remove(tmpCPath)
	outputBinary := "./" + cleanedName
	compilers := []string{"clang", "gcc"}
	var success bool

	for _, compiler := range compilers {
		cmd := exec.Command(compiler, "-w", tmpCPath, "-o", outputBinary)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()
		if runtime.GOOS == "windows" {
			outputBinary += ".exe"
		}
		if err == nil {
			fmt.Printf("Compiled %s\n", outputBinary)
			success = true
			break
		} else {
			fmt.Printf("%s failed.\n", compiler)
		}
	}

	if !success {
		log.Fatal("Failed to compile.")
	}
}
