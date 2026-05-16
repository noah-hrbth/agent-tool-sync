package canonical

import "testing"

// TestReservedRuleNames pins every documented reserved slug: it must report as
// reserved with a non-empty rationale. Adding a map entry auto-extends coverage.
func TestReservedRuleNames(t *testing.T) {
	if len(reservedRuleNames) == 0 {
		t.Fatal("reservedRuleNames unexpectedly empty")
	}
	for slug, reason := range reservedRuleNames {
		if !IsReservedRuleName(slug) {
			t.Errorf("IsReservedRuleName(%q) = false, want true", slug)
		}
		if reason == "" {
			t.Errorf("reservedRuleNames[%q] has empty rationale", slug)
		}
		if got := ReservedRuleReason(slug); got != reason {
			t.Errorf("ReservedRuleReason(%q) = %q, want %q", slug, got, reason)
		}
	}
}

// TestNonReservedRuleName confirms an ordinary slug is not reserved and yields
// no rationale.
func TestNonReservedRuleName(t *testing.T) {
	const slug = "coding-style"
	if IsReservedRuleName(slug) {
		t.Errorf("IsReservedRuleName(%q) = true, want false", slug)
	}
	if got := ReservedRuleReason(slug); got != "" {
		t.Errorf("ReservedRuleReason(%q) = %q, want \"\"", slug, got)
	}
}
