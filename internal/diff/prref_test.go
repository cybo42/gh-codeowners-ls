package diff

import "testing"

func TestParsePRRef(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantNum   int
		wantHost  string
		wantOwner string
		wantName  string
		wantHas   bool
		wantErr   bool
	}{
		{name: "number", in: "482", wantNum: 482},
		{name: "hash number", in: "#482", wantNum: 482},
		{
			name: "full https URL", in: "https://github.com/acme/widgets/pull/482",
			wantNum: 482, wantHost: "github.com", wantOwner: "acme", wantName: "widgets", wantHas: true,
		},
		{
			name: "files suffix", in: "https://github.com/acme/widgets/pull/482/files",
			wantNum: 482, wantHost: "github.com", wantOwner: "acme", wantName: "widgets", wantHas: true,
		},
		{
			name: "query and fragment suffix", in: "https://github.com/acme/widgets/pull/482?w=1#discussion_r123",
			wantNum: 482, wantHost: "github.com", wantOwner: "acme", wantName: "widgets", wantHas: true,
		},
		{
			name: "schemeless", in: "github.com/acme/widgets/pull/482",
			wantNum: 482, wantHost: "github.com", wantOwner: "acme", wantName: "widgets", wantHas: true,
		},
		{
			name: "GHE host", in: "https://github.example.com/acme/widgets/pull/482",
			wantNum: 482, wantHost: "github.example.com", wantOwner: "acme", wantName: "widgets", wantHas: true,
		},
		{
			name: "host normalization", in: "https://WWW.GitHub.com/acme/widgets/pull/482",
			wantNum: 482, wantHost: "github.com", wantOwner: "acme", wantName: "widgets", wantHas: true,
		},
		{name: "no pull segment", in: "https://github.com/acme/widgets", wantErr: true},
		{name: "non-numeric PR", in: "https://github.com/acme/widgets/pull/abc", wantErr: true},
		{name: "empty", in: "", wantErr: true},
		{name: "garbage", in: "not-a-ref", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParsePRRef(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParsePRRef(%q) = %+v, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParsePRRef(%q) unexpected error: %v", tc.in, err)
			}
			if got.Number != tc.wantNum {
				t.Errorf("Number = %d, want %d", got.Number, tc.wantNum)
			}
			if got.HasRepo != tc.wantHas {
				t.Errorf("HasRepo = %v, want %v", got.HasRepo, tc.wantHas)
			}
			if got.HasRepo {
				if got.Repo.Host != tc.wantHost {
					t.Errorf("Repo.Host = %q, want %q", got.Repo.Host, tc.wantHost)
				}
				if got.Repo.Owner != tc.wantOwner {
					t.Errorf("Repo.Owner = %q, want %q", got.Repo.Owner, tc.wantOwner)
				}
				if got.Repo.Name != tc.wantName {
					t.Errorf("Repo.Name = %q, want %q", got.Repo.Name, tc.wantName)
				}
			}
		})
	}
}
