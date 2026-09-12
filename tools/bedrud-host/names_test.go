package main

import "testing"

func TestHostFromArg(t *testing.T) {
	z := "example.com"
	if g := hostFromArg("meet", z); g != "meet" {
		t.Fatalf("got %q", g)
	}
	if g := hostFromArg("meet.example.com", z); g != "meet" {
		t.Fatalf("got %q", g)
	}
	if g := hostFromArg("MEET.Example.COM.", z); g != "meet" {
		t.Fatalf("got %q", g)
	}
}

func TestFqdnAndLabel(t *testing.T) {
	if g := fqdn("meet", "example.com"); g != "meet.example.com" {
		t.Fatalf("fqdn %q", g)
	}
	lab := linodeLabel("meet", "example.com")
	if lab != "meet-example-com" {
		t.Fatalf("label %q", lab)
	}
	long := linodeLabel("thisisareallylongsubdomainname", "example.com")
	if len(long) > 32 {
		t.Fatalf("label too long %d", len(long))
	}
}

func TestAdminEmail(t *testing.T) {
	if g := adminEmail("meet", "example.com"); g != "admin@meet.example.com" {
		t.Fatalf("got %q", g)
	}
}

func TestSplitAndResolveName(t *testing.T) {
	h, z, n, err := splitFQDN("Meet.Example.COM.")
	if err != nil || h != "meet" || z != "example.com" || n != "meet.example.com" {
		t.Fatalf("%s %s %s %v", h, z, n, err)
	}
	if _, _, _, err := splitFQDN("meet"); err == nil {
		t.Fatal("expected error")
	}
	h, z, n, err = resolveName("meet", "example.com")
	if err != nil || n != "meet.example.com" {
		t.Fatalf("%s %s %s %v", h, z, n, err)
	}
	h, z, n, err = resolveName("meet.example.com", "")
	if err != nil || h != "meet" || z != "example.com" {
		t.Fatalf("%s %s %s %v", h, z, n, err)
	}
}

func TestRandomLabel(t *testing.T) {
	s, err := randomLabel(5)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 5 {
		t.Fatalf("len %d", len(s))
	}
	for _, r := range s {
		if r < 'a' || r > 'z' {
			t.Fatalf("non letter %q", s)
		}
	}
	s2, err := randomLabel(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(s2) != 3 {
		t.Fatalf("min len want 3 got %d", len(s2))
	}
}

func TestRandomPassword(t *testing.T) {
	p, err := randomPassword(16)
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 16 {
		t.Fatalf("len %d", len(p))
	}
}
