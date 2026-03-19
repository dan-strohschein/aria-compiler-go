package diagnostic

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Severity represents the severity level of a diagnostic.
type Severity int

const (
	Error   Severity = iota
	Warning
	Info
	Hint
)

func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warning:
		return "warning"
	case Info:
		return "info"
	case Hint:
		return "hint"
	default:
		return "unknown"
	}
}

// LabelStyle distinguishes primary from secondary labels.
type LabelStyle int

const (
	Primary LabelStyle = iota
	Secondary
)

func (ls LabelStyle) String() string {
	if ls == Primary {
		return "primary"
	}
	return "secondary"
}

// Applicability indicates how safe a suggestion is to auto-apply.
type Applicability int

const (
	MachineApplicable Applicability = iota
	MaybeIncorrect
	HasPlaceholders
)

func (a Applicability) String() string {
	switch a {
	case MachineApplicable:
		return "MachineApplicable"
	case MaybeIncorrect:
		return "MaybeIncorrect"
	case HasPlaceholders:
		return "HasPlaceholders"
	default:
		return "Unknown"
	}
}

// Label is an annotated source span within a diagnostic.
type Label struct {
	File    string
	Line    int
	Column  int
	Span    [2]int // start and end byte offsets
	Message string
	Style   LabelStyle
}

// Suggestion is a concrete fix suggestion.
type Suggestion struct {
	Message       string
	Replacement   string
	Span          [2]int
	Applicability Applicability
}

// Diagnostic represents a single compiler diagnostic.
type Diagnostic struct {
	Code        string
	Severity    Severity
	Message     string
	File        string
	Line        int
	Column      int
	Span        [2]int
	SourceLine  string
	Labels      []Label
	Notes       []string
	Suggestions []Suggestion
}

// DiagnosticList collects diagnostics and provides summary information.
type DiagnosticList struct {
	Diagnostics []Diagnostic
}

// Add appends a diagnostic to the list.
func (dl *DiagnosticList) Add(d Diagnostic) {
	dl.Diagnostics = append(dl.Diagnostics, d)
}

// HasErrors returns true if any diagnostic is an error.
func (dl *DiagnosticList) HasErrors() bool {
	for _, d := range dl.Diagnostics {
		if d.Severity == Error {
			return true
		}
	}
	return false
}

// ErrorCount returns the number of error diagnostics.
func (dl *DiagnosticList) ErrorCount() int {
	count := 0
	for _, d := range dl.Diagnostics {
		if d.Severity == Error {
			count++
		}
	}
	return count
}

// WarningCount returns the number of warning diagnostics.
func (dl *DiagnosticList) WarningCount() int {
	count := 0
	for _, d := range dl.Diagnostics {
		if d.Severity == Warning {
			count++
		}
	}
	return count
}

// Render writes all diagnostics to w in human-readable format.
func (dl *DiagnosticList) Render(w io.Writer) {
	for i, d := range dl.Diagnostics {
		if i > 0 {
			fmt.Fprintln(w)
		}
		RenderDiagnostic(w, d)
	}
	if len(dl.Diagnostics) > 0 {
		fmt.Fprintf(w, "\n%d error(s), %d warning(s)\n", dl.ErrorCount(), dl.WarningCount())
	}
}

// RenderJSON writes all diagnostics to w as structured JSON.
func (dl *DiagnosticList) RenderJSON(w io.Writer) error {
	return RenderJSON(w, dl.Diagnostics)
}

