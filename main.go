package main

import (
	"flag"
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
	asm := flag.Bool("asm", false, "show assembly output")
	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Println("usage: scar [-asm] [program.x]")
		return
	}

	var (
		input   string
		baseDir string
		ptf     string
	)

	if len(flag.Args()) > 0 {
		wd, _ := os.Getwd()
		ptf = path.Join(wd, flag.Arg(0))
		baseDir = filepath.Dir(ptf)
		data, err := os.ReadFile(ptf)
		if err != nil {
			log.Fatal("Could not find file.")
		}
		input = string(data)
	}

	cleanedName := strings.ReplaceAll(filepath.Base(ptf), ".x", "")
	program, err := lexer.ParseWithIndentation(input)
	if err != nil {
		log.Fatal(err)
	}

	cCode := preprocessor.InsertMacros(renderer.RenderC(program, baseDir))

	if *asm {
		cmd := exec.Command("clang", "-w", "-S", "-x", "c", "-o", "-", "-")
		cmd.Stdin = strings.NewReader(cCode)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			log.Fatal("Failed to generate assembly.")
		}
		return
	}

	tmpCPath := cleanedName + ".c"
	err = os.WriteFile(tmpCPath, []byte(cCode), 0644)
	if err != nil {
		log.Fatalf("Failed to write temp file: %v", err)
	}
	defer os.Remove(tmpCPath)

	outputBinary := "./" + cleanedName
	var success bool

	cmd := exec.Command("clang", "-w", tmpCPath, "-o", outputBinary)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()

	if runtime.GOOS == "windows" {
		outputBinary += ".exe"
	}

	if err == nil {
		fmt.Printf("Compiled %s\n", outputBinary)
		success = true
	}

	if !success {
		log.Fatal("Failed to compile.")
	}
}
