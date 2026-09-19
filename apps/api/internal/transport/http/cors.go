package http

import (
	nethttp "net/http"
	"net/netip"
	"net/url"
	"strings"
)

// allowOrigin returns the CORS origin check: the configured origins always
// pass; in development so does any port on localhost and on private-network
// addresses, because dev servers (Expo, Vite) change ports and are opened
// from a phone or another machine by the LAN IP.
func allowOrigin(configured []string, dev bool) func(r *nethttp.Request, origin string) bool {
	allowed := make(map[string]bool, len(configured))
	for _, o := range configured {
		allowed[strings.ToLower(strings.TrimRight(strings.TrimSpace(o), "/"))] = true
	}
	return func(_ *nethttp.Request, origin string) bool {
		origin = strings.ToLower(origin)
		if allowed[origin] {
			return true
		}
		return dev && isLocalOrigin(origin)
	}
}

// isLocalOrigin reports whether the origin points at this machine or a
// private network (10/8, 172.16/12, 192.168/16, loopback, IPv6 ULA).
func isLocalOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" {
		return false
	}
	host := u.Hostname()
	if host == "localhost" {
		return true
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	return addr.IsLoopback() || addr.IsPrivate()
}
