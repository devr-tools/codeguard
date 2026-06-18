// Package safehttp hardens codeguard's outbound AI provider HTTP calls against
// SSRF and credential exfiltration.
//
// Because the AI provider base URL and the name of the environment variable
// holding the API key can be set from repository-checked-in config, an
// untrusted pull request could otherwise point codeguard at an attacker-
// controlled host and capture the operator's provider API key. By default this
// package:
//
//   - restricts config-supplied provider base URLs to a small allowlist of
//     known public provider hosts; and
//   - blocks outbound connections to non-public (loopback/private/link-local)
//     addresses, defending against DNS-rebinding and redirect-based SSRF.
//
// Both protections are relaxed when the operator opts in via the trust policy
// (CODEGUARD_ALLOW_CONFIG_AI_ENDPOINTS / --allow-config-ai-endpoints), which is
// required for self-hosted or internal provider endpoints.
package safehttp

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/trust"
)

// MaxResponseBytes caps how much of an AI provider response body codeguard will
// read, so a malicious or compromised endpoint cannot exhaust memory.
const MaxResponseBytes = 16 << 20 // 16 MiB

// allowedProviderHosts is the set of public provider hosts codeguard will talk
// to when the base URL comes from untrusted config without an opt-in.
var allowedProviderHosts = map[string]bool{
	"api.openai.com":    true,
	"api.anthropic.com": true,
}

func allowedHostList() string {
	hosts := make([]string, 0, len(allowedProviderHosts))
	for host := range allowedProviderHosts {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)
	return strings.Join(hosts, ", ")
}

// ValidateProviderURL reports an error when rawURL is not an acceptable AI
// provider base URL under the active trust policy. An empty URL is accepted
// (the provider default is used). trustedSource should be true when the URL
// originates from a trusted source such as a process environment variable
// rather than repository config.
func ValidateProviderURL(rawURL string, trustedSource bool) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid AI provider base URL %q: %w", rawURL, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("AI provider base URL %q must use http or https", rawURL)
	}
	if parsed.Hostname() == "" {
		return fmt.Errorf("AI provider base URL %q has no host", rawURL)
	}
	if trustedSource || trust.AllowConfigAIEndpoints() {
		return nil
	}
	host := strings.ToLower(parsed.Hostname())
	if allowedProviderHosts[host] {
		return nil
	}
	return fmt.Errorf(
		"AI provider base URL host %q is not on the allowlist (%s); it was set from repository "+
			"configuration. Set %s=1 or pass --allow-config-ai-endpoints to use a custom endpoint.",
		host, allowedHostList(), trust.AllowConfigAIEndpointsEnv)
}

// Client returns an HTTP client for AI provider calls. Unless the operator has
// opted into custom AI endpoints, the client refuses to connect to non-public
// addresses at dial time.
func Client(timeout time.Duration) *http.Client {
	if trust.AllowConfigAIEndpoints() {
		return &http.Client{Timeout: timeout}
	}
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   blockNonPublicAddress,
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: time.Second,
		},
	}
}

// blockNonPublicAddress is a net.Dialer Control hook that rejects connections to
// loopback, private, link-local, and otherwise non-routable addresses. It runs
// on the already-resolved address, so it also defends against a hostname that
// resolves to an internal IP (DNS rebinding).
func blockNonPublicAddress(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("ssrf guard: cannot parse dial address %q: %w", address, err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("ssrf guard: cannot parse IP %q", host)
	}
	if isNonPublicIP(ip) {
		return fmt.Errorf("ssrf guard: refusing to connect to non-public address %s", ip)
	}
	return nil
}

func isNonPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}
	// IPv4 broadcast and the IPv4-mapped/IPv6 documentation ranges are not
	// covered by the helpers above; treat the cloud metadata address explicitly.
	if v4 := ip.To4(); v4 != nil {
		if v4[0] == 169 && v4[1] == 254 { // link-local incl. 169.254.169.254 metadata
			return true
		}
	}
	return false
}
