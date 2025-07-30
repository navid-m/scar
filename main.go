package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"scar/lexer"
	"scar/preprocessor"
	"scar/renderer"
	"strings"
)

func main() {
	if len(os.Args) <= 1 {
		fmt.Println("usage: scar [program.x]")
		return
	}

	var input string
	var baseDir string

	if len(os.Args) > 1 {
		wd, _ := os.Getwd()
		ptf := path.Join(wd, os.Args[1])
		baseDir = filepath.Dir(ptf)

		data, err := os.ReadFile(ptf)
		if err != nil {
			log.Fatal("Could not find file.")
		}
		input = string(data)
	}

	cleanedName := strings.ReplaceAll(os.Args[1], ".x", "")
	program, err := lexer.ParseWithIndentation(input)
	if err != nil {
		log.Fatal(err)
	}

	cCode := preprocessor.InsertMacros(renderer.RenderC(program, baseDir))
	// fmt.Println(cCode)

	tmpCPath := cleanedName + ".c"
	err = os.WriteFile(tmpCPath, []byte(cCode), 0644)
	if err != nil {
		log.Fatalf("Failed to write temp C file: %v", err)
	}
	// defer os.Remove(tmpCPath)

	outputBinary := "./" + cleanedName
	if runtime.GOOS == "windows" {
		outputBinary += ".exe"
	}

	cmd := exec.Command("clang", "-w", tmpCPath, "-o", outputBinary) //add -lgc later
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()

	if err != nil {
		log.Fatalf("Failed to compile with clang: %v", err)
	}

	fmt.Printf("Compiled %s\n", outputBinary)
}
