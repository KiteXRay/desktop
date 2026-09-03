package route

import (
	"fmt"
	"net"
	"strings"
)

// Addr represents IP and Port.
type Addr net.IPNet

func (a *Addr) String() string {
	if a.Mask == nil {
		return a.IP.String()
	}

	return (*net.IPNet)(a).String()
}

// ParseAddr parses the given addr and transforms it into Addr.
//
// If CIDR notation is present, parses via net.ParseCIDR.
// If plain IP, resolves it and assigns full host mask (/32 for IPv4, /128 for IPv6).
func ParseAddr(addr string) (*Addr, error) {
	if strings.Contains(addr, "/") {
		_, ipNet, err := net.ParseCIDR(addr)
		if err != nil {
			return nil, fmt.Errorf("parse cidr: %w", err)
		}
		return (*Addr)(ipNet), nil
	}

	ip, _ := net.ResolveIPAddr("ip", addr)
	if ip != nil {
		mask := net.CIDRMask(32, 32)
		if ip.IP.To4() == nil {
			mask = net.CIDRMask(128, 128)
		}
		return &Addr{
			IP:   ip.IP,
			Mask: mask,
		}, nil
	}

	_, ipNet, err := net.ParseCIDR(addr)
	if err != nil {
		return nil, fmt.Errorf("parse cidr: %w", err)
	}

	return (*Addr)(ipNet), nil
}

// MustParseAddr is the same as ParseAddr but panics on errors.
func MustParseAddr(addr string) *Addr {
	a, err := ParseAddr(addr)
	if err != nil {
		panic(err)
	}

	return a
}
