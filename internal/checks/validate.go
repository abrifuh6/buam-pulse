package checks

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ValidateTarget rejects targets that would let a customer use Pulse to reach
// systems they shouldn't: our own cloud metadata endpoint, private networks,
// loopback, or link-local addresses. A monitoring service fetches arbitrary
// URLs on a schedule, which makes it an SSRF vector and a DDoS amplifier if
// left open. This runs at creation time; ResolveGuard runs again at check time
// because DNS can be re-pointed after the fact (a DNS rebinding attack).
func ValidateTarget(kind, target string) error {
	var host string

	switch kind {
	case "http":
		u, err := url.Parse(target)
		if err != nil {
			return fmt.Errorf("invalid URL")
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("target must be a full URL, e.g. https://example.com")
		}
		if u.Host == "" {
			return fmt.Errorf("target must include a hostname")
		}
		host = u.Hostname()
	case "tcp":
		h, _, err := net.SplitHostPort(target)
		if err != nil {
			return fmt.Errorf("target must be host:port")
		}
		host = h
	default:
		return fmt.Errorf("unknown monitor type")
	}

	if host == "" {
		return fmt.Errorf("target must include a hostname")
	}
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") ||
		strings.HasSuffix(lower, ".internal") || strings.HasSuffix(lower, ".local") {
		return fmt.Errorf("target must be a public host")
	}

	// If the user gave a literal IP, check it now. If it's a name, DNS
	// resolution at check time is guarded by ResolveGuard.
	if ip := net.ParseIP(host); ip != nil && !isPublicIP(ip) {
		return fmt.Errorf("target must be a public host")
	}
	return nil
}

// ResolveGuard resolves a hostname and fails if any address is non-public.
// Called on every check, not just at creation, because the DNS record behind a
// hostname can be changed to point at a private address after validation.
func ResolveGuard(host string) error {
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("dns lookup failed: %w", err)
	}
	for _, ip := range ips {
		if !isPublicIP(ip) {
			return fmt.Errorf("target resolves to a non-public address")
		}
	}
	return nil
}

func isPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	// 169.254.169.254 is covered by IsLinkLocalUnicast, but block the whole
	// carrier-grade NAT range too — it shows up in cloud provider networks.
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return false
		}
	}
	return true
}
