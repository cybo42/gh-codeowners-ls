package diff

import "testing"

func TestSplitRange(t *testing.T) {
	base, head, err := splitRange("main...feature")
	if err != nil {
		t.Fatalf("splitRange: %v", err)
	}
	if base != "main" || head != "feature" {
		t.Errorf("splitRange = %q, %q, want main, feature", base, head)
	}

	if _, _, err := splitRange("no-dots-here"); err == nil {
		t.Error("splitRange(no-dots-here) expected error, got nil")
	}
	if _, _, err := splitRange("main..."); err == nil {
		t.Error("splitRange(main...) with empty head expected error, got nil")
	}
}
