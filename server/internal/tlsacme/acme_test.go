package tlsacme

import "testing"

func TestWildcardNames(t *testing.T) {
	got := WildcardNames("bedrud.xyz", "wx.bedrud.xyz")
	want := []string{"bedrud.xyz", "*.bedrud.xyz", "wx.bedrud.xyz", "*.wx.bedrud.xyz"}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] got %q want %q", i, got[i], want[i])
		}
	}

	// same base as domain → no duplicate
	got = WildcardNames("bedrud.xyz", "bedrud.xyz")
	if len(got) != 2 || got[0] != "bedrud.xyz" || got[1] != "*.bedrud.xyz" {
		t.Fatalf("same base: %v", got)
	}

	// strip accidental wildcard prefix
	got = WildcardNames("*.example.com", "")
	if len(got) != 2 || got[0] != "example.com" {
		t.Fatalf("strip star: %v", got)
	}
}
