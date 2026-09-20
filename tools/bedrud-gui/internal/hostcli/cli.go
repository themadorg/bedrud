package hostcli

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"bedrud-gui/internal/hostbin"
)

type Host struct {
	ID       int
	FQDN     string
	IPv4     string
	LinodeID int
	Admin    string
	Created  string
	Took     string
	Hourly   string
}

type Status struct {
	Initialized bool
	Raw         string
	Lines       map[string]string
	CreateAvg   time.Duration
}

func FindBin() (string, error) {
	if p := strings.TrimSpace(os.Getenv("BEDRUD_HOST_BIN")); p != "" {
		return p, nil
	}
	if p, err := hostbin.Ensure(); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("bedrud-host"); err == nil {
		return p, nil
	}
	self, err := os.Executable()
	if err == nil {
		cand := filepath.Join(filepath.Dir(self), "..", "bedrud-host", "bedrud-host")
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand, nil
		}
	}
	wd, _ := os.Getwd()
	cand := filepath.Join(wd, "..", "bedrud-host", "bedrud-host")
	if st, err := os.Stat(cand); err == nil && !st.IsDir() {
		return cand, nil
	}
	return "", fmt.Errorf("bedrud-host binary not found (set BEDRUD_HOST_BIN)")
}

func run(bin string, timeout time.Duration, args ...string) (string, error) {
	cmd := exec.Command(bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	timer := time.AfterFunc(timeout, func() { _ = cmd.Process.Kill() })
	defer timer.Stop()
	err := cmd.Run()
	out := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
	if err != nil {
		if out == "" {
			return "", err
		}
		return out, fmt.Errorf("%w: %s", err, out)
	}
	return stdout.String(), nil
}

func StatusCmd(bin string) (Status, error) {
	return StatusCmdLocal(bin, false)
}

func StatusCmdLocal(bin string, local bool) (Status, error) {
	args := []string{"status"}
	if local {
		args = append(args, "-local")
	}
	out, err := run(bin, 8*time.Second, args...)
	st := ParseStatus(out)
	if err != nil && !st.Initialized {
		st.Raw = out
		return st, err
	}
	st.Raw = out
	return st, nil
}

func ParseStatus(out string) Status {
	s := Status{Lines: map[string]string{}}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := fields[0]
		val := strings.TrimSpace(strings.TrimPrefix(line, key))
		s.Lines[key] = val
	}
	lt := s.Lines["linode-token"]
	dom := s.Lines["cloudflare-domain"]
	cfTok := s.Lines["cloudflare-token"]
	cfKey := s.Lines["cloudflare-api-key"]
	linodeOK := lt != "" && !strings.Contains(lt, "not set") && !strings.Contains(lt, "missing")
	domOK := dom != "" && !strings.Contains(dom, "not set") && !strings.Contains(dom, "missing")
	cfOK := (cfTok != "" && !strings.Contains(cfTok, "not set")) ||
		(cfKey != "" && !strings.Contains(cfKey, "not set"))
	s.Initialized = linodeOK && domOK && cfOK
	if v := s.Lines["create-avg"]; v != "" && !strings.Contains(v, "no samples") {
		if f := strings.Fields(v); len(f) > 0 {
			if d, err := time.ParseDuration(f[0]); err == nil {
				s.CreateAvg = d
			}
		}
	}
	return s
}

var listRE = regexp.MustCompile(`^(\d+)\s+(\S+)\s+ipv4=(\S*)\s+linode=(\d+)\s+admin=(\S*)\s+created=(\S*)(.*)$`)

func ParseList(out string) []Host {
	var hosts []Host
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		m := listRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		id, _ := strconv.Atoi(m[1])
		lid, _ := strconv.Atoi(m[4])
		h := Host{
			ID: id, FQDN: m[2], IPv4: m[3], LinodeID: lid,
			Admin: m[5], Created: m[6],
		}
		rest := m[7]
		if i := strings.Index(rest, "took="); i >= 0 {
			h.Took = strings.Fields(rest[i+5:])[0]
		}
		if i := strings.Index(rest, "$"); i >= 0 {
			h.Hourly = strings.TrimSpace(rest[i:])
		}
		hosts = append(hosts, h)
	}
	return hosts
}

func List(bin string) ([]Host, string, error) {
	out, err := run(bin, 30*time.Second, "list")
	return ParseList(out), out, err
}

func Reset(bin string) (string, error) {
	return run(bin, 30*time.Second, "reset", "-yes")
}

func Init(bin string, linodeToken, cfToken, cfEmail, cfKey, domain, zoneID string) (string, error) {
	args := []string{"init"}
	if linodeToken != "" {
		args = append(args, "--linode-token", linodeToken)
	}
	if cfToken != "" {
		args = append(args, "--cloudflare-token", cfToken)
	}
	if cfEmail != "" {
		args = append(args, "--cloudflare-email", cfEmail)
	}
	if cfKey != "" {
		args = append(args, "--cloudflare-api-key", cfKey)
	}
	if domain != "" {
		args = append(args, "--cloudflare-domain", domain)
	}
	if zoneID != "" {
		args = append(args, "--cloudflare-zone", zoneID)
	}
	return run(bin, 60*time.Second, args...)
}

