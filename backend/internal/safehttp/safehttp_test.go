package safehttp

import (
	"net"
	"testing"
)

func TestIpDenied_StrictBlocksAllInternal(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "::1", // loopback
		"169.254.169.254",      // cloud metadata (link-local)
		"10.0.0.1", "10.1.2.3", // private
		"172.16.5.4",          // private
		"192.168.1.1",         // private
		"100.64.0.1",          // CGNAT
		"0.0.0.0",             // unspecified
		"fe80::1",             // link-local v6
		"fc00::1", "fd12::34", // ULA v6
		"224.0.0.1", // multicast
	}
	for _, s := range blocked {
		ip := net.ParseIP(s)
		if ip == nil {
			t.Fatalf("bad test IP %q", s)
		}
		if !ipDenied(ip, false) {
			t.Errorf("ipDenied(%s, strict) = false, want true", s)
		}
	}
	for _, s := range []string{"8.8.8.8", "1.1.1.1", "203.0.113.10", "2606:4700:4700::1111"} {
		if ipDenied(net.ParseIP(s), false) {
			t.Errorf("ipDenied(%s, strict) = true, want false (public)", s)
		}
	}
}

func TestIpDenied_IntegrationAllowsPrivateButNotMetadata(t *testing.T) {
	// Self-hosted integrations may use loopback/private hosts.
	for _, s := range []string{"127.0.0.1", "10.0.0.1", "192.168.1.10", "172.16.0.1"} {
		if ipDenied(net.ParseIP(s), true) {
			t.Errorf("ipDenied(%s, integration) = true, want false (private allowed)", s)
		}
	}
	// But cloud metadata / link-local is always denied.
	for _, s := range []string{"169.254.169.254", "fe80::1", "0.0.0.0"} {
		if !ipDenied(net.ParseIP(s), true) {
			t.Errorf("ipDenied(%s, integration) = false, want true (metadata/link-local)", s)
		}
	}
}

func TestValidatePublicURL_RejectsInternalAndBadScheme(t *testing.T) {
	bad := []string{
		"http://127.0.0.1/x",
		"https://169.254.169.254/latest/meta-data",
		"http://10.0.0.5:8080/internal",
		"https://[::1]/x",
		"ftp://example.com/x",
		"file:///etc/passwd",
		"javascript:alert(1)",
		"http://[fd00::1]/x",
	}
	for _, raw := range bad {
		if err := ValidatePublicURL(raw); err == nil {
			t.Errorf("ValidatePublicURL(%q) = nil, want error", raw)
		}
	}
	for _, raw := range []string{"https://8.8.8.8/x", "http://1.1.1.1/y"} {
		if err := ValidatePublicURL(raw); err != nil {
			t.Errorf("ValidatePublicURL(%q) = %v, want nil", raw, err)
		}
	}
}

func TestValidateIntegrationURL_AllowsPrivateRejectsMetadata(t *testing.T) {
	for _, raw := range []string{"https://jira.internal/x", "http://10.0.0.5:8080/rest", "http://127.0.0.1:9000/api"} {
		if err := ValidateIntegrationURL(raw); err != nil {
			t.Errorf("ValidateIntegrationURL(%q) = %v, want nil (private allowed)", raw, err)
		}
	}
	for _, raw := range []string{"http://169.254.169.254/x", "ftp://10.0.0.1/x"} {
		if err := ValidateIntegrationURL(raw); err == nil {
			t.Errorf("ValidateIntegrationURL(%q) = nil, want error", raw)
		}
	}
}
