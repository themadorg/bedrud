package livekit

type ConfigYAML struct {
	Port          int               `yaml:"port"`
	BindAddresses []string          `yaml:"bind_addresses,omitempty"`
	Keys          map[string]string `yaml:"keys"`
	RTC           struct {
		TCPPort        int    `yaml:"tcp_port,omitempty"`
		UDPPort        int    `yaml:"udp_port,omitempty"`
		PortRangeStart int    `yaml:"port_range_start,omitempty"`
		PortRangeEnd   int    `yaml:"port_range_end,omitempty"`
		UseExternalIP  bool   `yaml:"use_external_ip"`
		NodeIP         string `yaml:"node_ip"`
	} `yaml:"rtc"`
	TURN struct {
		Enabled  bool   `yaml:"enabled,omitempty"`
		Domain   string `yaml:"domain,omitempty"`
		UDPPort  int    `yaml:"udp_port,omitempty"`
		TLSPort  int    `yaml:"tls_port,omitempty"`
		CertFile string `yaml:"cert_file,omitempty"`
		KeyFile  string `yaml:"key_file,omitempty"`
	} `yaml:"turn"`
	Webhook struct {
		URLs   []string `yaml:"urls"`
		APIKey string   `yaml:"api_key"`
	} `yaml:"webhook"`
	Logging struct {
		JSON  bool   `yaml:"json,omitempty"`
		Level string `yaml:"level,omitempty"`
	} `yaml:"logging"`
}