func Create(bin string) (string, error) {
	return run(bin, 15*time.Minute, "create")
}

type CreateProgress struct {
	Stage    string
	Fraction float64
	FQDN     string
	IPv4     string
	LinodeID int
	Hourly   string
	Avg      time.Duration
	Ready    bool
}

var (
	planHostRE = regexp.MustCompile(`host\s+(\S+)\s*$`)
	linodeIDRE = regexp.MustCompile(`linode id=(\d+)`)
	ipv4RE     = regexp.MustCompile(`^ipv4\s+(\S+)`)
	avgRE      = regexp.MustCompile(`average create time:\s+(\S+)`)
	hourlyRE   = regexp.MustCompile(`hourly:\s+(\S+)`)
)

func ApplyCreateLine(p *CreateProgress, line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	if m := planHostRE.FindStringSubmatch(line); m != nil {
		p.FQDN = m[1]
		p.Stage = "Provisioning"
		p.bump(0.12)
		return
	}
	if m := avgRE.FindStringSubmatch(line); m != nil {
		if d, err := time.ParseDuration(m[1]); err == nil {
			p.Avg = d
		}
		return
	}
	if m := linodeIDRE.FindStringSubmatch(line); m != nil {
		p.LinodeID, _ = strconv.Atoi(m[1])
		p.Stage = "Linode created"
		p.bump(0.32)
		return
	}
	if m := ipv4RE.FindStringSubmatch(line); m != nil {
		p.IPv4 = m[1]
		p.Stage = "Address assigned"
		p.bump(0.45)
		return
	}
	if strings.HasPrefix(line, "dns ") {
		p.Stage = "DNS record"
		p.bump(0.55)
		return
	}
	if strings.HasPrefix(strings.ToLower(line), "ssh wait") {
		p.Stage = "Waiting for SSH"
		return
	}
	if strings.Contains(line, "installing bedrud") {
		p.Stage = "Installing"
		p.bump(0.68)
		return
	}
	if strings.Contains(line, "bedrud ready") || strings.HasPrefix(line, "url:") {
		p.Stage = "Ready"
		p.bump(0.92)
		p.Ready = true
		return
	}
	if m := hourlyRE.FindStringSubmatch(line); m != nil && m[1] != "unknown" {
		p.Hourly = strings.TrimSpace(strings.TrimPrefix(line, "hourly:"))
		return
	}
	if strings.HasPrefix(line, "took:") {
		p.Stage = "Finished"
		p.bump(1)
		p.Ready = true
	}
}

func (p *CreateProgress) bump(f float64) {
	// Stage labels only; the UI drives the bar from wall-clock vs average.
	_ = f
}

func CreateStream(bin string, onLine func(string)) (string, error) {
	return streamCmd(bin, 15*time.Minute, onLine, "create")
}

func DeleteStream(bin, idOrHost string, onLine func(string)) (string, error) {
	return streamCmd(bin, 10*time.Minute, onLine, "delete", idOrHost, "-yes")
}

func streamCmd(bin string, timeout time.Duration, onLine func(string), args ...string) (string, error) {
	cmd := exec.Command(bin, args...)
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		_ = pw.Close()
		return "", err
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
		_ = pw.Close()
	}()
	timer := time.AfterFunc(timeout, func() { _ = cmd.Process.Kill() })
	defer timer.Stop()

	var buf strings.Builder
	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		buf.WriteString(line)
		buf.WriteByte('\n')
		if onLine != nil {
			onLine(line)
		}
	}
	err := <-done
	out := buf.String()
	if err != nil {
		return out, err
	}
	return out, sc.Err()
}

func ApplyDeleteLine(p *CreateProgress, line string) {
	line = strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(line, "delete dns"):
		p.Stage = "Deleting DNS"
	case strings.HasPrefix(line, "dns wait"):
		p.Stage = "Waiting for DNS"
	case line == "dns gone":
		p.Stage = "DNS removed"
	case strings.HasPrefix(line, "delete linode"):
		p.Stage = "Deleting Linode"
	case strings.HasPrefix(line, "linode wait"):
		p.Stage = "Waiting for Linode"
	case line == "linode gone":
		p.Stage = "Linode removed"
	case strings.HasPrefix(line, "deleted "):
		p.Stage = "Deleted"
		p.Ready = true
	}
}

func ShortErr(err error, out string) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if i := strings.IndexByte(msg, '\n'); i > 0 {
		msg = msg[:i]
	}
	if len(msg) > 80 {
		msg = msg[:77] + "…"
	}
	_ = out
	return msg
}

func Delete(bin string, idOrHost string) (string, error) {
	return run(bin, 10*time.Minute, "delete", idOrHost, "-yes")
}

func View(bin, idOrHost string) (string, error) {
	return run(bin, 30*time.Second, "view", idOrHost)
}

func Admin(bin, idOrHost string) (string, error) {
	return run(bin, 30*time.Second, "admin", idOrHost)
}

type Field struct {
	Key, Value string
}

// ParseFields reads CLI "key: value" lines (view/admin). Non-matching lines are skipped.
func ParseFields(out string) []Field {
	var fields []Field
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" {
			continue
		}
		fields = append(fields, Field{Key: k, Value: v})
	}
	return fields
}
