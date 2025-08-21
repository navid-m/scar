// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Captures GCC/Clang stderr and reports relevant errors.

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func runCompilerWithMappedErrors(compiler string, args []string, cCode string, cFilePath string, scarPath string, scarSource string) bool {
	if cFilePath == "" {
		base := strings.TrimSuffix(filepath.Base(scarPath), ".scar")
		cFilePath = base + ".c"
	}

	cmd := exec.Command(compiler, args...)
	var stderr bytes.Buffer
	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return true
	}

	reportCompilerErrors(stderr.String(), cFilePath, scarPath, scarSource, cCode)
	return false
}

var (
	gccClangErrRe = regexp.MustCompile(`^(?P<file>[^:\n]+):(\d+)(?::(\d+))?:\s*(fatal error|error|warning|note):\s*(?P<msg>.*)$`)
)

func reportCompilerErrors(stderrStr, cPath, scarPath, scarSource, cCode string) {
	lines := splitNonEmptyLines(stderrStr)
	if len(lines) == 0 {
		fmt.Fprint(os.Stderr, stderrStr)
		return
	}

	var relevant []string
	for _, l := range lines {
		if strings.Contains(l, cPath) || (!strings.Contains(l, ":") && len(relevant) > 0) {
			relevant = append(relevant, l)
		}
	}
	if len(relevant) == 0 {
		relevant = lines
	}

	var (
		cErrFile string
		cErrLine int
		cErrCol  int
		cErrMsg  string
	)
	for _, l := range relevant {
		m := gccClangErrRe.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		cErrFile = m[1]
		cErrLine = atoiSafe(m[2])
		if len(m) > 3 {
			cErrCol = atoiSafe(m[3])
		}
		cErrMsg = m[len(m)-1]
		if strings.TrimSpace(cErrFile) != "" {
			cPath = cErrFile
		}
		break
	}

	if cErrLine == 0 {
		fmt.Fprintf(os.Stderr, "\n\x1b[31mC Compiler Error(s)\x1b[0m\n")
		for _, l := range relevant {
			fmt.Fprintln(os.Stderr, l)
		}
		return
	}

	scarLine := inferScarLineFromC(cCode, cErrLine)

	fmt.Fprintf(os.Stderr, "\n\x1b[31mPCompileError\x1b[0m: %s\n", strings.TrimSpace(cErrMsg))
	if scarLine > 0 {
		printScarContext(scarSource, scarPath, scarLine, 2)
	} else {
		printCContext(cCode, cPath, cErrLine, cErrCol, 2)
	}
}

func splitNonEmptyLines(s string) []string {
	var out []string
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		t := scanner.Text()
		if strings.TrimSpace(t) != "" {
			out = append(out, t)
		}
	}
	return out
}

func atoiSafe(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch < '0' || ch > '9' {
			break
		}
		n = n*10 + int(ch-'0')
	}
	return n
}

// A best-effort mapper that searches backward from the C error line
// for a comment marker of the form:
//
//	// scar: <path>:<line>
//
// If not found, returns 0.
func inferScarLineFromC(cCode string, cErrLine int) int {
	if cErrLine <= 0 {
		return 0
	}
	lines := strings.Split(cCode, "\n")
	idx := cErrLine - 1
	if idx >= len(lines) {
		idx = len(lines) - 1
	}
	re := regexp.MustCompile(`^\s*//\s*scar:\s*.*?:(\d+)\s*$`)
	for i := idx; i >= 0 && i >= idx-20; i-- {
		m := re.FindStringSubmatch(lines[i])
		if m != nil {
			return atoiSafe(m[1])
		}
	}
	return 0
}

func printScarContext(src, path string, line, radius int) {
	lines := strings.Split(src, "\n")
	start := line - radius
	if start < 1 {
		start = 1
	}
	end := line + radius
	if end > len(lines) {
		end = len(lines)
	}
	fmt.Fprintf(os.Stderr, "in %s:%d\n", path, line)
	for i := start; i <= end; i++ {
		prefix := "   "
		if i == line {
			prefix = " > "
		}
		fmt.Fprintf(os.Stderr, "%s%5d | %s\n", prefix, i, lines[i-1])
	}
}

func printCContext(src, path string, line, col, radius int) {
	lines := strings.Split(src, "\n")
	if line < 1 || line > len(lines) {
		fmt.Fprintf(os.Stderr, "at %s:%d:%d\n", path, line, col)
		return
	}
	start := line - radius
	if start < 1 {
		start = 1
	}
	end := line + radius
	if end > len(lines) {
		end = len(lines)
	}
	fmt.Fprintf(os.Stderr, "at %s:%d:%d\n", path, line, col)
	for i := start; i <= end; i++ {
		prefix := "   "
		if i == line {
			prefix = " > "
		}
		fmt.Fprintf(os.Stderr, "%s%5d | %s\n", prefix, i, lines[i-1])
	}
}
