package tools

import (
	"strings"
	"testing"
)

func TestBuildTOMLEmitsStringSliceArray(t *testing.T) {
	got := buildTOML([]fmField{
		{key: "enabled_tools", value: []string{"read_file", "grep"}},
	})
	want := `enabled_tools = ["read_file", "grep"]` + "\n"
	if got != want {
		t.Errorf("buildTOML []string: got %q, want %q", got, want)
	}
}

func TestBuildTOMLOmitsEmptyStringSlice(t *testing.T) {
	got := buildTOML([]fmField{
		{key: "before", value: "x"},
		{key: "enabled_tools", value: []string{}},
		{key: "also_empty", value: []string(nil)},
		{key: "after", value: "y"},
	})
	if strings.Contains(got, "enabled_tools") {
		t.Errorf("expected empty slice to be omitted, got:\n%s", got)
	}
	if strings.Contains(got, "also_empty") {
		t.Errorf("expected nil slice to be omitted, got:\n%s", got)
	}
	if !strings.Contains(got, `before = "x"`) || !strings.Contains(got, `after = "y"`) {
		t.Errorf("expected non-slice fields to render, got:\n%s", got)
	}
}

func TestBuildTOMLEscapesQuotesAndBackslashesInArray(t *testing.T) {
	got := buildTOML([]fmField{
		{key: "items", value: []string{`he said "hi"`, `path\to\thing`}},
	})
	want := `items = ["he said \"hi\"", "path\\to\\thing"]` + "\n"
	if got != want {
		t.Errorf("buildTOML []string escaping: got %q, want %q", got, want)
	}
}