// RenderDiagnostic writes a single diagnostic in human-readable format.
func RenderDiagnostic(w io.Writer, d Diagnostic) {
	// Header: error[E0042]: type mismatch
	fmt.Fprintf(w, "%s[%s]: %s\n", d.Severity, d.Code, d.Message)

	// Location: --> src/main.aria:12:18
	if d.File != "" {
		fmt.Fprintf(w, "  --> %s:%d:%d\n", d.File, d.Line, d.Column)
	}

	// Source line with caret
	if d.SourceLine != "" {
		lineNumStr := fmt.Sprintf("%d", d.Line)
		padding := strings.Repeat(" ", len(lineNumStr))

		fmt.Fprintf(w, "   %s |\n", padding)
		fmt.Fprintf(w, "   %s | %s\n", lineNumStr, d.SourceLine)

		// Primary label caret
		if d.Column > 0 {
			caretPadding := strings.Repeat(" ", d.Column-1)
			spanLen := d.Span[1] - d.Span[0]
			if spanLen < 1 {
				spanLen = 1
			}
			carets := strings.Repeat("^", spanLen)

			// Find primary label message
			labelMsg := ""
			for _, l := range d.Labels {
				if l.Style == Primary {
					labelMsg = l.Message
					break
				}
			}
			if labelMsg != "" {
				fmt.Fprintf(w, "   %s | %s%s %s\n", padding, caretPadding, carets, labelMsg)
			} else {
				fmt.Fprintf(w, "   %s | %s%s\n", padding, caretPadding, carets)
			}
		}
	}

	// Notes
	for _, note := range d.Notes {
		fmt.Fprintf(w, "   = note: %s\n", note)
	}

	// Suggestions
	for _, s := range d.Suggestions {
		fmt.Fprintf(w, "   = help: %s\n", s.Message)
		if s.Replacement != "" {
			fmt.Fprintf(w, "           %s\n", s.Replacement)
		}
	}
}

// jsonDiagnostic is the JSON serialization format.
type jsonDiagnostic struct {
	Code        string           `json:"code"`
	Severity    string           `json:"severity"`
	Message     string           `json:"message"`
	File        string           `json:"file"`
	Line        int              `json:"line"`
	Column      int              `json:"column"`
	Span        [2]int           `json:"span"`
	SourceLine  string           `json:"source_line"`
	Labels      []jsonLabel      `json:"labels"`
	Notes       []string         `json:"notes"`
	Suggestions []jsonSuggestion `json:"suggestions"`
}

type jsonLabel struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Span    [2]int `json:"span"`
	Message string `json:"message"`
	Style   string `json:"style"`
}

type jsonSuggestion struct {
	Message       string `json:"message"`
	Replacement   string `json:"replacement"`
	Span          [2]int `json:"span"`
	Applicability string `json:"applicability"`
}

type jsonOutput struct {
	Diagnostics []jsonDiagnostic `json:"diagnostics"`
	Summary     jsonSummary      `json:"summary"`
}

type jsonSummary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
}

// RenderJSON writes diagnostics as structured JSON to w.
func RenderJSON(w io.Writer, diagnostics []Diagnostic) error {
	output := jsonOutput{
		Diagnostics: make([]jsonDiagnostic, len(diagnostics)),
	}

	for i, d := range diagnostics {
		jd := jsonDiagnostic{
			Code:       d.Code,
			Severity:   d.Severity.String(),
			Message:    d.Message,
			File:       d.File,
			Line:       d.Line,
			Column:     d.Column,
			Span:       d.Span,
			SourceLine: d.SourceLine,
			Labels:     make([]jsonLabel, len(d.Labels)),
			Notes:      d.Notes,
			Suggestions: make([]jsonSuggestion, len(d.Suggestions)),
		}

		for j, l := range d.Labels {
			jd.Labels[j] = jsonLabel{
				File:    l.File,
				Line:    l.Line,
				Column:  l.Column,
				Span:    l.Span,
				Message: l.Message,
				Style:   l.Style.String(),
			}
		}

		for j, s := range d.Suggestions {
			jd.Suggestions[j] = jsonSuggestion{
				Message:       s.Message,
				Replacement:   s.Replacement,
				Span:          s.Span,
				Applicability: s.Applicability.String(),
			}
		}

		if d.Severity == Error {
			output.Summary.Errors++
		} else if d.Severity == Warning {
			output.Summary.Warnings++
		}

		output.Diagnostics[i] = jd
	}

	if output.Diagnostics == nil {
		output.Diagnostics = []jsonDiagnostic{}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}
