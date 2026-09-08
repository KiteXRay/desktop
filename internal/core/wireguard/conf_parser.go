package wireguard

import (
	"bufio"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// Config represents parsed parameters from a WireGuard or AmneziaWG configuration file.
type Config struct {
	// Interface
	PrivateKey string
	Address    string
	DNS        string
	MTU        int

	// AmneziaWG parameters
	IsAmnezia bool
	Jc        int
	Jmin      int
	Jmax      int
	S1        int
	S2        int
	H1        int64
	H2        int64
	H3        int64
	H4        int64

	// Peer
	PublicKey           string
	PreSharedKey        string
	Endpoint            string
	AllowedIPs          string
	PersistentKeepalive int
	Reserved            string
}

// IsConfContent checks whether input text is a WireGuard/AmneziaWG INI configuration file.
func IsConfContent(text string) bool {
	trimmed := strings.TrimSpace(text)
	hasInterface := strings.Contains(strings.ToLower(trimmed), "[interface]")
	hasPeer := strings.Contains(strings.ToLower(trimmed), "[peer]")
	return hasInterface && hasPeer
}

// ParseConf parses standard WireGuard / AmneziaWG .conf file content into a Config.
func ParseConf(content string) (*Config, error) {
	cfg := &Config{
		MTU: 1420,
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	currentSection := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.ToLower(line[1 : len(line)-1])
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])
		// Strip inline comments
		if idx := strings.IndexAny(val, "#;"); idx != -1 {
			val = strings.TrimSpace(val[:idx])
		}

		switch currentSection {
		case "interface":
			switch key {
			case "privatekey":
				cfg.PrivateKey = val
			case "address":
				cfg.Address = val
			case "dns":
				cfg.DNS = val
			case "mtu":
				if m, err := strconv.Atoi(val); err == nil && m > 0 {
					cfg.MTU = m
				}
			case "jc":
				if v, err := strconv.Atoi(val); err == nil {
					cfg.Jc = v
					cfg.IsAmnezia = true
				}
			case "jmin":
				if v, err := strconv.Atoi(val); err == nil {
					cfg.Jmin = v
					cfg.IsAmnezia = true
				}
			case "jmax":
				if v, err := strconv.Atoi(val); err == nil {
					cfg.Jmax = v
					cfg.IsAmnezia = true
				}
			case "s1":
				if v, err := strconv.Atoi(val); err == nil {
					cfg.S1 = v
					cfg.IsAmnezia = true
				}
			case "s2":
				if v, err := strconv.Atoi(val); err == nil {
					cfg.S2 = v
					cfg.IsAmnezia = true
				}
			case "h1":
				if v, err := strconv.ParseInt(val, 10, 64); err == nil {
					cfg.H1 = v
					cfg.IsAmnezia = true
				}
			case "h2":
				if v, err := strconv.ParseInt(val, 10, 64); err == nil {
					cfg.H2 = v
					cfg.IsAmnezia = true
				}
			case "h3":
				if v, err := strconv.ParseInt(val, 10, 64); err == nil {
					cfg.H3 = v
					cfg.IsAmnezia = true
				}
			case "h4":
				if v, err := strconv.ParseInt(val, 10, 64); err == nil {
					cfg.H4 = v
					cfg.IsAmnezia = true
				}
			}

		case "peer":
			switch key {
			case "publickey":
				cfg.PublicKey = val
			case "presharedkey":
				cfg.PreSharedKey = val
			case "endpoint":
				cfg.Endpoint = val
			case "allowedips":
				cfg.AllowedIPs = val
			case "persistentkeepalive":
				if k, err := strconv.Atoi(val); err == nil {
					cfg.PersistentKeepalive = k
				}
			case "reserved":
				cfg.Reserved = val
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read conf: %w", err)
	}

	if cfg.PrivateKey == "" {
		return nil, fmt.Errorf("missing PrivateKey in [Interface]")
	}
	if cfg.PublicKey == "" {
		return nil, fmt.Errorf("missing PublicKey in [Peer]")
	}
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("missing Endpoint in [Peer]")
	}

	return cfg, nil
}

