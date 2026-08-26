package config

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

// Server is where `tq serve` binds, when the project pins it. Both keys are
// optional, and a project that sets neither behaves exactly as before.
//
// A `server:` block rather than a bare `port:` because `host` is its immediate
// neighbour and belongs in the same place.
type Server struct {
	Host string `yaml:"host" json:"host,omitempty"`

	// Port is a pointer because zero is a real answer: `port: 0` asks the OS
	// to pick one, which the test harnesses rely on. A plain int could not
	// tell that apart from a project pinning nothing at all.
	Port *int `yaml:"port" json:"port,omitempty"`
}

// MaxPort is the highest TCP port there is.
const MaxPort = 65535

// ServerHost is the host the project pins, or "" when it pins none. A nil
// *Config — no config file — is the same case, so callers never have to check
// before asking.
func (c *Config) ServerHost() string {
	if c == nil {
		return ""
	}
	return c.Server.Host
}

// ServerPort is the port the project pins, and whether it pins one at all.
// Zero is a real port here, so the bool is the only thing that says absent.
func (c *Config) ServerPort() (int, bool) {
	if c == nil || c.Server.Port == nil {
		return 0, false
	}
	return *c.Server.Port, true
}

// hostPattern is the shape of a hostname, for the cases ParseIP rejects. It is
// deliberately loose — resolving a name at load time would make `tq list` wait
// on DNS — and only exists to catch the spellings that are not addresses at all.
var hostPattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?)*$`)

// validateServer keeps a nonsense address from reaching the listener, where the
// failure would be a bind error that says nothing about the config.
func validateServer(s Server) error {
	if s.Port != nil && (*s.Port < 0 || *s.Port > MaxPort) {
		return fmt.Errorf("server port %d is out of range (0-%d; 0 asks the OS to pick one)", *s.Port, MaxPort)
	}
	// A bracketed IPv6 address is the spelling people reach for, because it is
	// how the address is written once a port is on the end of it. Caught here
	// rather than at the listener, which would report `[[::1]]:7331` and name
	// neither the file nor the key — and ExposedHost cannot parse it either, so
	// it would warn that loopback reaches the network.
	if host := s.Host; host != "" && net.ParseIP(host) == nil && !hostPattern.MatchString(host) {
		hint := ""
		if trimmed := strings.Trim(host, "[]"); net.ParseIP(trimmed) != nil {
			hint = fmt.Sprintf(" (write it unbracketed, as %s)", trimmed)
		}
		return fmt.Errorf("server host %q is neither an IP address nor a hostname%s", host, hint)
	}
	return nil
}

// ExposedHost reports whether binding to this host reaches off the machine.
//
// The board has no authentication in front of it, so a project that commits a
// non-loopback host exposes every clone of itself on whatever network it is
// run. That is worth a word on start-up rather than a surprise.
//
// Anything that is not recognisably loopback counts as exposed, including a
// hostname this cannot resolve: the safe direction to be wrong in is warning
// about an address that turns out to be local.
func ExposedHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return !ip.IsLoopback()
	}
	// "" is every interface, and an unresolvable name could be anything.
	return true
}
