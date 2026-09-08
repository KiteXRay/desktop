package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/goxray/core/wireguard"
	xray3 "github.com/lilendian0x00/xray-knife/v3/pkg/xray"
)

func buildLinkFromMap(cfg map[string]string) (string, error) {
	proto := strings.ToLower(strings.TrimSpace(cfg["Protocol"]))
	addr := strings.TrimSpace(cfg["Address"])
	port := strings.TrimSpace(cfg["Port"])
	id := strings.TrimSpace(cfg["ID"])
	remark := strings.TrimSpace(cfg["Remark"])

	if proto == "" {
		return "", fmt.Errorf("protocol is required")
	}
	if addr == "" {
		return "", fmt.Errorf("server address is required")
	}
	if port == "" {
		return "", fmt.Errorf("port is required")
	}

	var link string

	switch proto {
	case "vless":
		if id == "" {
			return "", fmt.Errorf("UUID / ID is required for VLESS")
		}
		u := fmt.Sprintf("vless://%s@%s:%s", id, addr, port)
		v := url.Values{}

		// Network / Transport
		netType := strings.TrimSpace(cfg["Network"])
		if netType == "" {
			netType = strings.TrimSpace(cfg["Type"])
		}
		if netType != "" {
			v.Set("type", netType)
		} else {
			v.Set("type", "tcp")
		}

		// Security
		sec := strings.TrimSpace(cfg["Security"])
		if sec == "" {
			sec = strings.TrimSpace(cfg["TLS"])
		}
		if sec != "" {
			v.Set("security", sec)
		}

		// Flow
		if f := strings.TrimSpace(cfg["Flow"]); f != "" {
			v.Set("flow", f)
		}

		// REALITY keys
		if pbk := strings.TrimSpace(cfg["Pbk"]); pbk != "" {
			v.Set("pbk", pbk)
		}
		if sid := strings.TrimSpace(cfg["Sid"]); sid != "" {
			v.Set("sid", sid)
		}
		if spx := strings.TrimSpace(cfg["Spx"]); spx != "" {
			v.Set("spx", spx)
		}

		// TLS settings
		if sni := strings.TrimSpace(cfg["SNI"]); sni != "" {
			v.Set("sni", sni)
		}
		fp := strings.TrimSpace(cfg["TlsFingerprint"])
		if fp == "" {
			fp = strings.TrimSpace(cfg["Fp"])
		}
		if fp != "" {
			v.Set("fp", fp)
		}
		if alpn := strings.TrimSpace(cfg["ALPN"]); alpn != "" {
			v.Set("alpn", alpn)
		}

		// Path & Host
		if path := strings.TrimSpace(cfg["Path"]); path != "" {
			v.Set("path", path)
		}
		if host := strings.TrimSpace(cfg["Host"]); host != "" {
			v.Set("host", host)
		}
		if sName := strings.TrimSpace(cfg["ServiceName"]); sName != "" {
			v.Set("serviceName", sName)
		}
		if hType := strings.TrimSpace(cfg["HeaderType"]); hType != "" {
			v.Set("headerType", hType)
		}
		if mode := strings.TrimSpace(cfg["Mode"]); mode != "" {
			v.Set("mode", mode)
		}
		if enc := strings.TrimSpace(cfg["Encryption"]); enc != "" {
			v.Set("encryption", enc)
		} else {
			v.Set("encryption", "none")
		}

		encodedQuery := v.Encode()
		link = u
		if encodedQuery != "" {
			link += "?" + encodedQuery
		}
		if remark != "" {
			link += "#" + url.PathEscape(remark)
		}

	case "vmess":
		if id == "" {
			return "", fmt.Errorf("UUID / ID is required for VMess")
		}
		aid := strings.TrimSpace(cfg["Aid"])
		if aid == "" {
			aid = "0"
		}
		scy := strings.TrimSpace(cfg["Security"])
		if scy == "" {
			scy = "auto"
		}
		netType := strings.TrimSpace(cfg["Network"])
		if netType == "" {
			netType = "tcp"
		}
		hType := strings.TrimSpace(cfg["HeaderType"])
		if hType == "" {
			hType = "none"
		}

		tlsSec := strings.TrimSpace(cfg["TLS"])
		if tlsSec == "" && (scy == "tls" || strings.TrimSpace(cfg["SNI"]) != "") {
			tlsSec = "tls"
		}

		fp := strings.TrimSpace(cfg["TlsFingerprint"])
		if fp == "" {
			fp = strings.TrimSpace(cfg["Fp"])
		}

		m := map[string]any{
			"v":    "2",
			"ps":   remark,
			"add":  addr,
			"port": port,
			"id":   id,
			"aid":  aid,
			"scy":  scy,
			"net":  netType,
			"type": hType,
			"host": strings.TrimSpace(cfg["Host"]),
			"path": strings.TrimSpace(cfg["Path"]),
			"tls":  tlsSec,
			"sni":  strings.TrimSpace(cfg["SNI"]),
			"alpn": strings.TrimSpace(cfg["ALPN"]),
			"fp":   fp,
		}

		b, err := json.Marshal(m)
		if err != nil {
			return "", fmt.Errorf("marshal vmess json: %w", err)
		}
		link = "vmess://" + base64.StdEncoding.EncodeToString(b)

	case "trojan":
		if id == "" {
			return "", fmt.Errorf("password / key is required for Trojan")
		}
		u := fmt.Sprintf("trojan://%s@%s:%s", id, addr, port)
		v := url.Values{}

		sec := strings.TrimSpace(cfg["Security"])
		if sec == "" {
			sec = "tls"
		}
		v.Set("security", sec)

		if sni := strings.TrimSpace(cfg["SNI"]); sni != "" {
			v.Set("sni", sni)
		}
		fp := strings.TrimSpace(cfg["TlsFingerprint"])
		if fp == "" {
			fp = strings.TrimSpace(cfg["Fp"])
		}
		if fp != "" {
			v.Set("fp", fp)
		}
		if alpn := strings.TrimSpace(cfg["ALPN"]); alpn != "" {
			v.Set("alpn", alpn)
		}
		netType := strings.TrimSpace(cfg["Network"])
		if netType == "" {
			netType = strings.TrimSpace(cfg["Type"])
		}
		if netType != "" {
			v.Set("type", netType)
		}
		if path := strings.TrimSpace(cfg["Path"]); path != "" {
			v.Set("path", path)
		}
		if host := strings.TrimSpace(cfg["Host"]); host != "" {
			v.Set("host", host)
		}

		encodedQuery := v.Encode()
		link = u
		if encodedQuery != "" {
			link += "?" + encodedQuery
		}
		if remark != "" {
			link += "#" + url.PathEscape(remark)
		}

	case "ss", "shadowsocks":
		if id == "" {
			return "", fmt.Errorf("password / key is required for Shadowsocks")
		}
		enc := strings.TrimSpace(cfg["Encryption"])
		if enc == "" {
			enc = "aes-256-gcm"
		}
		userPass := base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", enc, id)))
		link = fmt.Sprintf("ss://%s@%s:%s", userPass, addr, port)
		if remark != "" {
			link += "#" + url.PathEscape(remark)
		}

	case "wireguard":
		secretKey := strings.TrimSpace(cfg["SecretKey"])
		if secretKey == "" {
			secretKey = strings.TrimSpace(cfg["PrivateKey"])
		}
		if secretKey == "" {
			secretKey = id
		}
		if secretKey == "" {
			return "", fmt.Errorf("private/secret key is required for WireGuard")
		}

		pubKey := strings.TrimSpace(cfg["PublicKey"])
		if pubKey == "" {
			pubKey = strings.TrimSpace(cfg["Pubkey"])
		}
		if pubKey == "" {
			return "", fmt.Errorf("public key is required for WireGuard")
		}

		endpoint := addr
		if port != "" && !strings.Contains(addr, ":") {
			endpoint = fmt.Sprintf("%s:%s", addr, port)
		}

		localAddr := strings.TrimSpace(cfg["LocalAddress"])
		if localAddr == "" {
			localAddr = strings.TrimSpace(cfg["LocalIP"])
		}
		if localAddr == "" {
			localAddr = "10.0.0.2/32"
		}

		v := url.Values{}
		v.Set("publickey", pubKey)
		v.Set("address", localAddr)

		if psk := strings.TrimSpace(cfg["PreSharedKey"]); psk != "" {
			v.Set("presharedkey", psk)
		} else if psk := strings.TrimSpace(cfg["Psk"]); psk != "" {
			v.Set("presharedkey", psk)
		}

		mtu := strings.TrimSpace(cfg["MTU"])
		if mtu == "" {
			mtu = strings.TrimSpace(cfg["Mtu"])
		}
		if mtu != "" {
			v.Set("mtu", mtu)
		}

		if res := strings.TrimSpace(cfg["Reserved"]); res != "" {
			v.Set("reserved", res)
		}

		if aip := strings.TrimSpace(cfg["AllowedIPs"]); aip != "" {
			v.Set("allowedips", aip)
		} else if aip := strings.TrimSpace(cfg["Allowed_ips"]); aip != "" {
			v.Set("allowedips", aip)
		}
		if dns := strings.TrimSpace(cfg["DNS"]); dns != "" {
			v.Set("dns", dns)
		} else if dns := strings.TrimSpace(cfg["Dns"]); dns != "" {
			v.Set("dns", dns)
		}
		if ka := strings.TrimSpace(cfg["PersistentKeepalive"]); ka != "" {
			v.Set("persistentkeepalive", ka)
		} else if ka := strings.TrimSpace(cfg["Keepalive"]); ka != "" {
			v.Set("persistentkeepalive", ka)
		}

		encodedQuery := v.Encode()
		u := fmt.Sprintf("wireguard://%s@%s", url.PathEscape(secretKey), endpoint)
		link = u
		if encodedQuery != "" {
			link += "?" + encodedQuery
		}
		if remark != "" {
			link += "#" + url.PathEscape(remark)
		}

	case "awg", "amneziawg":
		secretKey := strings.TrimSpace(cfg["SecretKey"])
		if secretKey == "" {
			secretKey = strings.TrimSpace(cfg["PrivateKey"])
		}
		if secretKey == "" {
			secretKey = id
		}
		if secretKey == "" {
			return "", fmt.Errorf("private/secret key is required for AmneziaWG")
		}

		pubKey := strings.TrimSpace(cfg["PublicKey"])
		if pubKey == "" {
			pubKey = strings.TrimSpace(cfg["Pubkey"])
		}
		if pubKey == "" {
			return "", fmt.Errorf("public key is required for AmneziaWG")
		}

		endpoint := addr
		if port != "" && !strings.Contains(addr, ":") {
			endpoint = fmt.Sprintf("%s:%s", addr, port)
		}

		localAddr := strings.TrimSpace(cfg["LocalAddress"])
		if localAddr == "" {
			localAddr = strings.TrimSpace(cfg["LocalIP"])
		}
		if localAddr == "" {
			localAddr = "10.0.0.2/32"
		}

		v := url.Values{}
		v.Set("publickey", pubKey)
		v.Set("address", localAddr)

		if psk := strings.TrimSpace(cfg["PreSharedKey"]); psk != "" {
			v.Set("presharedkey", psk)
		} else if psk := strings.TrimSpace(cfg["Psk"]); psk != "" {
			v.Set("presharedkey", psk)
		}

		mtu := strings.TrimSpace(cfg["MTU"])
		if mtu == "" {
			mtu = strings.TrimSpace(cfg["Mtu"])
		}
		if mtu != "" {
			v.Set("mtu", mtu)
		}

		if res := strings.TrimSpace(cfg["Reserved"]); res != "" {
			v.Set("reserved", res)
		}

		if aip := strings.TrimSpace(cfg["AllowedIPs"]); aip != "" {
			v.Set("allowedips", aip)
		} else if aip := strings.TrimSpace(cfg["Allowed_ips"]); aip != "" {
			v.Set("allowedips", aip)
		}
		if dns := strings.TrimSpace(cfg["DNS"]); dns != "" {
			v.Set("dns", dns)
		} else if dns := strings.TrimSpace(cfg["Dns"]); dns != "" {
			v.Set("dns", dns)
		}
		if ka := strings.TrimSpace(cfg["PersistentKeepalive"]); ka != "" {
			v.Set("persistentkeepalive", ka)
		} else if ka := strings.TrimSpace(cfg["Keepalive"]); ka != "" {
			v.Set("persistentkeepalive", ka)
		}

		// AWG obfuscation headers
		for _, f := range []string{"jc", "jmin", "jmax", "s1", "s2", "s3", "s4", "h1", "h2", "h3", "h4", "i1", "i2", "i3", "i4", "i5"} {
			if val := strings.TrimSpace(cfg[f]); val != "" {
				v.Set(f, val)
			} else if val := strings.TrimSpace(cfg[strings.ToUpper(f)]); val != "" {
				v.Set(f, val)
			} else if val := strings.TrimSpace(cfg[strings.Title(f)]); val != "" {
				v.Set(f, val)
			}
		}

		encodedQuery := v.Encode()
		u := fmt.Sprintf("awg://%s@%s", url.PathEscape(secretKey), endpoint)
		link = u
		if encodedQuery != "" {
			link += "?" + encodedQuery
		}
		if remark != "" {
			link += "#" + url.PathEscape(remark)
		}

	default:
		return "", fmt.Errorf("unsupported protocol: %s", proto)
	}

	if proto == "wireguard" || proto == "awg" || proto == "amneziawg" {
		if _, _, err := wireguard.ParseLink(link); err != nil {
			return "", fmt.Errorf("invalid wireguard link: %w", err)
		}
		return link, nil
	}

	// Validate link using XRay Core
	protoInstance, err := (&xray3.Core{}).CreateProtocol(link)
	if err != nil {
		return "", fmt.Errorf("invalid protocol configuration: %w", err)
	}
	if err := protoInstance.Parse(); err != nil {
		return "", fmt.Errorf("failed to parse generated protocol: %w", err)
	}

	return link, nil
}
