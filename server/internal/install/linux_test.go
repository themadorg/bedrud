package install

import "testing"

func TestDerivePublicEndpoints(t *testing.T) {
	const (
		lanIP  = "192.168.1.10"
		domain = "meet.example.com"
	)
	tests := []struct {
		name string
		cfg  InstallConfig
		want publicEndpoints
	}{
		{
			name: "should use https without the listen port behind a TLS-terminating proxy",
			cfg:  InstallConfig{BehindProxy: true, DisableTLS: true, Port: "8080", Domain: domain, OverrideIP: lanIP},
			want: publicEndpoints{
				ServerHost:  "0.0.0.0",
				LiveKitHost: "https://meet.example.com/livekit",
				CORSOrigins: "https://meet.example.com",
				FrontendURL: "https://meet.example.com",
			},
		},
		{
			name: "should treat a domain without TLS as behind a proxy",
			cfg:  InstallConfig{Port: "8090", Domain: domain, OverrideIP: lanIP},
			want: publicEndpoints{
				ServerHost:  "0.0.0.0",
				LiveKitHost: "https://meet.example.com/livekit",
				CORSOrigins: "https://meet.example.com",
				FrontendURL: "https://meet.example.com",
			},
		},
		{
			name: "should bind all interfaces behind a proxy with TLS on 443",
			cfg:  InstallConfig{BehindProxy: true, EnableTLS: true, Port: "443", Domain: domain, OverrideIP: lanIP},
			want: publicEndpoints{
				ServerHost:  "0.0.0.0",
				LiveKitHost: "https://meet.example.com/livekit",
				CORSOrigins: "https://meet.example.com",
				FrontendURL: "https://meet.example.com",
			},
		},
		{
			name: "should keep the detected IP and port for direct TLS on a custom port",
			cfg:  InstallConfig{EnableTLS: true, Port: "8443", Domain: domain, OverrideIP: lanIP},
			want: publicEndpoints{
				ServerHost:  lanIP,
				LiveKitHost: "https://meet.example.com:8443/livekit",
				CORSOrigins: "https://meet.example.com",
				FrontendURL: "https://meet.example.com",
			},
		},
		{
			name: "should drop the port for direct TLS on 443",
			cfg:  InstallConfig{EnableTLS: true, Port: "443", Domain: domain, OverrideIP: lanIP},
			want: publicEndpoints{
				ServerHost:  lanIP,
				LiveKitHost: "https://meet.example.com/livekit",
				CORSOrigins: "https://meet.example.com",
				FrontendURL: "https://meet.example.com",
			},
		},
		{
			name: "should use plain http on the IP without a domain",
			cfg:  InstallConfig{DisableTLS: true, Port: "8090", OverrideIP: lanIP},
			want: publicEndpoints{
				ServerHost:  lanIP,
				LiveKitHost: "http://192.168.1.10:8090/livekit",
				CORSOrigins: "*",
				FrontendURL: "",
			},
		},
		{
			name: "should use https on the IP behind a proxy without a domain",
			cfg:  InstallConfig{BehindProxy: true, DisableTLS: true, Port: "8080", OverrideIP: lanIP},
			want: publicEndpoints{
				ServerHost:  "0.0.0.0",
				LiveKitHost: "https://192.168.1.10/livekit",
				CORSOrigins: "*",
				FrontendURL: "",
			},
		},
		{
			name: "should keep the external LiveKit URL behind a proxy",
			cfg:  InstallConfig{BehindProxy: true, DisableTLS: true, Port: "8080", Domain: domain, OverrideIP: lanIP, ExternalLKURL: "https://lk.example.com"},
			want: publicEndpoints{
				ServerHost:  "0.0.0.0",
				LiveKitHost: "https://lk.example.com",
				CORSOrigins: "https://meet.example.com",
				FrontendURL: "https://meet.example.com",
			},
		},
		{
			name: "should keep the separate LiveKit domain behind a proxy",
			cfg:  InstallConfig{BehindProxy: true, DisableTLS: true, Port: "8080", Domain: domain, OverrideIP: lanIP, LKDomain: "lk.example.com"},
			want: publicEndpoints{
				ServerHost:  "0.0.0.0",
				LiveKitHost: "https://lk.example.com",
				CORSOrigins: "https://meet.example.com",
				FrontendURL: "https://meet.example.com",
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := derivePublicEndpoints(&testCase.cfg)
			if got != testCase.want {
				t.Errorf("derivePublicEndpoints() =\n  %+v\nwant\n  %+v", got, testCase.want)
			}
		})
	}
}
