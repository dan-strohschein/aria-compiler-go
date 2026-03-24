package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

Usage: aria <command> [flags] [files/directories...]

Commands:
  lex       Tokenize source files and dump token stream
  parse     Parse source files and dump AST
  check     Type-check source files (no code generation)
  build     Compile source files to executable
  run       Compile and run source files
  test      Compile and run test blocks

Multi-file: Pass a directory or multiple .aria files to compile together.
  aria build .                  # compile all .aria files in current dir
  aria build src/               # compile all .aria files in src/
  aria run main.aria lib.aria   # compile specific files together

Flags:
  --format=text|json   Output format (default: text)
  --version, -v        Print version
  --help, -h           Print this help
`, version)
}

func requireFiles(files []string, cmd string) {
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "error: %s requires at least one source file or directory\n", cmd)
		fmt.Fprintf(os.Stderr, "usage: aria %s [flags] <file.aria | directory>\n", cmd)
		os.Exit(1)
	}
}

// discoverAriaFiles expands file arguments into a list of .aria files.
// If a directory is given, all .aria files in that directory are included.
// If a single .aria file is given and other .aria files exist in the same
// directory, they are all included for multi-file compilation.
func discoverAriaFiles(inputs []string) []string {
	var files []string
	seen := make(map[string]bool)

	for _, input := range inputs {
		info, err := os.Stat(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: cannot access %q: %v\n", input, err)
			os.Exit(1)
		}

		if info.IsDir() {
			// Directory: find all .aria files recursively
			filepath.WalkDir(input, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if !d.IsDir() && strings.HasSuffix(d.Name(), ".aria") {
					if !seen[path] {
						files = append(files, path)
						seen[path] = true
					}
				}
				return nil
			})
		} else if strings.HasSuffix(input, ".aria") {
			absPath, _ := filepath.Abs(input)
			if !seen[absPath] {
				files = append(files, input)
				seen[absPath] = true
			}
		}
	}

	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "error: no .aria files found")
		os.Exit(1)
	}
	return files
}

// ---------- Single-file commands ----------

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

// ---------- Multi-file pipeline ----------

// parsedFile holds the parse result for one .aria file.
type parsedFile struct {
	path string
	prog *parser.Program
}

// compileProject runs the full multi-file pipeline.
// Returns Go source files for all Aria files.
func compileProject(inputs []string, format string) ([]codegen.GoFile, []codegen.GoFile) {
	files := discoverAriaFiles(inputs)

	// Phase 1: Parse all files
	var parsed []parsedFile
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
			p.Diagnostics().Render(os.Stderr)
			hasErrors = true
			continue
		}

		parsed = append(parsed, parsedFile{path: file, prog: prog})
	}
	if hasErrors {
		os.Exit(1)
	}

	// Phase 2: Resolve all files together.
	// First pass: register top-level declarations from ALL files.
	r := resolver.New()
	allProgs := make([]*parser.Program, len(parsed))
	for i, pf := range parsed {
		allProgs[i] = pf.prog
	}
	scope := r.ResolveMulti(allProgs)
	if r.Diagnostics().HasErrors() {
		r.Diagnostics().Render(os.Stderr)
		os.Exit(1)
	}

	// Phase 3: Type-check all files together.
	ch := checker.New(scope)
	for _, pf := range parsed {
		ch.Check(pf.prog)
	}
	if ch.Diagnostics().HasErrors() {
		ch.Diagnostics().Render(os.Stderr)
		os.Exit(1)
	}
	exprTypes := ch.ExprTypes()

	// Phase 4: Generate Go source.
	// Find which file has the entry block — that becomes main.go.
	// Other files become module_name.go.
	var goFiles []codegen.GoFile
	var testFiles []codegen.GoFile

	// First pass: collect all types across files
	typeCollector := codegen.New()
	for _, pf := range parsed {
		typeCollector.RegisterProgramTypes(pf.prog)
	}
	sharedTypes := typeCollector.GetTypes()

	entryFound := false
	for _, pf := range parsed {
		hasEntry := false
		for _, decl := range pf.prog.Decls {
			if _, ok := decl.(*parser.EntryBlock); ok {
				hasEntry = true
				break
			}
		}

		goName := ariaToGoFilename(pf.path)

		if hasEntry {
			entryFound = true
			gen := codegen.NewWithTypes(sharedTypes)
			gen.SetExprTypes(exprTypes)
			goSrc := gen.Generate(pf.prog)
			goFiles = append(goFiles, codegen.GoFile{Name: goName, Source: goSrc})

			testSrc := gen.GenerateTest(pf.prog)
			if testSrc != "" {
				testName := strings.TrimSuffix(goName, ".go") + "_test.go"
				testFiles = append(testFiles, codegen.GoFile{Name: testName, Source: testSrc})
			}
		} else {
			gen := codegen.NewWithTypes(sharedTypes)
			gen.SetExprTypes(exprTypes)
			goSrc := gen.GenerateModule(pf.prog)
			goFiles = append(goFiles, codegen.GoFile{Name: goName, Source: goSrc})

			testSrc := gen.GenerateTest(pf.prog)
			if testSrc != "" {
				testName := strings.TrimSuffix(goName, ".go") + "_test.go"
				testFiles = append(testFiles, codegen.GoFile{Name: testName, Source: testSrc})
			}
		}
	}

	if !entryFound && len(parsed) > 0 {
		// If no entry block, generate an empty main with runtime helpers
		goFiles = append(goFiles, codegen.GoFile{
			Name:   "aria_main.go",
			Source: "package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n\t\"strconv\"\n\t\"strings\"\n)\n\nvar _ = fmt.Sprintf\nvar _ = os.Exit\nvar _ = strconv.Itoa\nvar _ = strings.Contains\n\nfunc main() {}\n" + codegen.RuntimeHelpers(),
		})
	}

	return goFiles, testFiles
}

func ariaToGoFilename(ariaPath string) string {
	base := filepath.Base(ariaPath)
	name := strings.TrimSuffix(base, ".aria")
	// Sanitize: replace non-alphanumeric with underscore
	var sb strings.Builder
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			sb.WriteRune(c)
		} else {
			sb.WriteRune('_')
		}
	}
	goName := sb.String()
	// Go treats *_test.go as test files — avoid that
	if strings.HasSuffix(goName, "_test") {
		goName = goName + "_src"
	}
	return goName + ".go"
}

// ---------- Commands ----------

func runCheck(files []string, format string) {
	requireFiles(files, "check")
	allFiles := discoverAriaFiles(files)

	// Parse all files
	var progs []*parser.Program
	hasErrors := false
	for _, file := range allFiles {
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
			p.Diagnostics().Render(os.Stderr)
			hasErrors = true
			continue
		}
		progs = append(progs, prog)
	}
	if hasErrors {
		os.Exit(1)
	}

	// Resolve all together
	r := resolver.New()
	scope := r.ResolveMulti(progs)
	if r.Diagnostics().HasErrors() {
		r.Diagnostics().Render(os.Stderr)
		os.Exit(1)
	}

	// Check all together
	ch := checker.New(scope)
	for _, prog := range progs {
		ch.Check(prog)
	}
	if ch.Diagnostics().HasErrors() {
		ch.Diagnostics().Render(os.Stderr)
		os.Exit(1)
	}

	for _, f := range allFiles {
		fmt.Printf("%s: OK\n", f)
	}
}

func runBuild(files []string, format string) {
	requireFiles(files, "build")
	goFiles, _, err := compileProjectWrapper(files, format)
	if err != nil {
		os.Exit(1)
	}

	base := "aria_output"
	if len(files) > 0 {
		b := filepath.Base(files[0])
		b = strings.TrimSuffix(b, ".aria")
		if b != "." && b != "" {
			base = b
		}
	}

	result, err := codegen.BuildMulti(goFiles, nil, codegen.BuildOptions{
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
	goFiles, _, err := compileProjectWrapper(files, format)
	if err != nil {
		os.Exit(1)
	}

	exitCode, err := codegen.RunMulti(goFiles, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	os.Exit(exitCode)
}

func runTest(files []string, format string) {
	requireFiles(files, "test")
	goFiles, testFiles, err := compileProjectWrapper(files, format)
	if err != nil {
		os.Exit(1)
	}

	if len(testFiles) == 0 {
		fmt.Println("no tests found")
		return
	}

	output, err := codegen.RunTestsMulti(goFiles, testFiles)
	fmt.Print(output)
	if err != nil {
		os.Exit(1)
	}
}

func compileProjectWrapper(inputs []string, format string) ([]codegen.GoFile, []codegen.GoFile, error) {
	goFiles, testFiles := compileProject(inputs, format)
	return goFiles, testFiles, nil
}
