package webxdc

import (
	"strings"
	"testing"
	"time"
)

func TestMintTicket_EmptySecret(t *testing.T) {
	_, _, err := MintTicket("", "j", "i", "r", "u", time.Minute)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestVerifyTicket_Expired(t *testing.T) {
	tok, _, err := MintTicket("sec", "j1", "inst", "room", "user", -time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyTicket("sec", tok, "inst"); err == nil {
		t.Fatal("expected expired")
	}
}

func TestVerifyTicket_Malformed(t *testing.T) {
	if _, err := VerifyTicket("sec", "no-dot", "i"); err == nil {
		t.Fatal("malformed")
	}
	if _, err := VerifyTicket("sec", "", "i"); err == nil {
		t.Fatal("empty")
	}
}

func TestVerifyTicket_TamperedPayload(t *testing.T) {
	tok, _, err := MintTicket("sec", "j1", "inst", "room", "user", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(tok, ".")
	// flip a character in signature
	sig := []byte(parts[1])
	sig[0] ^= 0xff
	bad := parts[0] + "." + string(sig)
	if _, err := VerifyTicket("sec", bad, "inst"); err == nil {
		t.Fatal("expected bad sig")
	}
}

func TestInstanceOrigin(t *testing.T) {
	if got := InstanceOrigin("abc", "wx.example.com", true, ""); got != "https://webxdc-abc.wx.example.com" {
		t.Fatal(got)
	}
	if got := InstanceOrigin("abc", "wx.example.com", false, ""); got != "http://webxdc-abc.wx.example.com" {
		t.Fatal(got)
	}
	if got := InstanceOrigin("abc", "localhost", false, "7071"); got != "http://webxdc-abc.localhost:7071" {
		t.Fatal(got)
	}
}
