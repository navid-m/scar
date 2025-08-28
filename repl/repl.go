// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Scar REPL: compiles accumulated input to C and executes it per entry.

package repl

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"scar/comptime"
	"scar/lexer"
	"scar/preprocessor"
	"scar/renderer"
	"strings"
)

func RunRepl() {
	fmt.Println("Scar REPL. Type :quit to exit, :reset to clear state.")

	scanner := bufio.NewScanner(os.Stdin)
	var sessionSrc strings.Builder

	for {
		fmt.Print("scar> ")
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		switch strings.TrimSpace(line) {
		case ":quit", ":exit":
			fmt.Println("bye")
			return
		case ":reset":
			sessionSrc.Reset()
			fmt.Println("state cleared")
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}

		sessionSrc.WriteString(line)
		sessionSrc.WriteString("\n")

		src := sessionSrc.String()
		baseDir, _ := os.Getwd()
		tmpDir, err := os.MkdirTemp("", "scarrepl-*")
		if err != nil {
			log.Fatalf("temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		scarPath := filepath.Join(tmpDir, "repl_session.scar")
		if err := os.WriteFile(scarPath, []byte(src), 0644); err != nil {
			log.Printf("write scar: %v", err)
			continue
		}

		processed := preprocessor.ProcessSourceLevelMacros(src)
		program, err := lexer.InnerParseWithIndentation(processed, scarPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s", "\033[31mSyntaxError: "+err.Error()+"\033[0m\n")
			continue
		}

		macros := preprocessor.MacroNames(processed)
		if verrs := lexer.ValidateProgramWithMacros(program, macros); len(verrs) > 0 {
			for _, e := range verrs {
				fmt.Fprintf(os.Stderr, "\033[31mComptimeError: %v\033[0m\n", e)
			}
			continue
		}

		var compileArgs []string
		cplr := "gcc"
		outputBin := filepath.Join(tmpDir, "repl_out")
		if runtime.GOOS == "windows" {
			outputBin += ".exe"
		}

		cCode := preprocessor.InsertMacros(renderer.RenderC(program, baseDir, false))

		cPath := filepath.Join(tmpDir, "repl_out.c")
		if err := os.WriteFile(cPath, []byte(cCode), 0644); err != nil {
			log.Printf("write c: %v", err)
			continue
		}

		switch runtime.GOOS {
		case "darwin":
			cplr = "/opt/homebrew/opt/llvm/bin/clang"
			compileArgs = []string{
				"-w", "-fopenmp", "-g", "-fno-omit-frame-pointer", "-fstack-protector-strong",
				"-I/opt/homebrew/opt/libomp/include", "-I/opt/homebrew/include",
				cPath, "-L/opt/homebrew/opt/libomp/lib", "-L/opt/homebrew/lib", "-o", outputBin,
				"-pthread",
			}
		case "linux":
			compileArgs = []string{"-fopenmp", "-g", "-fno-omit-frame-pointer", "-fstack-protector-strong", cPath, "-o", outputBin}
		case "windows":
			compileArgs = []string{"-fopenmp", "-g", "-fno-omit-frame-pointer", "-fstack-protector-strong", "-w", cPath, "-o", outputBin, "-ldbghelp"}
		default:
			compileArgs = []string{"-w", "-g", cPath, "-o", outputBin}
		}

		if preprocessor.ContainsExternalCurl(processed) {
			compileArgs = append(compileArgs, "-lcurl")
		}
		if preprocessor.ContainsExternalRegex(processed) {
			compileArgs = append(compileArgs, "-lpcre")
		}
		if preprocessor.ContainsExternalJson(processed) {
			compileArgs = append(compileArgs, "-ljansson")
		}
		if preprocessor.ContainsExternalNet(processed) && runtime.GOOS == "windows" {
			compileArgs = append(compileArgs, "-lws2_32")
		}

		success := comptime.RunCompilerWithMappedErrors(cplr, compileArgs, cCode, cPath, scarPath, processed)
		if !success {
			continue
		}
		cmd := exec.Command(outputBin)
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "run error: %v\n", err)
			continue
		}
		go io.Copy(os.Stdout, stdout)
		go io.Copy(os.Stderr, stderr)
		_ = cmd.Wait()
	}
}
