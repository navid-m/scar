// By Navid M (c)
// Date: 2025
// License: GPL3
//
// Captures stderr and reports relevant errors.

package comptime

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

func RunCompilerWithMappedErrors(
	compiler string,
	args []string,
	cCode string,
	cFilePath string,
	scarPath string,
	scarSource string,
) bool {
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

func extractQuotedIdentifier(msg string) string {
	i := strings.Index(msg, "'")
	if i == -1 {
		return ""
	}
	j := strings.Index(msg[i+1:], "'")
	if j == -1 {
		return ""
	}
	cand := msg[i+1 : i+1+j]
	if len(cand) == 0 {
		return ""
	}
	if (cand[0] >= 'A' && cand[0] <= 'Z') || (cand[0] >= 'a' && cand[0] <= 'z') || cand[0] == '_' {
		return cand
	}
	return ""
}

func findScarLineBySymbol(scarSrc, symbol string) int {
	lines := strings.Split(scarSrc, "\n")
	best := 0
	for i, ln := range lines {
		if !containsWord(ln, symbol) {
			continue
		}
		low := strings.ToLower(ln)
		score := 0
		if strings.Contains(low, "print") || strings.Contains(low, "fmt") {
			score += 2
		}
		if strings.Contains(ln, symbol) {
			score++
		}
		if score > 0 {
			return i + 1
		}
		if best == 0 {
			best = i + 1
		}
	}
	return best
}

func containsWord(line, word string) bool {
	if word == "" {
		return false
	}
	f := func(r rune) bool {
		return !(r == '_' || (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'))
	}
	fields := strings.FieldsFunc(line, f)
	return slices.Contains(fields, word)
}

func firstMeaningfulScarLine(s string) int {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		return i + 1
	}
	return 0
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
		cErrMsg  string
	)
	for _, l := range relevant {
		m := gccClangErrRe.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		cErrFile = m[1]
		cErrLine = atoiSafe(m[2])
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

		if sym := extractQuotedIdentifier(cErrMsg); sym != "" {
			if ln := findScarLineBySymbol(scarSource, sym); ln > 0 {
				printScarContext(scarSource, scarPath, ln, 2)
				return
			}
		}

		if ln := firstMeaningfulScarLine(scarSource); ln > 0 {
			printScarContext(scarSource, scarPath, ln, 2)
			return
		}

		fmt.Fprintf(os.Stderr, "in %s\n", scarPath)
		return
	}

	scarLine := inferScarLineFromC(cCode, cErrLine)

	fmt.Fprintf(
		os.Stderr,
		"\x1b[31mCompError: %s\x1b[0m\n",
		strings.ReplaceAll(strings.TrimSpace(cErrMsg), "(first use in this function)", ""),
	)
	if scarLine > 0 {
		printScarContext(scarSource, scarPath, scarLine, 2)
		return
	}
	if sym := extractQuotedIdentifier(cErrMsg); sym != "" {
		if ln := findScarLineBySymbol(scarSource, sym); ln > 0 {
			printScarContext(scarSource, scarPath, ln, 2)
			return
		}
	}
	if ln := firstMeaningfulScarLine(scarSource); ln > 0 {
		printScarContext(scarSource, scarPath, ln, 2)
		return
	}
	fmt.Fprintf(os.Stderr, "in %s\n", scarPath)
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
	var (
		lines = strings.Split(src, "\n")
		start = max(line-radius, 1)
		end   = min(line+radius, len(lines))
	)
	fmt.Fprintf(os.Stderr, "in %s:%d\n", path, line)
	for i := start; i <= end; i++ {
		prefix := "   "
		if i == line {
			prefix = " > "
		}
		fmt.Fprintf(os.Stderr, "%s%5d | %s\n", prefix, i, lines[i-1])
	}
}
