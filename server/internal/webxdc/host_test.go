package webxdc

import (
	"testing"
	"time"
)

func TestParseInstanceHost(t *testing.T) {
	label, ok := ParseInstanceHost("webxdc-abc123.wx.example.com", "wx.example.com")
	if !ok || label != "abc123" {
		t.Fatalf("got %q %v", label, ok)
	}
	if _, ok := ParseInstanceHost("webxdc-abc.wx.example.com:443", "wx.example.com"); !ok {
		t.Fatal("port should strip")
	}
	if _, ok := ParseInstanceHost("app.example.com", "wx.example.com"); ok {
		t.Fatal("spa host must not match")
	}
	if _, ok := ParseInstanceHost("webxdc-ab.c.wx.example.com", "wx.example.com"); ok {
		// multi-dot before base is wrong parse - actually host is webxdc-ab.c.wx.example.com
		// suffix .wx.example.com -> prefix webxdc-ab.c which contains dot -> reject
		t.Fatal("nested label must fail")
	}
}

func TestTicketRoundTrip(t *testing.T) {
	tok, exp, err := MintTicket("secret", "j1", "inst1", "room1", "user1", 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if exp.Before(time.Now()) {
		t.Fatal("exp")
	}
	claims, err := VerifyTicket("secret", tok, "inst1")
	if err != nil {
		t.Fatal(err)
	}
	if claims.Room != "room1" || claims.Sub != "user1" {
		t.Fatalf("%+v", claims)
	}
	if _, err := VerifyTicket("secret", tok, "other"); err == nil {
		t.Fatal("expected instance mismatch")
	}
	if _, err := VerifyTicket("wrong", tok, "inst1"); err == nil {
		t.Fatal("expected bad sig")
	}
}

func TestSelfAddrStableAndUnlinkable(t *testing.T) {
	a := SelfAddr("s", "r", "i1", "u")
	b := SelfAddr("s", "r", "i1", "u")
	c := SelfAddr("s", "r", "i2", "u")
	if a != b || a == c || len(a) != 32 {
		t.Fatalf("a=%s b=%s c=%s", a, b, c)
	}
}

func TestParsePathModeInstance(t *testing.T) {
	label, asset, ok := ParsePathModeInstance("/__webxdc/abc123/")
	if !ok || label != "abc123" || asset != "/" {
		t.Fatalf("got %q %q %v", label, asset, ok)
	}
	label, asset, ok = ParsePathModeInstance("/__webxdc/deadbeef/index.html")
	if !ok || label != "deadbeef" || asset != "/index.html" {
		t.Fatalf("got %q %q %v", label, asset, ok)
	}
	if _, _, ok := ParsePathModeInstance("/api/rooms/x"); ok {
		t.Fatal("expected reject")
	}
}

func TestInstanceOriginPath(t *testing.T) {
	got := InstanceOriginPath("abc", "http://localhost:7071/")
	if got != "http://localhost:7071/__webxdc/abc" {
		t.Fatal(got)
	}
}
