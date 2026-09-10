package bridge

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	M "github.com/xjasonlyu/tun2socks/v2/metadata"
	"github.com/xjasonlyu/tun2socks/v2/proxy"
)

// BridgeDialer routes connections either through a proxy or directly based on active Bridge rules.
type BridgeDialer struct {
	defaultProxyAddr string
	defaultProxy     proxy.Proxy
	directProxy      proxy.Proxy
	rulesGetter      func() []BridgeRule
	groupsGetter     func() []BridgeGroup
	logger           *slog.Logger

	mu            sync.RWMutex
	customProxies map[string]proxy.Proxy
}

func NewBridgeDialer(defaultSocksAddr string, rulesGetter func() []BridgeRule, logger *slog.Logger, groupsGetter ...func() []BridgeGroup) *BridgeDialer {
	if logger == nil {
		logger = slog.Default()
	}

	var defProxy proxy.Proxy
	if defaultSocksAddr != "" {
		parsed, err := parseProxyHostPort(defaultSocksAddr)
		if err == nil {
			defProxy, _ = newSocks5Proxy(parsed, "", "")
		}
	}

	var gg func() []BridgeGroup
	if len(groupsGetter) > 0 {
		gg = groupsGetter[0]
	}

	return &BridgeDialer{
		defaultProxyAddr: defaultSocksAddr,
		defaultProxy:     defProxy,
		directProxy:      newBridgeDirectProxy(),
		rulesGetter:      rulesGetter,
		groupsGetter:     gg,
		logger:           logger,
		customProxies:    make(map[string]proxy.Proxy),
	}
}

func getProcessWithRetry(isTCP bool, port uint16) (string, uint32) {
	procName, pid := GetProcessByPort(isTCP, port)
	if procName != "" {
		return procName, pid
	}
	for _, delay := range []time.Duration{2 * time.Millisecond, 5 * time.Millisecond, 10 * time.Millisecond, 15 * time.Millisecond} {
		time.Sleep(delay)
		procName, pid = GetProcessByPort(isTCP, port)
		if procName != "" {
			return procName, pid
		}
	}
	return "", 0
}

func (b *BridgeDialer) DialContext(ctx context.Context, metadata *M.Metadata) (net.Conn, error) {
	dstAddr := metadata.DestinationAddress()
	isTCP := metadata.Network == M.TCP

	procName, pid := getProcessWithRetry(isTCP, metadata.SrcPort)
	matchedRule := b.matchRule(procName)
	if matchedRule != nil {
		b.logger.Info("Bridge routing process via proxy",
			"process", procName,
			"pid", pid,
			"rule", matchedRule.Pattern,
			"target", matchedRule.ProxyTarget,
			"dst", dstAddr,
		)
		px := b.getProxyForRule(matchedRule)
		if px != nil {
			return px.DialContext(ctx, metadata)
		}
	} else {
		b.logger.Debug("Bridge routing TCP direct",
			"process", procName,
			"pid", pid,
			"srcPort", metadata.SrcPort,
			"dst", dstAddr,
		)
	}

	return b.directProxy.DialContext(ctx, metadata)
}

func (b *BridgeDialer) DialUDP(metadata *M.Metadata) (net.PacketConn, error) {
	dstAddr := metadata.DestinationAddress()

	procName, pid := getProcessWithRetry(false, metadata.SrcPort)
	matchedRule := b.matchRule(procName)
	if matchedRule != nil {
		b.logger.Info("Bridge routing UDP via proxy",
			"process", procName,
			"pid", pid,
			"rule", matchedRule.Pattern,
			"target", matchedRule.ProxyTarget,
			"dst", dstAddr,
		)
		px := b.getProxyForRule(matchedRule)
		if px != nil {
			return px.DialUDP(metadata)
		}
	} else {
		b.logger.Debug("Bridge routing UDP direct",
			"process", procName,
			"pid", pid,
			"srcPort", metadata.SrcPort,
			"dst", dstAddr,
		)
	}

	return b.directProxy.DialUDP(metadata)
}

func (b *BridgeDialer) matchRule(procName string) *BridgeRule {
	if procName == "" || b.rulesGetter == nil {
		return nil
	}
	var disabledGroups map[string]bool
	if b.groupsGetter != nil {
		for _, g := range b.groupsGetter() {
			if !g.Enabled {
				if disabledGroups == nil {
					disabledGroups = make(map[string]bool)
				}
				disabledGroups[g.ID] = true
			}
		}
	}

	rules := b.rulesGetter()
	for i := range rules {
		if !rules[i].Enabled {
			continue
		}
		if rules[i].GroupID != "" && disabledGroups != nil && disabledGroups[rules[i].GroupID] {
			continue
		}
		if rules[i].Matches(procName) {
			return &rules[i]
		}
	}
	return nil
}

func (b *BridgeDialer) getProxyForRule(rule *BridgeRule) proxy.Proxy {
	target := strings.TrimSpace(rule.ProxyTarget)
	if target == "" || strings.Contains(target, "10808") {
		if b.defaultProxy != nil {
			return b.defaultProxy
		}
	}

	b.mu.RLock()
	px, ok := b.customProxies[target]
	b.mu.RUnlock()
	if ok {
		return px
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if px, ok := b.customProxies[target]; ok {
		return px
	}

	parsedAddr, pType, err := parseTargetURL(target, rule.ProxyType)
	if err != nil {
		return b.defaultProxy
	}

	var newPx proxy.Proxy
	if pType == "http" {
		newPx, err = proxy.NewHTTP(parsedAddr, "", "")
	} else {
		newPx, err = newSocks5Proxy(parsedAddr, "", "")
	}
	if err != nil {
		return b.defaultProxy
	}
	b.customProxies[target] = newPx
	return newPx
}

func parseProxyHostPort(raw string) (string, error) {
	if !strings.Contains(raw, "://") {
		raw = "socks5://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	host := u.Host
	if host == "" {
		host = u.Path
	}
	return host, nil
}

func parseTargetURL(raw, pType string) (string, string, error) {
	resolvedType := strings.ToLower(pType)
	if resolvedType == "" {
		resolvedType = "socks5"
	}

	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		resolvedType = "http"
	} else if strings.HasPrefix(trimmed, "socks5://") || strings.HasPrefix(trimmed, "socks://") {
		resolvedType = "socks5"
	}

	hostPort, err := parseProxyHostPort(trimmed)
	if err != nil {
		return "", "", fmt.Errorf("invalid proxy target %q: %w", raw, err)
	}

	return hostPort, resolvedType, nil
}
