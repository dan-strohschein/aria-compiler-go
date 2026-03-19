package diagnostic

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderDiagnostic_TypeMismatch(t *testing.T) {
	d := Diagnostic{
		Code:       E0100,
		Severity:   Error,
		Message:    "type mismatch",
		File:       "src/process.aria",
		Line:       12,
		Column:     18,
		Span:       [2]int{245, 251},
		SourceLine: `    result := add(userId, orderId)`,
		Labels: []Label{
			{
				File:    "src/process.aria",
				Line:    12,
				Column:  18,
				Span:    [2]int{245, 251},
				Message: "expected UserId, found OrderId",
				Style:   Primary,
			},
		},
		Notes: []string{"UserId and OrderId are distinct newtypes wrapping i64"},
		Suggestions: []Suggestion{
			{
				Message:       "use .value to access the raw i64 values",
				Replacement:   "add(userId.value, orderId.value)",
				Span:          [2]int{234, 262},
				Applicability: MaybeIncorrect,
			},
		},
	}

	var buf bytes.Buffer
	RenderDiagnostic(&buf, d)
	output := buf.String()

	// Check header
	if !strings.Contains(output, "error[E0100]: type mismatch") {
		t.Errorf("expected error header, got:\n%s", output)
	}

	// Check location
	if !strings.Contains(output, "--> src/process.aria:12:18") {
		t.Errorf("expected location, got:\n%s", output)
	}

	// Check source line
	if !strings.Contains(output, "result := add(userId, orderId)") {
		t.Errorf("expected source line, got:\n%s", output)
	}

	// Check caret with label
	if !strings.Contains(output, "^^^^^^ expected UserId, found OrderId") {
		t.Errorf("expected caret with label, got:\n%s", output)
	}

	// Check note
	if !strings.Contains(output, "= note: UserId and OrderId are distinct newtypes wrapping i64") {
		t.Errorf("expected note, got:\n%s", output)
	}

	// Check suggestion
	if !strings.Contains(output, "= help: use .value to access the raw i64 values") {
		t.Errorf("expected help, got:\n%s", output)
	}
}

func TestRenderDiagnostic_SyntaxError(t *testing.T) {
	d := Diagnostic{
		Code:       E0001,
		Severity:   Error,
		Message:    "invalid character",
		File:       "test.aria",
		Line:       1,
		Column:     5,
		Span:       [2]int{4, 5},
		SourceLine: "mod $main",
		Labels: []Label{
			{
				File:    "test.aria",
				Line:    1,
				Column:  5,
				Span:    [2]int{4, 5},
				Message: "unexpected character '$'",
				Style:   Primary,
			},
		},
	}

	var buf bytes.Buffer
	RenderDiagnostic(&buf, d)
	output := buf.String()

	if !strings.Contains(output, "error[E0001]: invalid character") {
		t.Errorf("expected error header, got:\n%s", output)
	}
	if !strings.Contains(output, "^ unexpected character '$'") {
		t.Errorf("expected caret, got:\n%s", output)
	}
}

func TestRenderJSON(t *testing.T) {
	diagnostics := []Diagnostic{
		{
			Code:       E0100,
			Severity:   Error,
			Message:    "type mismatch",
			File:       "src/main.aria",
			Line:       5,
			Column:     10,
			Span:       [2]int{40, 46},
			SourceLine: `    x := "hello"`,
			Labels: []Label{
				{
					File:    "src/main.aria",
					Line:    5,
					Column:  10,
					Span:    [2]int{40, 46},
					Message: `expected i64, found str`,
					Style:   Primary,
				},
			},
			Notes: []string{"x was declared as i64"},
			Suggestions: []Suggestion{
				{
					Message:       `try parsing: x := "hello".parseInt[i64]()`,
					Replacement:   `"hello".parseInt[i64]()`,
					Span:          [2]int{40, 47},
					Applicability: MaybeIncorrect,
				},
			},
		},
	}

	var buf bytes.Buffer
	err := RenderJSON(&buf, diagnostics)
	if err != nil {
		t.Fatalf("RenderJSON failed: %v", err)
	}

	// Parse the JSON to verify structure
	var output jsonOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, buf.String())
	}

	if len(output.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(output.Diagnostics))
	}

	d := output.Diagnostics[0]
	if d.Code != "E0100" {
		t.Errorf("expected code E0100, got %s", d.Code)
	}
	if d.Severity != "error" {
		t.Errorf("expected severity error, got %s", d.Severity)
	}
	if d.Line != 5 {
		t.Errorf("expected line 5, got %d", d.Line)
	}
	if len(d.Labels) != 1 {
		t.Errorf("expected 1 label, got %d", len(d.Labels))
	}
	if d.Labels[0].Style != "primary" {
		t.Errorf("expected primary style, got %s", d.Labels[0].Style)
	}
	if output.Summary.Errors != 1 {
		t.Errorf("expected 1 error in summary, got %d", output.Summary.Errors)
	}
	if output.Summary.Warnings != 0 {
		t.Errorf("expected 0 warnings in summary, got %d", output.Summary.Warnings)
	}
}

func TestDiagnosticList(t *testing.T) {
	dl := &DiagnosticList{}

	dl.Add(Diagnostic{Code: E0001, Severity: Error, Message: "error 1"})
	dl.Add(Diagnostic{Code: W0001, Severity: Warning, Message: "warning 1"})
	dl.Add(Diagnostic{Code: E0002, Severity: Error, Message: "error 2"})

	if !dl.HasErrors() {
		t.Error("expected HasErrors to be true")
	}
	if dl.ErrorCount() != 2 {
		t.Errorf("expected 2 errors, got %d", dl.ErrorCount())
	}
	if dl.WarningCount() != 1 {
		t.Errorf("expected 1 warning, got %d", dl.WarningCount())
	}
}

func TestSeverityString(t *testing.T) {
	tests := []struct {
		s    Severity
		want string
	}{
		{Error, "error"},
		{Warning, "warning"},
		{Info, "info"},
		{Hint, "hint"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("Severity(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}
