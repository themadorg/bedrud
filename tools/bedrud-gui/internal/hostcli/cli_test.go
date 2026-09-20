package hostcli

import (
	"testing"
	"time"
)

func TestParseList(t *testing.T) {
	out := "1  meet.example.com  ipv4=203.0.113.10  linode=99  admin=a@b.c  created=2026-01-01T00:00:00Z  took=1m49s  $0.0075/hr\n"
	hs := ParseList(out)
	if len(hs) != 1 {
		t.Fatalf("len %d", len(hs))
	}
	h := hs[0]
	if h.ID != 1 || h.FQDN != "meet.example.com" || h.IPv4 != "203.0.113.10" || h.LinodeID != 99 {
		t.Fatalf("%+v", h)
	}
	if h.Took != "1m49s" || h.Hourly == "" {
		t.Fatalf("took/hourly %+v", h)
	}
}

func TestParseStatusCreateAvg(t *testing.T) {
	st := ParseStatus("create-avg             1m49s (2 runs)\nlinode-token           ****abcd\ncloudflare-domain      example.com\ncloudflare-token       ****zzzz\n")
	if st.CreateAvg != 1*time.Minute+49*time.Second {
		t.Fatalf("avg %s", st.CreateAvg)
	}
}

func TestParseStatus(t *testing.T) {
	out := "linode-token           ****abcd\ncloudflare-api-key     ****efgh\ncloudflare-domain      example.com\ncloudflare-token       not set\n"
	st := ParseStatus(out)
	if !st.Initialized {
		t.Fatalf("expected initialized: %+v", st.Lines)
	}
	st = ParseStatus("linode-token           not set\ncloudflare-domain      not set\n")
	if st.Initialized {
		t.Fatal("expected not initialized")
	}
}

func TestApplyCreateLine(t *testing.T) {
	var p CreateProgress
	ApplyCreateLine(&p, "plan: linode g6-nanode-1 in eu-central, host meet.example.com")
	if p.FQDN != "meet.example.com" {
		t.Fatalf("%+v", p)
	}
	ApplyCreateLine(&p, "average create time: 1m49s (2 runs)")
	if p.Avg != 1*time.Minute+49*time.Second {
		t.Fatalf("avg %s", p.Avg)
	}
	ApplyCreateLine(&p, "linode id=99 status=provisioning")
	if p.LinodeID != 99 {
		t.Fatalf("%+v", p)
	}
	ApplyCreateLine(&p, "ipv4 203.0.113.10")
	if p.IPv4 != "203.0.113.10" {
		t.Fatalf("%+v", p)
	}
	ApplyCreateLine(&p, "installing bedrud…")
	if p.Stage != "Installing" {
		t.Fatalf("%+v", p)
	}
	ApplyCreateLine(&p, "========== bedrud ready ==========")
	if !p.Ready {
		t.Fatal("ready")
	}
}

func TestApplyDeleteLine(t *testing.T) {
	var p CreateProgress
	ApplyDeleteLine(&p, "delete dns abc")
	if p.Stage != "Deleting DNS" {
		t.Fatalf("%+v", p)
	}
	ApplyDeleteLine(&p, "linode wait 3/48 status=deleting")
	if p.Stage != "Waiting for Linode" {
		t.Fatalf("%+v", p)
	}
	ApplyDeleteLine(&p, "deleted meet.example.com")
	if !p.Ready {
		t.Fatal("ready")
	}
}

func TestParseFields(t *testing.T) {
	out := "id:\t1\nfqdn:            meet.example.com\nipv4: 203.0.113.10\n"
	fs := ParseFields(out)
	if len(fs) != 3 {
		t.Fatalf("%+v", fs)
	}
	if fs[1].Key != "fqdn" || fs[1].Value != "meet.example.com" {
		t.Fatalf("%+v", fs[1])
	}
}
