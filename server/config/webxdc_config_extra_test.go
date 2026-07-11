package config

import "testing"

func TestWebxdcConfig_ApplyDefaults(t *testing.T) {
	w := WebxdcConfig{}
	w.applyDefaults()
	if w.UploadPolicy != "owner_mod" {
		t.Fatal(w.UploadPolicy)
	}
	if w.StorageDir != "./data/webxdc" {
		t.Fatal(w.StorageDir)
	}
	if w.TicketTTLMinutes != 10 {
		t.Fatal(w.TicketTTLMinutes)
	}
	if w.SendUpdateMaxSize != 128000 {
		t.Fatal(w.SendUpdateMaxSize)
	}
	if w.SendUpdateIntervalMs != 10000 {
		t.Fatal(w.SendUpdateIntervalMs)
	}
	if w.Gallery.Source != "local" {
		t.Fatal(w.Gallery.Source)
	}
}

func TestWebxdcConfig_Validate_AnyMember(t *testing.T) {
	w := WebxdcConfig{
		Enabled: true, BaseDomain: "wx.example.com", UploadPolicy: "any_member",
	}
	w.applyDefaults()
	if err := w.Validate("meet.example.com"); err != nil {
		t.Fatal(err)
	}
}
