package main

import (
	"strings"
	"testing"
)

func TestLoadConfigMissing(t *testing.T) {
	t.Setenv("LINODE_TOKEN", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("CLOUDFLARE_ZONE", "")
	t.Setenv("BEDRUD_HOST_CONFIG_DIR", t.TempDir())
	_, err := loadConfig()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "bedrud-host init") {
		t.Fatalf("want init hint, got %v", err)
	}
}

func TestRequireInit(t *testing.T) {
	err := requireInit(Config{})
	if err == nil || !strings.Contains(err.Error(), "bedrud-host init") {
		t.Fatalf("%v", err)
	}
	err = requireInit(Config{
		LinodeToken: "t", CloudflareTok: "c", Zone: "example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoadConfigOK(t *testing.T) {
	t.Setenv("LINODE_TOKEN", "tok")
	t.Setenv("CLOUDFLARE_API_TOKEN", "cftok")
	t.Setenv("CLOUDFLARE_ZONE", "example.com")
	t.Setenv("BEDRUD_HOST_CONFIG_DIR", t.TempDir())
	t.Setenv("LINODE_REGION", "us-east")
	c, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.LinodeToken != "tok" || c.Zone != "example.com" || c.LinodeRegion != "us-east" {
		t.Fatalf("%+v", c)
	}
	if c.LinodeBase != "https://api.linode.com/v4" {
		t.Fatalf("base %s", c.LinodeBase)
	}
}

func TestLoadConfigFromStore(t *testing.T) {
	dir := t.TempDir()
	st, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = st.SetSetting(setLinodeToken, "from-db")
	_ = st.SetSetting(setCFToken, "cf-db")
	_ = st.SetSetting(setCFDomain, "example.com")
	_ = st.Close()
	t.Setenv("LINODE_TOKEN", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("CLOUDFLARE_ZONE", "")
	t.Setenv("BEDRUD_HOST_CONFIG_DIR", dir)
	c, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.LinodeToken != "from-db" || c.CloudflareTok != "cf-db" || c.Zone != "example.com" {
		t.Fatalf("%+v", c)
	}
}

func TestEnvOrAndTrimBase(t *testing.T) {
	t.Setenv("LINODE_TOKEN", "tok")
	t.Setenv("CLOUDFLARE_API_TOKEN", "cftok")
	t.Setenv("CLOUDFLARE_ZONE", "example.com")
	t.Setenv("BEDRUD_HOST_CONFIG_DIR", t.TempDir())
	t.Setenv("LINODE_API_URL", "https://api.linode.com/v4/")
	t.Setenv("CLOUDFLARE_API_URL", "https://api.cloudflare.com/client/v4/")
	c, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.LinodeBase[len(c.LinodeBase)-1] == '/' {
		t.Fatalf("linode base still has slash %s", c.LinodeBase)
	}
	if c.CloudflareBase[len(c.CloudflareBase)-1] == '/' {
		t.Fatalf("cf base still has slash %s", c.CloudflareBase)
	}
}

func TestTakenHosts(t *testing.T) {
	recs := []cfRecord{
		{Name: "foo.example.com"},
		{Name: "bar.example.com"},
	}
	m := takenHosts(recs, "example.com")
	if !m["foo"] || !m["bar"] {
		t.Fatalf("%v", m)
	}
}

func TestRecordsForHost(t *testing.T) {
	recs := []cfRecord{
		{Type: "A", Name: "meet.example.com", Content: "1.2.3.4", ID: "1"},
		{Type: "A", Name: "other.example.com", Content: "9.9.9.9", ID: "2"},
		{Type: "A", Name: "*.meet.example.com", Content: "1.2.3.4", ID: "3"},
	}
	got := recordsForHost(recs, "meet", "example.com")
	if len(got) != 2 {
		t.Fatalf("got %d", len(got))
	}
}
