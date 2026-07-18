package gate

import (
	"testing"
)

func TestParseSeverity_ValidInputs(t *testing.T) {
	tests := []struct {
		input    string
		expected Severity
	}{
		{"None", SeverityNone},
		{"Low", SeverityLow},
		{"Medium", SeverityMedium},
		{"High", SeverityHigh},
		{"Critical", SeverityCritical},
		// Case-insensitive variants
		{"none", SeverityNone},
		{"low", SeverityLow},
		{"MEDIUM", SeverityMedium},
		{"HIGH", SeverityHigh},
		{"critical", SeverityCritical},
		{"hIgH", SeverityHigh},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseSeverity(tc.input)
			if err != nil {
				t.Fatalf("ParseSeverity(%q) returned unexpected error: %v", tc.input, err)
			}
			if got != tc.expected {
				t.Errorf("ParseSeverity(%q) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}

func TestParseSeverity_InvalidInputs(t *testing.T) {
	invalid := []string{"", "unknown", "severe", "12345", "  high  "}
	for _, input := range invalid {
		t.Run(input, func(t *testing.T) {
			_, err := ParseSeverity(input)
			if err == nil {
				t.Errorf("ParseSeverity(%q) expected error, got nil", input)
			}
		})
	}
}

func TestSeverity_String(t *testing.T) {
	tests := []struct {
		severity Severity
		expected string
	}{
		{SeverityNone, "None"},
		{SeverityLow, "Low"},
		{SeverityMedium, "Medium"},
		{SeverityHigh, "High"},
		{SeverityCritical, "Critical"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			got := tc.severity.String()
			if got != tc.expected {
				t.Errorf("Severity(%d).String() = %q, want %q", tc.severity, got, tc.expected)
			}
		})
	}
}

func TestSeverity_RoundTrip(t *testing.T) {
	validStrings := []string{"None", "Low", "Medium", "High", "Critical"}
	for _, s := range validStrings {
		t.Run(s, func(t *testing.T) {
			sev, err := ParseSeverity(s)
			if err != nil {
				t.Fatalf("ParseSeverity(%q) returned unexpected error: %v", s, err)
			}
			got := sev.String()
			if got != s {
				t.Errorf("Round-trip failed: ParseSeverity(%q).String() = %q", s, got)
			}
		})
	}
}

func TestSeverity_UnknownValue(t *testing.T) {
	unknown := Severity(99)
	got := unknown.String()
	if got != "Unknown(99)" {
		t.Errorf("Severity(99).String() = %q, want %q", got, "Unknown(99)")
	}
}
