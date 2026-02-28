package review

import "testing"

func TestIsValid(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			"has severity tags",
			"[CRITICAL-1] Missing validation\n[HIGH-1] No rate limiting",
			true,
		},
		{
			"no findings form",
			"No findings.\n\n- Critical: 0\n- High: 0\n- Medium: 0\n- Low: 0",
			true,
		},
		{
			"no findings missing counts",
			"No findings.\n\nLooks good!",
			false,
		},
		{
			"empty",
			"",
			false,
		},
		{
			"random text",
			"This is just some text without findings",
			false,
		},
		{
			"partial no findings",
			"No findings.\nCritical: 0\nHigh: 0",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValid(tt.content); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractFindingIDs(t *testing.T) {
	content := "[CRITICAL-1] issue\n[HIGH-1] thing\n[HIGH-1] dup\n[MEDIUM-2] other"
	ids := ExtractFindingIDs(content)
	if len(ids) != 3 {
		t.Fatalf("expected 3 unique IDs, got %d: %v", len(ids), ids)
	}
	want := []string{"CRITICAL-1", "HIGH-1", "MEDIUM-2"}
	for i, w := range want {
		if ids[i] != w {
			t.Errorf("ids[%d] = %q, want %q", i, ids[i], w)
		}
	}
}

func TestVerifyFindingsCoveragePass(t *testing.T) {
	review := "[HIGH-1] issue\n[MEDIUM-1] thing"
	findings := "### HIGH-1\n**Decision: FIX**\nDone.\n\n### MEDIUM-1\n**Decision: REJECT**\nNot needed."

	fc := VerifyFindingsCoverage(review, findings)
	if fc != nil {
		t.Errorf("expected nil coverage, got %+v", fc)
	}
}

func TestVerifyFindingsCoverageMissing(t *testing.T) {
	review := "[HIGH-1] issue\n[MEDIUM-1] thing\n[LOW-1] minor"
	findings := "### HIGH-1\n**Decision: FIX**\nDone."

	fc := VerifyFindingsCoverage(review, findings)
	if fc == nil {
		t.Fatal("expected non-nil coverage")
	}
	if len(fc.MissingIDs) != 2 {
		t.Errorf("MissingIDs = %v", fc.MissingIDs)
	}
}

func TestVerifyFindingsCoverageMissingDecision(t *testing.T) {
	review := "[HIGH-1] issue"
	findings := "### HIGH-1\nSome text but no decision keyword"

	fc := VerifyFindingsCoverage(review, findings)
	if fc == nil {
		t.Fatal("expected non-nil coverage")
	}
	if len(fc.MissingDecisions) != 1 {
		t.Errorf("MissingDecisions = %v", fc.MissingDecisions)
	}
}

func TestVerifyFindingsCoverageNoFindings(t *testing.T) {
	fc := VerifyFindingsCoverage("No findings.", "")
	if fc != nil {
		t.Errorf("expected nil for no findings")
	}
}
