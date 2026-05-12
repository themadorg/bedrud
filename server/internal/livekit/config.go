package livekit

type ConfigYAML struct {
	Port          string            `yaml:"port"`
	BindAddresses []string          `yaml:"bind_addresses"`
	Keys          map[string]string `yaml:"keys"`
	RTC           struct {
		TCPPort        string `yaml:"tcp_port"`
		UDPPort        string `yaml:"udp_port,omitempty"`
		PortRangeStart string `yaml:"port_range_start,omitempty"`
		PortRangeEnd   string `yaml:"port_range_end,omitempty"`
		UseExternalIP  bool   `yaml:"use_external_ip"`
		NodeIP         string `yaml:"node_ip"`
	} `yaml:"rtc"`
	TURN struct {
		Enabled  bool   `yaml:"enabled"`
		Domain   string `yaml:"domain"`
		UDPPort  int    `yaml:"udp_port"`
		TLSPort  int    `yaml:"tls_port,omitempty"`
		CertFile string `yaml:"cert_file,omitempty"`
		KeyFile  string `yaml:"key_file,omitempty"`
	} `yaml:"turn"`
	Logging struct {
		JSON  bool   `yaml:"json"`
		Level string `yaml:"level"`
	} `yaml:"logging"`
}
