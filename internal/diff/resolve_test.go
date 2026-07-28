package diff

import "testing"

func TestParsePRNumber(t *testing.T) {
	tests := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{in: "42", want: 42},
		{in: "#42", want: 42},
		{in: "https://github.com/acme/widgets/pull/482", want: 482},
		{in: "not-a-number", wantErr: true},
	}

	for _, tc := range tests {
		got, err := parsePRNumber(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parsePRNumber(%q) = %d, want error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parsePRNumber(%q) unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parsePRNumber(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

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
