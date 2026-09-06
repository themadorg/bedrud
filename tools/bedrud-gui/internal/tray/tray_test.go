package tray

import "testing"

func TestMenuLayout(t *testing.T) {
	m := &menuServer{revision: 1}
	rev, layout, err := m.GetLayout(0, -1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rev != 1 {
		t.Fatalf("rev %d", rev)
	}
	if len(layout.Children) != 4 {
		t.Fatalf("children %d", len(layout.Children))
	}
}
