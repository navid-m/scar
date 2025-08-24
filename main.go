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
	"scar/comptime"
	"scar/lexer"
	"scar/logger"
	"scar/meta"
	"scar/preprocessor"
	"scar/renderer"
	"strings"
)

func main() {
	flag.Usage = meta.ShowUsage
	var (
		asm     = flag.Bool("asm", false, "show assembly output")
		c       = flag.Bool("c", false, "show c output")
		debug   = flag.Bool("d", false, "enable verbose logging")
		dll     = flag.Bool("dll", false, "compile as dynamic link library")
		gc      = flag.Bool("gc", false, "use bdwgc garbage collector")
		keepc   = flag.Bool("keepc", false, "keep generated c file")
		linker  = flag.String("l", "", "additional linker options (e.g., -lm -lpthread)")
		outName = flag.String("o", "", "output binary name")
		opt     = flag.Bool("opt", false, "optimise for performance")
		version = flag.Bool("v", false, "show version")
		nowin   *bool
		asan    *bool
	)

	if runtime.GOOS == "windows" {
		nowin = flag.Bool("nowin", false, "disable native windows headers")
	} else {
		asan = flag.Bool("asan", false, "enable address/undefined sanitizers")
	}

	flag.Parse()

	if runtime.GOOS == "windows" {
		if *nowin {
			comptime.WinEnabled = false
		}
	}

	if *debug {
		logger.Loud = true
	}

	if *version {
		fmt.Println(meta.Version)
		return
	}

	if len(flag.Args()) < 1 {
		meta.ShowUsage()
		return
	}

	var (
		input    string
		baseDir  string
		ptf      string
		extCflag string
	)

	if len(flag.Args()) > 0 {
		wd, _ := os.Getwd()
		ptf = path.Join(wd, flag.Arg(0))
		baseDir = filepath.Dir(ptf)
		data, err := os.ReadFile(ptf + ".scar")
		if err != nil {
			data, err = os.ReadFile(ptf)
			if err != nil {
				log.Fatal("Could not find file.")
			}
		}
		input = string(data)
	}

	var (
		hasCurl     = preprocessor.ContainsExternalCurlWithPath(input, baseDir)
		hasRegex    = preprocessor.ContainsExternalRegexWithPath(input, baseDir)
		hasJson     = preprocessor.ContainsExternalJsonWithPath(input, baseDir)
		hasNet      = preprocessor.ContainsExternalNetWithPath(input, baseDir)
		cleanedName = strings.ReplaceAll(filepath.Base(ptf), ".scar", "")
		outputName  = cleanedName
	)

	if *outName != "" {
		outputName = *outName
	}

	input = preprocessor.ProcessSourceLevelMacros(input)
	program, err := lexer.InnerParseWithIndentation(input, ptf)

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", "\033[31mSyntaxError: "+err.Error()+"\033[0m")
		os.Exit(1)
	}

	validationErrors := lexer.ValidateProgram(program)
	if len(validationErrors) > 0 {
		for _, err := range validationErrors {
			fmt.Fprintf(os.Stderr, "\033[31mComptimeError: %v\033[0m\n", err)
		}
		log.Fatal("Failed to compile.")
	}

	if preprocessor.ContainsExternalCurl(input) {
		extCflag = "-lcurl"
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
		outputBinary = "./" + outputName
		cmpPath      = "gcc"
		compileArgs  = []string{"-w", "-fopenmp", "-g", "-fno-omit-frame-pointer", "-fstack-protector-strong", tmpCPath, "-o", outputBinary, extCflag}
	)

	if *opt {
		compileArgs = append([]string{"-O2", "-fno-fast-math"}, compileArgs...)
	}

	if runtime.GOOS != "windows" {
		if *asan {
			compileArgs = append([]string{"-fsanitize=address,undefined"}, compileArgs...)
		}
	}

	if *linker != "" {
		linkerOpts := strings.Fields(*linker)
		compileArgs = append(compileArgs, linkerOpts...)
	}

	if *dll {
		compileArgs = append(compileArgs, "-shared", "-fPIC")
		outputBinary = "./" + outputName + ".so"
		for i, arg := range compileArgs {
			if arg == "-o" && i+1 < len(compileArgs) {
				compileArgs[i+1] = outputBinary
				break
			}
		}
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

	switch runtime.GOOS {
	case "darwin":
		cmpPath = "/opt/homebrew/opt/llvm/bin/clang"

		compileArgs = []string{
			"-w",
			"-fopenmp",
			"-g",
			"-fno-omit-frame-pointer",
			"-fstack-protector-strong",
			"-I/opt/homebrew/opt/libomp/include",
			"-I/opt/homebrew/include",
			tmpCPath,
			"-L/opt/homebrew/opt/libomp/lib",
			"-L/opt/homebrew/lib",
			"-o", "./" + outputName,
			"-pthread",
		}

		if *opt {
			compileArgs = append([]string{"-O2", "-fno-fast-math"}, compileArgs...)
		}
		if *asan {
			compileArgs = append([]string{"-fsanitize=address,undefined"}, compileArgs...)
		}

		if *dll {
			compileArgs = append(compileArgs, "-shared", "-fPIC")
			compileArgs[len(compileArgs)-2] = "./" + outputName + ".dylib"
		}
		if hasCurl {
			compileArgs = append(compileArgs, "-lcurl")
		}
		if hasRegex {
			compileArgs = append(compileArgs, "-lpcre")
		}
		if hasJson {
			compileArgs = append(compileArgs, "-ljansson")
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
			"-g",
			"-fno-omit-frame-pointer",
			"-fstack-protector-strong",
			tmpCPath,
			"-o", outputBinary,
		}
		if *opt {
			compileArgs = append([]string{"-O2", "-fno-fast-math"}, compileArgs...)
		}
		if *asan {
			compileArgs = append([]string{"-fsanitize=address,undefined"}, compileArgs...)
		}
		if *dll {
			compileArgs = append(compileArgs, "-shared", "-fPIC")
			outputBinary = "./" + outputName + ".so"
			for i, arg := range compileArgs {
				if arg == "-o" && i+1 < len(compileArgs) {
					compileArgs[i+1] = outputBinary
					break
				}
			}
		}
		if hasCurl {
			compileArgs = append(compileArgs, "-lcurl")
		}
		if hasRegex {
			compileArgs = append(compileArgs, "-lpcre")
		}
		if hasJson {
			compileArgs = append(compileArgs, "-ljansson")
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
		if !*dll {
			outputBinary = "./" + outputName + ".exe"
		}

		compileArgs = []string{
			"-fopenmp",
			"-g",
			"-fno-omit-frame-pointer",
			"-fstack-protector-strong",
		}

		if hasNet {
			compileArgs = append(compileArgs, "-Wall", "-Wextra")
		}

		compileArgs = append(compileArgs, "-w")

		if *opt {
			compileArgs = append([]string{"-O2", "-fno-fast-math"}, compileArgs...)
		}

		compileArgs = append(compileArgs, tmpCPath)
		compileArgs = append(compileArgs, "-o", outputBinary)

		if hasNet {
			compileArgs = append(compileArgs, "-lws2_32")
		}

		compileArgs = append(compileArgs, "-ldbghelp")

		if *linker != "" {
			linkerOpts := strings.Fields(*linker)
			compileArgs = append(compileArgs, linkerOpts...)
		}

		if *dll {
			compileArgs = append(compileArgs, "-shared")
			outputBinary = "./" + outputName + ".dll"
			for i, arg := range compileArgs {
				if arg == "-o" && i+1 < len(compileArgs) {
					compileArgs[i+1] = outputBinary
					break
				}
			}
		}

		if hasCurl {
			compileArgs = append(compileArgs, "-lcurl")
		}
		if hasRegex {
			compileArgs = append(compileArgs, "-lpcre")
		}
		if hasJson {
			compileArgs = append(compileArgs, "-ljansson")
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

	success := comptime.RunCompilerWithMappedErrors(cmpPath, compileArgs, cCode, tmpCPath, ptf, input)
	if success {
		fmt.Printf("Compiled %s\n", outputBinary)
		if *keepc {
			fmt.Printf("C file kept as %s\n", tmpCPath)
		}
	} else {
		log.Fatal("Failed to compile.")
	}
}