// ToURI converts the configuration into a Kite wireguard:// or awg:// URI.
func (c *Config) ToURI(label string) string {
	scheme := "wireguard"
	if c.IsAmnezia {
		scheme = "awg"
	}

	v := url.Values{}
	v.Set("publickey", c.PublicKey)

	if c.Address != "" {
		v.Set("address", c.Address)
	} else {
		v.Set("address", "10.0.0.2/32")
	}

	if c.PreSharedKey != "" {
		v.Set("presharedkey", c.PreSharedKey)
	}

	if c.MTU > 0 {
		v.Set("mtu", strconv.Itoa(c.MTU))
	}

	if c.Reserved != "" {
		v.Set("reserved", c.Reserved)
	}

	if c.IsAmnezia {
		if c.Jc > 0 {
			v.Set("jc", strconv.Itoa(c.Jc))
		}
		if c.Jmin > 0 {
			v.Set("jmin", strconv.Itoa(c.Jmin))
		}
		if c.Jmax > 0 {
			v.Set("jmax", strconv.Itoa(c.Jmax))
		}
		if c.S1 > 0 {
			v.Set("s1", strconv.Itoa(c.S1))
		}
		if c.S2 > 0 {
			v.Set("s2", strconv.Itoa(c.S2))
		}
		if c.H1 > 0 {
			v.Set("h1", strconv.FormatInt(c.H1, 10))
		}
		if c.H2 > 0 {
			v.Set("h2", strconv.FormatInt(c.H2, 10))
		}
		if c.H3 > 0 {
			v.Set("h3", strconv.FormatInt(c.H3, 10))
		}
		if c.H4 > 0 {
			v.Set("h4", strconv.FormatInt(c.H4, 10))
		}
	}

	uri := fmt.Sprintf("%s://%s@%s", scheme, url.PathEscape(c.PrivateKey), c.Endpoint)
	q := v.Encode()
	if q != "" {
		uri += "?" + q
	}
	if label != "" {
		uri += "#" + url.PathEscape(label)
	}

	return uri
}

// IsAWGLink checks whether a link is a WireGuard or AmneziaWG URI.
func IsAWGLink(link string) bool {
	l := strings.ToLower(strings.TrimSpace(link))
	return strings.HasPrefix(l, "awg://") || strings.HasPrefix(l, "wireguard://")
}

// IsWireguardLink checks whether a link is a WireGuard or AmneziaWG URI.
func IsWireguardLink(link string) bool {
	return IsAWGLink(link)
}

