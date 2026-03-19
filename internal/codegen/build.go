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

// Build generates Go source, compiles it, and produces a binary.
func Build(goSource string, testSource string, opts BuildOptions) (*BuildResult, error) {
	// Create temp directory for generated Go project
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

	// Write main.go
	mainPath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainPath, []byte(goSource), 0644); err != nil {
		return nil, fmt.Errorf("failed to write main.go: %w", err)
	}

	// Optionally format the generated Go source
	fmtCmd := exec.Command("gofmt", "-w", mainPath)
	fmtCmd.Run() // ignore errors; formatting is best-effort

	// Write test file if present
	if testSource != "" {
		testPath := filepath.Join(tmpDir, "main_test.go")
		if err := os.WriteFile(testPath, []byte(testSource), 0644); err != nil {
			return nil, fmt.Errorf("failed to write test file: %w", err)
		}
		fmtCmd := exec.Command("gofmt", "-w", testPath)
		fmtCmd.Run()
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

// Run generates Go source, compiles, executes, and cleans up.
func Run(goSource string, args []string) (int, error) {
	tmpDir, err := os.MkdirTemp("", "aria-run-*")
	if err != nil {
		return 1, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	goMod := "module aria_generated\n\ngo 1.21\n"
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644)
	mainPath := filepath.Join(tmpDir, "main.go")
	os.WriteFile(mainPath, []byte(goSource), 0644)
	exec.Command("gofmt", "-w", mainPath).Run()

	binaryPath := filepath.Join(tmpDir, "aria_run")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = tmpDir
	buildCmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	output, err := buildCmd.CombinedOutput()
	if err != nil {
		return 1, fmt.Errorf("compilation failed:\n%s", string(output))
	}

	// Execute the binary
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

// RunTests generates Go source with test functions, runs `go test`.
func RunTests(goSource string, testSource string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "aria-test-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	goMod := "module aria_generated\n\ngo 1.21\n"
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644)
	mainPath := filepath.Join(tmpDir, "main.go")
	os.WriteFile(mainPath, []byte(goSource), 0644)
	exec.Command("gofmt", "-w", mainPath).Run()

	if testSource != "" {
		testPath := filepath.Join(tmpDir, "main_test.go")
		os.WriteFile(testPath, []byte(testSource), 0644)
		exec.Command("gofmt", "-w", testPath).Run()
	}

	testCmd := exec.Command("go", "test", "-v", ".")
	testCmd.Dir = tmpDir
	output, err := testCmd.CombinedOutput()

	// Parse Go test output and reformat for Aria
	result := formatTestOutput(string(output))
	if err != nil {
		return result, fmt.Errorf("tests failed")
	}
	return result, nil
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
	// "--- PASS: TestCircleArea (0.00s)" -> "circle area"
	parts := strings.SplitN(line, ":", 2)
	if len(parts) < 2 {
		return line
	}
	name := strings.TrimSpace(parts[1])
	if idx := strings.Index(name, " ("); idx != -1 {
		name = name[:idx]
	}
	// Remove "Test" prefix and convert to readable
	name = strings.TrimPrefix(name, "Test")
	// Convert CamelCase to spaces
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
