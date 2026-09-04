package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

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

	default:
		return "", fmt.Errorf("unsupported protocol: %s", proto)
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
