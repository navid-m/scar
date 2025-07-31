// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Contains entry point for the scar compiler.

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
	"scar/meta"
	"scar/preprocessor"
	"scar/renderer"
	"strings"
)

func main() {
	flag.Usage = meta.ShowUsage
	asm := flag.Bool("asm", false, "show assembly output")
	c := flag.Bool("c", false, "show IL")

	flag.Parse()

	if len(flag.Args()) < 1 {
		meta.ShowUsage()
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
	input = preprocessor.ProcessSourceLevelMacros(input)
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

	if *c {
		fmt.Println(cCode)
		return
	}

	tmpCPath := cleanedName + ".c"
	err = os.WriteFile(tmpCPath, []byte(cCode), 0644)
	if err != nil {
		log.Fatalf("Failed to write temp file: %v", err)
	}
	defer os.Remove(tmpCPath)

	var (
		outputBinary = "./" + cleanedName
		cmpPath      = "clang"
		compileArgs  = []string{"-w", "-fopenmp", tmpCPath, "-o", outputBinary}
	)
	switch runtime.GOOS {
	case "darwin":
		cmpPath = "/opt/homebrew/opt/llvm/bin/clang"
		compileArgs = []string{
			"-fopenmp",
			tmpCPath,
			"-I/opt/homebrew/opt/libomp/include",
			"-L/opt/homebrew/opt/libomp/lib",
			"-o", outputBinary,
		}
	case "linux":
		compileArgs = []string{
			"-fopenmp",
			tmpCPath,
			"-o", outputBinary,
		}
	case "windows":
		// Assumes libomp is present and MSVC or Clang is installed
		// Will be part of the installer. I suppose.
		outputBinary += ".exe"
		compileArgs = []string{
			"-fopenmp",
			tmpCPath,
			"-o", outputBinary,
		}
	}

	cmd := exec.Command(cmpPath, compileArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()

	success := false
	if err == nil {
		fmt.Printf("Compiled %s\n", outputBinary)
		success = true
	}

	if !success {
		log.Fatal("Failed to compile.")
	}
}
