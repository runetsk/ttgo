// Package safehttp provides HTTP clients and URL validators hardened against
// Server-Side Request Forgery (SSRF). The resolved destination IP is re-checked
// at connect time so DNS-rebinding cannot slip past a creation-time check.
//
// Two strictness levels exist:
//
//   - Strict (GuardedClient / ValidatePublicURL): denies ALL non-public
//     destinations (loopback, private, link-local, CGNAT, metadata). Use for
//     user-supplied outbound targets that should only ever reach the public
//     internet — webhooks and cloud LLM providers.
//
//   - Integration (IntegrationClient / ValidateIntegrationURL): denies only the
//     cloud-metadata / link-local range, but ALLOWS loopback and private hosts.
//     Use for admin-configured integrations (Jira/Confluence) that are
//     frequently self-hosted on a private network.
package safehttp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// DefaultTimeout is a sensible per-request timeout for outbound calls.
const DefaultTimeout = 15 * time.Second

const maxRedirects = 5

// ipDenied reports whether connecting to ip should be refused. When allowPrivate
// is true, loopback/private/CGNAT are permitted (self-hosted integrations) but
// link-local (which contains the 169.254.169.254 cloud-metadata address) is
// always denied.
func ipDenied(ip net.IP, allowPrivate bool) bool {
	if ip == nil ||
		ip.IsUnspecified() ||
		ip.IsMulticast() ||
		ip.IsLinkLocalUnicast() || // 169.254.0.0/16 (cloud metadata) and fe80::/10
		ip.IsLinkLocalMulticast() {
		return true
	}
	if allowPrivate {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() { // 10/8, 172.16/12, 192.168/16, fc00::/7
		return true
	}
	if ip4 := ip.To4(); ip4 != nil && ip4[0] == 100 && ip4[1]&0xc0 == 64 { // CGNAT 100.64/10
		return true
	}
	return false
}

// guardedDialContext re-resolves the host, rejects any denied address, and dials
// the validated IP directly (so a second, rebinding resolution can't occur).
func guardedDialContext(dialer *net.Dialer, allowPrivate bool) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if ipDenied(ip.IP, allowPrivate) {
				return nil, fmt.Errorf("safehttp: refusing to connect to blocked address %s (host %q)", ip.IP, host)
			}
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}
}

func newClient(timeout time.Duration, allowPrivate bool) *http.Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		DialContext:           guardedDialContext(dialer, allowPrivate),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("safehttp: stopped after %d redirects", maxRedirects)
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("safehttp: refusing redirect to scheme %q", req.URL.Scheme)
			}
			return nil // the guarded DialContext re-validates the destination IP on each hop
		},
	}
}

// GuardedClient returns a strict client that refuses any internal destination.
func GuardedClient(timeout time.Duration) *http.Client { return newClient(timeout, false) }

// IntegrationClient returns a client that allows loopback/private hosts (for
// self-hosted integrations) but still refuses link-local/cloud-metadata.
func IntegrationClient(timeout time.Duration) *http.Client { return newClient(timeout, true) }

func validateURL(raw string, allowPrivate bool) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https, got %q", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL has no host")
	}
	if ip := net.ParseIP(host); ip != nil {
		if ipDenied(ip, allowPrivate) {
			return fmt.Errorf("URL host %s is a blocked address", ip)
		}
		return nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil // transient/offline DNS — the dial-time guard remains authoritative
	}
	for _, ip := range ips {
		if ipDenied(ip, allowPrivate) {
			return fmt.Errorf("URL host %q resolves to blocked address %s", host, ip)
		}
	}
	return nil
}

// ValidatePublicURL strictly rejects http(s) URLs whose host is internal.
func ValidatePublicURL(raw string) error { return validateURL(raw, false) }

// ValidateIntegrationURL rejects metadata/link-local but allows private/loopback
// hosts, for admin-configured self-hostable integrations.
func ValidateIntegrationURL(raw string) error { return validateURL(raw, true) }
