package codegen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// BuildResult contains the result of a build operation.
type BuildResult struct {
	OutputPath   string
	GeneratedDir string // set if --keep-generated
}

// BuildOptions configures the build process.
type BuildOptions struct {
	OutputPath    string // output binary path
	KeepGenerated bool   // keep the generated Go project
	Verbose       bool
}

// GoFile represents a generated Go source file.
type GoFile struct {
	Name   string // e.g., "main.go", "lexer.go"
	Source string // Go source code
}

// BuildMulti compiles multiple Go source files into a single binary.
func BuildMulti(goFiles []GoFile, testFiles []GoFile, opts BuildOptions) (*BuildResult, error) {
	tmpDir, err := os.MkdirTemp("", "aria-build-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	if !opts.KeepGenerated {
		defer os.RemoveAll(tmpDir)
	}

	// Write go.mod
	goMod := "module aria_generated\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return nil, fmt.Errorf("failed to write go.mod: %w", err)
	}

	// Write all Go source files
	for _, gf := range goFiles {
		path := filepath.Join(tmpDir, gf.Name)
		if err := os.WriteFile(path, []byte(gf.Source), 0644); err != nil {
			return nil, fmt.Errorf("failed to write %s: %w", gf.Name, err)
		}
		exec.Command("gofmt", "-w", path).Run()
	}

	// Write test files
	for _, tf := range testFiles {
		path := filepath.Join(tmpDir, tf.Name)
		if err := os.WriteFile(path, []byte(tf.Source), 0644); err != nil {
			return nil, fmt.Errorf("failed to write %s: %w", tf.Name, err)
		}
		exec.Command("gofmt", "-w", path).Run()
	}

	// Determine output path
	outputPath := opts.OutputPath
	if outputPath == "" {
		outputPath = "aria_output"
	}
	absOutput, err := filepath.Abs(outputPath)
	if err != nil {
		absOutput = outputPath
	}

	// Run go build
	buildCmd := exec.Command("go", "build", "-o", absOutput, ".")
	buildCmd.Dir = tmpDir
	buildCmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	output, err := buildCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("go build failed:\n%s", string(output))
	}

	result := &BuildResult{OutputPath: absOutput}
	if opts.KeepGenerated {
		result.GeneratedDir = tmpDir
	}
	return result, nil
}

// Build is the single-file convenience wrapper.
func Build(goSource string, testSource string, opts BuildOptions) (*BuildResult, error) {
	goFiles := []GoFile{{Name: "main.go", Source: goSource}}
	var testFiles []GoFile
	if testSource != "" {
		testFiles = []GoFile{{Name: "main_test.go", Source: testSource}}
	}
	return BuildMulti(goFiles, testFiles, opts)
}

// RunMulti compiles multiple Go source files and executes the result.
func RunMulti(goFiles []GoFile, args []string) (int, error) {
	tmpDir, err := os.MkdirTemp("", "aria-run-*")
	if err != nil {
		return 1, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	goMod := "module aria_generated\n\ngo 1.21\n"
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644)

	for _, gf := range goFiles {
		path := filepath.Join(tmpDir, gf.Name)
		os.WriteFile(path, []byte(gf.Source), 0644)
		exec.Command("gofmt", "-w", path).Run()
	}

	binaryPath := filepath.Join(tmpDir, "aria_run")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = tmpDir
	buildCmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	output, err := buildCmd.CombinedOutput()
	if err != nil {
		return 1, fmt.Errorf("compilation failed:\n%s", string(output))
	}

	runCmd := exec.Command(binaryPath)
	runCmd.Args = append(runCmd.Args, args...)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	runCmd.Stdin = os.Stdin

	err = runCmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}

// Run is the single-file convenience wrapper.
func Run(goSource string, args []string) (int, error) {
	return RunMulti([]GoFile{{Name: "main.go", Source: goSource}}, args)
}

// RunTestsMulti compiles multiple Go source files with tests and runs them.
func RunTestsMulti(goFiles []GoFile, testFiles []GoFile) (string, error) {
	tmpDir, err := os.MkdirTemp("", "aria-test-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	goMod := "module aria_generated\n\ngo 1.21\n"
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644)

	for _, gf := range goFiles {
		path := filepath.Join(tmpDir, gf.Name)
		os.WriteFile(path, []byte(gf.Source), 0644)
		exec.Command("gofmt", "-w", path).Run()
	}
	for _, tf := range testFiles {
		path := filepath.Join(tmpDir, tf.Name)
		os.WriteFile(path, []byte(tf.Source), 0644)
		exec.Command("gofmt", "-w", path).Run()
	}

	testCmd := exec.Command("go", "test", "-v", ".")
	testCmd.Dir = tmpDir
	output, err := testCmd.CombinedOutput()

	result := formatTestOutput(string(output))
	if err != nil {
		return result, fmt.Errorf("tests failed")
	}
	return result, nil
}

// RunTests is the single-file convenience wrapper.
func RunTests(goSource string, testSource string) (string, error) {
	goFiles := []GoFile{{Name: "main.go", Source: goSource}}
	var testFiles []GoFile
	if testSource != "" {
		testFiles = []GoFile{{Name: "main_test.go", Source: testSource}}
	}
	return RunTestsMulti(goFiles, testFiles)
}

func formatTestOutput(goOutput string) string {
	var sb strings.Builder
	lines := strings.Split(goOutput, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "--- PASS:") {
			name := extractTestName(line)
			sb.WriteString(fmt.Sprintf("  ✓ %s\n", name))
		} else if strings.HasPrefix(line, "--- FAIL:") {
			name := extractTestName(line)
			sb.WriteString(fmt.Sprintf("  ✗ %s\n", name))
		} else if strings.HasPrefix(line, "PASS") {
			sb.WriteString("\nAll tests passed.\n")
		} else if strings.HasPrefix(line, "FAIL") && !strings.HasPrefix(line, "--- FAIL:") {
			sb.WriteString("\nSome tests failed.\n")
		}
	}
	return sb.String()
}

func extractTestName(line string) string {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) < 2 {
		return line
	}
	name := strings.TrimSpace(parts[1])
	if idx := strings.Index(name, " ("); idx != -1 {
		name = name[:idx]
	}
	name = strings.TrimPrefix(name, "Test")
	var sb strings.Builder
	for i, c := range name {
		if i > 0 && c >= 'A' && c <= 'Z' {
			sb.WriteRune(' ')
		}
		sb.WriteRune(c)
	}
	result := strings.ToLower(sb.String())
	return strings.TrimSpace(result)
}
