package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/aria-lang/aria/internal/checker"
	"github.com/aria-lang/aria/internal/codegen"
	"github.com/aria-lang/aria/internal/lexer"
	"github.com/aria-lang/aria/internal/parser"
	"github.com/aria-lang/aria/internal/resolver"
)

const version = "0.1.0-bootstrap"

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		printUsage()
		os.Exit(0)
	}

	cmd := args[0]
	cmdArgs := args[1:]

	// Parse global flags
	format := "text"
	var files []string
	for i := 0; i < len(cmdArgs); i++ {
		switch cmdArgs[i] {
		case "--format":
			if i+1 < len(cmdArgs) {
				i++
				format = cmdArgs[i]
			} else {
				fmt.Fprintln(os.Stderr, "error: --format requires a value (text or json)")
				os.Exit(1)
			}
		case "--format=text":
			format = "text"
		case "--format=json":
			format = "json"
		case "--version", "-v":
			fmt.Printf("aria %s\n", version)
			os.Exit(0)
		case "--help", "-h":
			printUsage()
			os.Exit(0)
		default:
			if cmdArgs[i][0] == '-' {
				// Check for --format=value pattern
				if len(cmdArgs[i]) > 9 && cmdArgs[i][:9] == "--format=" {
					format = cmdArgs[i][9:]
				} else {
					fmt.Fprintf(os.Stderr, "error: unknown flag %q\n", cmdArgs[i])
					os.Exit(1)
				}
			} else {
				files = append(files, cmdArgs[i])
			}
		}
	}

	switch cmd {
	case "--version", "-v":
		fmt.Printf("aria %s\n", version)
	case "--help", "-h":
		printUsage()
	case "lex":
		runLex(files, format)
	case "parse":
		runParse(files, format)
	case "check":
		runCheck(files, format)
	case "build":
		runBuild(files, format)
	case "run":
		runRun(files, format)
	case "test":
		runTest(files, format)
	default:
		fmt.Fprintf(os.Stderr, "error: unknown command %q\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`aria %s — the Aria bootstrap compiler

Usage: aria <command> [flags] [files...]

Commands:
  lex       Tokenize source files and dump token stream
  parse     Parse source files and dump AST
  check     Type-check source files (no code generation)
  build     Compile source files to executable
  run       Compile and run source files
  test      Compile and run test blocks

Flags:
  --format=text|json   Output format (default: text)
  --version, -v        Print version
  --help, -h           Print this help
`, version)
}

func requireFiles(files []string, cmd string) {
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "error: %s requires at least one source file\n", cmd)
		fmt.Fprintf(os.Stderr, "usage: aria %s [flags] <file.aria>\n", cmd)
		os.Exit(1)
	}
}

func runLex(files []string, format string) {
	requireFiles(files, "lex")
	for _, file := range files {
		source, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: cannot read file %q: %v\n", file, err)
			os.Exit(1)
		}

		l := lexer.New(file, string(source))
		tokens := l.Tokenize()

		if l.Diagnostics().HasErrors() {
			if format == "json" {
				l.Diagnostics().RenderJSON(os.Stderr)
			} else {
				l.Diagnostics().Render(os.Stderr)
			}
			os.Exit(1)
		}

		if format == "json" {
			type jsonToken struct {
				Type    string `json:"type"`
				Literal string `json:"literal,omitempty"`
				Line    int    `json:"line"`
				Column  int    `json:"column"`
			}
			var out []jsonToken
			for _, tok := range tokens {
				out = append(out, jsonToken{
					Type:    tok.Type.String(),
					Literal: tok.Literal,
					Line:    tok.Pos.Line,
					Column:  tok.Pos.Column,
				})
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(out)
		} else {
			for _, tok := range tokens {
				if tok.Type == lexer.EOF {
					break
				}
				fmt.Printf("%-4d:%-3d  %-16s %s\n", tok.Pos.Line, tok.Pos.Column, tok.Type, tok.Literal)
			}
		}
	}
}

func runParse(files []string, format string) {
	requireFiles(files, "parse")
	for _, file := range files {
		source, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: cannot read file %q: %v\n", file, err)
			os.Exit(1)
		}

		l := lexer.New(file, string(source))
		tokens := l.Tokenize()
		if l.Diagnostics().HasErrors() {
			l.Diagnostics().Render(os.Stderr)
			os.Exit(1)
		}

		p := parser.New(tokens)
		prog := p.Parse()
		if p.Diagnostics().HasErrors() {
			if format == "json" {
				p.Diagnostics().RenderJSON(os.Stderr)
			} else {
				p.Diagnostics().Render(os.Stderr)
			}
			os.Exit(1)
		}

		if format == "json" {
			out, _ := parser.FormatJSON(prog)
			fmt.Println(out)
		} else {
			fmt.Print(parser.FormatAST(prog))
		}
	}
}

func runCheck(files []string, format string) {
	requireFiles(files, "check")
	hasErrors := false
	for _, file := range files {
		source, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: cannot read file %q: %v\n", file, err)
			os.Exit(1)
		}

		l := lexer.New(file, string(source))
		tokens := l.Tokenize()
		if l.Diagnostics().HasErrors() {
			l.Diagnostics().Render(os.Stderr)
			hasErrors = true
			continue
		}

		p := parser.New(tokens)
		prog := p.Parse()
		if p.Diagnostics().HasErrors() {
			if format == "json" {
				p.Diagnostics().RenderJSON(os.Stderr)
			} else {
				p.Diagnostics().Render(os.Stderr)
			}
			hasErrors = true
			continue
		}

		r := resolver.New()
		scope := r.Resolve(prog)
		if r.Diagnostics().HasErrors() {
			if format == "json" {
				r.Diagnostics().RenderJSON(os.Stderr)
			} else {
				r.Diagnostics().Render(os.Stderr)
			}
			hasErrors = true
			continue
		}

		ch := checker.New(scope)
		ch.Check(prog)
		if ch.Diagnostics().HasErrors() {
			if format == "json" {
				ch.Diagnostics().RenderJSON(os.Stderr)
			} else {
				ch.Diagnostics().Render(os.Stderr)
			}
			hasErrors = true
			continue
		}

		fmt.Printf("%s: OK\n", file)
	}
	if hasErrors {
		os.Exit(1)
	}
}

// compileToGo runs the full pipeline (lex → parse → resolve → check → codegen)
// and returns the generated Go source code.
func compileToGo(file string, format string) (string, string, error) {
	source, err := os.ReadFile(file)
	if err != nil {
		return "", "", fmt.Errorf("cannot read file %q: %v", file, err)
	}

	l := lexer.New(file, string(source))
	tokens := l.Tokenize()
	if l.Diagnostics().HasErrors() {
		if format == "json" {
			l.Diagnostics().RenderJSON(os.Stderr)
		} else {
			l.Diagnostics().Render(os.Stderr)
		}
		return "", "", fmt.Errorf("lexer errors")
	}

	p := parser.New(tokens)
	prog := p.Parse()
	if p.Diagnostics().HasErrors() {
		if format == "json" {
			p.Diagnostics().RenderJSON(os.Stderr)
		} else {
			p.Diagnostics().Render(os.Stderr)
		}
		return "", "", fmt.Errorf("parse errors")
	}

	r := resolver.New()
	scope := r.Resolve(prog)
	if r.Diagnostics().HasErrors() {
		if format == "json" {
			r.Diagnostics().RenderJSON(os.Stderr)
		} else {
			r.Diagnostics().Render(os.Stderr)
		}
		return "", "", fmt.Errorf("resolution errors")
	}

	ch := checker.New(scope)
	ch.Check(prog)
	if ch.Diagnostics().HasErrors() {
		if format == "json" {
			ch.Diagnostics().RenderJSON(os.Stderr)
		} else {
			ch.Diagnostics().Render(os.Stderr)
		}
		return "", "", fmt.Errorf("type errors")
	}

	gen := codegen.New()
	goSrc := gen.Generate(prog)
	testSrc := gen.GenerateTest(prog)
	return goSrc, testSrc, nil
}

func runBuild(files []string, format string) {
	requireFiles(files, "build")

	goSrc, _, err := compileToGo(files[0], format)
	if err != nil {
		os.Exit(1)
	}

	// Determine output name from input file
	base := files[0]
	base = base[:len(base)-len(".aria")]
	if idx := len(base) - 1; idx >= 0 {
		for i := idx; i >= 0; i-- {
			if base[i] == '/' || base[i] == '\\' {
				base = base[i+1:]
				break
			}
		}
	}

	result, err := codegen.Build(goSrc, "", codegen.BuildOptions{
		OutputPath: base,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("built: %s\n", result.OutputPath)
}

func runRun(files []string, format string) {
	requireFiles(files, "run")

	goSrc, _, err := compileToGo(files[0], format)
	if err != nil {
		os.Exit(1)
	}

	exitCode, err := codegen.Run(goSrc, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	os.Exit(exitCode)
}

func runTest(files []string, format string) {
	requireFiles(files, "test")

	goSrc, testSrc, err := compileToGo(files[0], format)
	if err != nil {
		os.Exit(1)
	}

	if testSrc == "" {
		fmt.Println("no tests found")
		return
	}

	output, err := codegen.RunTests(goSrc, testSrc)
	fmt.Print(output)
	if err != nil {
		os.Exit(1)
	}
}