// ParseLink parses an awg:// or wireguard:// link into a Config and label.
func ParseLink(rawLink string) (*Config, string, error) {
	rawLink = strings.TrimSpace(rawLink)
	u, err := url.Parse(rawLink)
	if err != nil {
		return nil, "", err
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "wireguard" && scheme != "awg" {
		return nil, "", fmt.Errorf("unsupported scheme: %s", scheme)
	}

	var sk string
	if u.User != nil {
		sk, _ = url.PathUnescape(u.User.Username())
	}
	remark, _ := url.PathUnescape(u.Fragment)

	q := u.Query()
	cfg := &Config{
		PrivateKey:   sk,
		Endpoint:     u.Host,
		PublicKey:    q.Get("publickey"),
		Address:      q.Get("address"),
		PreSharedKey: q.Get("presharedkey"),
		Reserved:     q.Get("reserved"),
		DNS:          q.Get("dns"),
		AllowedIPs:   q.Get("allowedips"),
		MTU:          1420,
	}

	if cfg.Address == "" {
		cfg.Address = q.Get("ip")
	}
	if cfg.AllowedIPs == "" {
		cfg.AllowedIPs = q.Get("allowed_ips")
	}
	if cfg.PreSharedKey == "" {
		cfg.PreSharedKey = q.Get("preshared_key")
	}
	if cfg.PreSharedKey == "" {
		cfg.PreSharedKey = q.Get("psk")
	}
	if ka := q.Get("persistentkeepalive"); ka != "" {
		if v, err := strconv.Atoi(ka); err == nil && v > 0 {
			cfg.PersistentKeepalive = v
		}
	} else if ka := q.Get("keepalive"); ka != "" {
		if v, err := strconv.Atoi(ka); err == nil && v > 0 {
			cfg.PersistentKeepalive = v
		}
	}

	if scheme == "awg" {
		cfg.IsAmnezia = true
	}

	if m, err := strconv.Atoi(q.Get("mtu")); err == nil && m > 0 {
		cfg.MTU = m
	}
	if v, err := strconv.Atoi(q.Get("jc")); err == nil && v > 0 {
		cfg.Jc = v
		cfg.IsAmnezia = true
	}
	if v, err := strconv.Atoi(q.Get("jmin")); err == nil && v > 0 {
		cfg.Jmin = v
		cfg.IsAmnezia = true
	}
	if v, err := strconv.Atoi(q.Get("jmax")); err == nil && v > 0 {
		cfg.Jmax = v
		cfg.IsAmnezia = true
	}
	if v, err := strconv.Atoi(q.Get("s1")); err == nil && v > 0 {
		cfg.S1 = v
		cfg.IsAmnezia = true
	}
	if v, err := strconv.Atoi(q.Get("s2")); err == nil && v > 0 {
		cfg.S2 = v
		cfg.IsAmnezia = true
	}
	if v, err := strconv.ParseInt(q.Get("h1"), 10, 64); err == nil && v > 0 {
		cfg.H1 = v
		cfg.IsAmnezia = true
	}
	if v, err := strconv.ParseInt(q.Get("h2"), 10, 64); err == nil && v > 0 {
		cfg.H2 = v
		cfg.IsAmnezia = true
	}
	if v, err := strconv.ParseInt(q.Get("h3"), 10, 64); err == nil && v > 0 {
		cfg.H3 = v
		cfg.IsAmnezia = true
	}
	if v, err := strconv.ParseInt(q.Get("h4"), 10, 64); err == nil && v > 0 {
		cfg.H4 = v
		cfg.IsAmnezia = true
	}

	return cfg, remark, nil
}

// ToMap converts the Config to a key-value map for Kite's ConnectionDTO.
func (c *Config) ToMap(remark string) map[string]string {
	proto := "wireguard"
	if c.IsAmnezia {
		proto = "awg"
	}
	host := c.Endpoint
	port := "51820"
	if h, p, err := net.SplitHostPort(c.Endpoint); err == nil {
		host = h
		port = p
	}

	m := map[string]string{
		"Protocol":     proto,
		"Address":      host,
		"Port":         port,
		"Endpoint":     c.Endpoint,
		"Publickey":    c.PublicKey,
		"Secretkey":    c.PrivateKey,
		"LocalAddress": c.Address,
		"Remark":       remark,
		"Mtu":          strconv.Itoa(c.MTU),
	}
	if c.PreSharedKey != "" {
		m["Presharedkey"] = c.PreSharedKey
	}
	if c.Reserved != "" {
		m["Reserved"] = c.Reserved
	}
	if c.IsAmnezia {
		if c.Jc > 0 {
			m["Jc"] = strconv.Itoa(c.Jc)
		}
		if c.Jmin > 0 {
			m["Jmin"] = strconv.Itoa(c.Jmin)
		}
		if c.Jmax > 0 {
			m["Jmax"] = strconv.Itoa(c.Jmax)
		}
		if c.S1 > 0 {
			m["S1"] = strconv.Itoa(c.S1)
		}
		if c.S2 > 0 {
			m["S2"] = strconv.Itoa(c.S2)
		}
		if c.H1 > 0 {
			m["H1"] = strconv.FormatInt(c.H1, 10)
		}
		if c.H2 > 0 {
			m["H2"] = strconv.FormatInt(c.H2, 10)
		}
		if c.H3 > 0 {
			m["H3"] = strconv.FormatInt(c.H3, 10)
		}
		if c.H4 > 0 {
			m["H4"] = strconv.FormatInt(c.H4, 10)
		}
	}
	return m
}
