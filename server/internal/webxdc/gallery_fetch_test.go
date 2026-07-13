package webxdc

import (
	"testing"
)

func TestParseGalleryCatalog_AppsArray(t *testing.T) {
	raw := []byte(`{
		"apps": [
			{"id":"a1","name":"Chess","description":"Play","xdcUrl":"https://example.com/chess.xdc","icon":"https://example.com/i.png"},
			{"name":"NoUrl"}
		]
	}`)
	list, err := parseGalleryCatalog(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("len %d", len(list))
	}
	if list[0].Name != "Chess" || list[0].XdcURL != "https://example.com/chess.xdc" {
		t.Fatalf("%+v", list[0])
	}
	if list[1].Name != "NoUrl" {
		t.Fatal(list[1].Name)
	}
}

func TestParseGalleryCatalog_BareArray(t *testing.T) {
	raw := []byte(`[{"title":"App","download":"https://cdn.example.com/a.xdc"}]`)
	list, err := parseGalleryCatalog(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "App" || list[0].XdcURL == "" {
		t.Fatalf("%+v", list)
	}
}

func TestParseGalleryCatalog_XdcgetLock(t *testing.T) {
	raw := []byte(`[{
		"app_id": "link2xt-count",
		"name": "Chatters cannot count",
		"url": "https://codeberg.org/x.xdc",
		"description": "Learn to count",
		"icon_relname": "link2xt-count-icon.png",
		"source_code_url": "https://codeberg.org/x/"
	}]`)
	list, err := parseGalleryCatalog(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatal(len(list))
	}
	if list[0].ID != "link2xt-count" || list[0].XdcURL == "" || list[0].IconURL != "link2xt-count-icon.png" {
		t.Fatalf("%+v", list[0])
	}
}

func TestResolveCatalogURL_StorePage(t *testing.T) {
	if got := resolveCatalogURL("https://webxdc.com/apps"); got != WellKnownXdcgetLock {
		t.Fatal(got)
	}
	if got := resolveCatalogURL("https://webxdc.org/apps/"); got != WellKnownXdcgetLock {
		t.Fatal(got)
	}
	if got := resolveCatalogURL("https://mirror.example.com/catalog.json"); got != "https://mirror.example.com/catalog.json" {
		t.Fatal(got)
	}
}

func TestRejectPrivateHost(t *testing.T) {
	if err := rejectPrivateHost("localhost"); err == nil {
		t.Fatal("expected block")
	}
	if err := rejectPrivateHost("127.0.0.1"); err == nil {
		t.Fatal("expected block")
	}
	if err := rejectPrivateHost("10.0.0.1"); err == nil {
		t.Fatal("expected block")
	}
}
