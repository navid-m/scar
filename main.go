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
	var (
		asm   = flag.Bool("asm", false, "show assembly output")
		c     = flag.Bool("c", false, "show c output")
		gc    = flag.Bool("gc", false, "use bdwgc garbage collector")
		keepc = flag.Bool("keepc", false, "keep generated c file")
	)
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
		data, err := os.ReadFile(ptf + ".scar")
		if err != nil {
			log.Fatal("Could not find file.")
		}
		input = string(data)
	}

	cleanedName := strings.ReplaceAll(filepath.Base(ptf), ".scar", "")
	input = preprocessor.ProcessSourceLevelMacros(input)
	program, err := lexer.ParseWithIndentation(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", "\033[31mSyntaxError: "+err.Error()+"\033[0m")
		os.Exit(1)
	}

	validationErrors := lexer.ValidateProgram(program)
	if len(validationErrors) > 0 {
		for _, err := range validationErrors {
			fmt.Fprintf(os.Stderr, "\033[31m%v\033[0m\n", err)
		}
		log.Fatal("Failed to compile.")
	}

	cCode := preprocessor.InsertMacros(renderer.RenderC(program, baseDir, *gc))

	if *asm {
		cplr := "clang"
		if runtime.GOOS == "windows" {
			cplr = "gcc"
		}
		cmd := exec.Command(cplr, "-w", "-S", "-x", "c", "-o", "-", "-")
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
	if !*keepc {
		defer os.Remove(tmpCPath)
	}

	var (
		outputBinary = "./" + cleanedName
		cmpPath      = "clang"
		compileArgs  = []string{"-w", "-fopenmp", tmpCPath, "-o", outputBinary}
	)

	if *gc {
		if gcFlags := findBundledBoehm(); gcFlags != nil {
			compileArgs = append(compileArgs, gcFlags...)
		} else if gcFlags := tryPkgConfig("bdw-gc"); gcFlags != nil {
			compileArgs = append(compileArgs, gcFlags...)
		} else {
			compileArgs = append(compileArgs, "-lgc")
		}
	}

	switch runtime.GOOS {
	case "darwin":
		cmpPath = "/opt/homebrew/opt/llvm/bin/clang"
		compileArgs = []string{
			"-w",
			"-fopenmp",
			tmpCPath,
			"-I/opt/homebrew/opt/libomp/include",
			"-L/opt/homebrew/opt/libomp/lib",
			"-o", outputBinary,
		}
		if *gc {
			if gcFlags := findBundledBoehm(); gcFlags != nil {
				compileArgs = append(compileArgs, gcFlags...)
			} else if gcFlags := tryPkgConfig("bdw-gc"); gcFlags != nil {
				compileArgs = append(compileArgs, gcFlags...)
			} else {
				compileArgs = append(compileArgs, "-lgc")
			}
		}
	case "linux":
		compileArgs = []string{
			"-fopenmp",
			tmpCPath,
			"-o", outputBinary,
		}
		if *gc {
			if gcFlags := findBundledBoehm(); gcFlags != nil {
				compileArgs = append(compileArgs, gcFlags...)
			} else if gcFlags := tryPkgConfig("bdw-gc"); gcFlags != nil {
				compileArgs = append(compileArgs, gcFlags...)
			} else {
				compileArgs = append(compileArgs, "-lgc")
			}
		}
	case "windows":
		cmpPath = "gcc"
		outputBinary += ".exe"
		compileArgs = []string{
			"-fopenmp",
			"-w",
			tmpCPath,
			"-o", outputBinary,
		}
		if *gc {
			if gcFlags := findBundledBoehm(); gcFlags != nil {
				compileArgs = append(compileArgs, gcFlags...)
			} else if gcFlags := findMinGWBoehm(); gcFlags != nil {
				compileArgs = append(compileArgs, gcFlags...)
			} else {
				compileArgs = append(compileArgs, "-lgc")
			}
		}
	}

	cmd := exec.Command(cmpPath, compileArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	success := false

	if err == nil {
		fmt.Printf("Compiled %s\n", outputBinary)
		if *keepc {
			fmt.Printf("C file kept as %s\n", tmpCPath)
		}
		success = true
	}

	if !success {
		log.Fatal("Failed to compile.")
	}
}
