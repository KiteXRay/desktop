package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/energye/systray"
	"github.com/jackpal/gateway"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	tproxy "github.com/xjasonlyu/tun2socks/v2/proxy"
	socks5proxy "golang.org/x/net/proxy"

	"github.com/goxray/core/awg"
	"github.com/goxray/core/client"
	"github.com/goxray/core/wireguard"
	"github.com/KiteXRay/desktop/internal/appscan"
	"github.com/KiteXRay/desktop/internal/bridge"
	"github.com/KiteXRay/desktop/internal/connlist"
	"github.com/KiteXRay/desktop/internal/osspecific/clean"
	"github.com/KiteXRay/desktop/internal/osspecific/hotkey"
	"github.com/KiteXRay/desktop/internal/osspecific/networkready"
	"github.com/KiteXRay/desktop/internal/osspecific/proxy"
	"github.com/KiteXRay/desktop/internal/osspecific/root"
	"github.com/KiteXRay/desktop/internal/sleepwatch"
	"github.com/KiteXRay/desktop/internal/subscription"
	"github.com/KiteXRay/desktop/internal/updater"
	xray3 "github.com/lilendian0x00/xray-knife/v3/pkg/xray"
)

type HotkeySettingsDTO struct {
	Enabled       bool   `json:"enabled"`
	ToggleWindow  string `json:"toggleWindow"`
	ToggleConnect string `json:"toggleConnect"`
}

type ProxyEndpointsDTO struct {

	Socks5Host string `json:"socks5Host"`
	Socks5Port int    `json:"socks5Port"`
	HTTPHost   string `json:"httpHost"`
	HTTPPort   int    `json:"httpPort"`
	Socks5URL  string `json:"socks5Url"`
	HTTPURL    string `json:"httpUrl"`
}

type TunnelSettingsDTO struct {
	DeviceIP string `json:"deviceIP"`
	DNS      string `json:"dns"`
}

type ConnectionDTO struct {
	ID             string            `json:"id"`
	SubscriptionID string            `json:"subscriptionId,omitempty"`
	Label          string            `json:"label"`
	Link           string            `json:"link"`
	Active       bool              `json:"active"`
	Address      string            `json:"address"`
	Port         string            `json:"port"`
	Protocol     string            `json:"protocol"`
	TLS          string            `json:"tls"`
	Flow         string            `json:"flow"`
	Network      string            `json:"network"`
	Security     string            `json:"security"`
	ConfigMap    map[string]string `json:"configMap"`
	BytesRead    int64             `json:"bytesRead"`
	BytesWritten int64             `json:"bytesWritten"`
	TotalBytes   int64             `json:"totalBytes"`
}

type StatsDTO struct {
	ID            string    `json:"id"`
	Active        bool      `json:"active"`
	BytesRead     int64     `json:"bytesRead"`
	BytesWritten  int64     `json:"bytesWritten"`
	TotalBytes    int64     `json:"totalBytes"`
	UploadSpeed   float64   `json:"uploadSpeed"`   // Current rate in KB/s
	DownloadSpeed float64   `json:"downloadSpeed"` // Current rate in KB/s
	ReadHistory   []float64 `json:"readHistory"`   // MB
	WriteHistory  []float64 `json:"writeHistory"`  // MB
}

type AppInfoDTO struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	RepoURL     string `json:"repoUrl"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	Description string `json:"description"`
}

type NetworkPrivilegesDTO struct {
	HasPrivileges bool   `json:"hasPrivileges"`
	OS            string `json:"os"`
	ExePath       string `json:"exePath"`
	Command       string `json:"command"`
	Error         string `json:"error,omitempty"`
}

type PingResultDTO struct {
	ID     string `json:"id"`
	PingMs int64  `json:"pingMs"`
}

type App struct {
	ctx            context.Context
	items          *connlist.Collection
	saveFile       *SaveFile
	activeIDMu     sync.RWMutex
	activeID       string
	connectMu      sync.Mutex
	onTrayUpdate   func()
	stopTicker     chan struct{}
	tunnelModeMu   sync.RWMutex
	tunnelMode     string
	systemProxyOn  bool
	sleepWatcher   *sleepwatch.Watcher
	isReconnecting atomic.Bool
	updateMu       sync.Mutex
	isUpdating     bool
	updateCancel   context.CancelFunc
	latestRelease  *updater.ReleaseInfo
	windowMu       sync.Mutex
	windowVisible  bool
	lastGeom       WindowGeometry
	hotkeyMgr      hotkey.Manager
	quitting       atomic.Bool
}

func NewApp() *App {
	items := connlist.New()
	saveFile := NewSaveFile()

	app := &App{
		items:         items,
		saveFile:      saveFile,
		stopTicker:    make(chan struct{}),
		tunnelMode:    saveFile.GetTunnelMode(),
		windowVisible: true,
	}

	if geom := saveFile.GetWindowGeometry(); geom != nil {
		app.lastGeom = *geom
	}

	saveFile.Load(items)
	app.tunnelMode = saveFile.GetTunnelMode()
	devIP, dns := saveFile.GetTunnelSettings()
	items.SetTunnelSettings(devIP, dns)
	items.SetBridgeDialerFactory(func(defaultSocksAddr string) tproxy.Dialer {
		return bridge.NewBridgeDialer(defaultSocksAddr, saveFile.GetBridgeRules, slog.Default(), saveFile.GetBridgeGroups)
	})
	items.SetConfigPath(saveFile.FilePath())

	items.OnChange(func() {
		saveFile.Update(items)
		if app.onTrayUpdate != nil {
			app.onTrayUpdate()
		}
		if app.ctx != nil {
			wruntime.EventsEmit(app.ctx, "connections:changed", app.GetConnections())
		}
	})

	return app
}

func (a *App) ActiveID() string {
	a.activeIDMu.RLock()
	defer a.activeIDMu.RUnlock()
	return a.activeID
}

func (a *App) SetActiveID(id string) {
	a.activeIDMu.Lock()
	defer a.activeIDMu.Unlock()
	a.activeID = id
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.windowVisible = true
	a.hotkeyMgr = hotkey.NewManager()
	a.setupHotkeys()
	_ = clean.ClearStuckNetwork()
	go a.startStatsTicker()
	a.startSleepWatcher()
	go a.startHealthWatchdog()

	// Automatically update all subscriptions in the background on startup
	go func() {
		time.Sleep(2 * time.Second)
		a.UpdateAllSubscriptions()
	}()
}

func (a *App) shutdown(ctx context.Context) {
	a.quitting.Store(true)
	if a.hotkeyMgr != nil {
		_ = a.hotkeyMgr.Close()
	}
	if a.sleepWatcher != nil {
		a.sleepWatcher.Stop()
	}
	select {
	case <-a.stopTicker:
	default:
		close(a.stopTicker)
	}
	_ = a.Disconnect()
}

func (a *App) startStatsTicker() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	flushTicks := 0
	geoTicks := 0

	for {
		select {
		case <-a.stopTicker:
			return
		case <-ticker.C:
			geoTicks++
			if geoTicks >= 2 {
				geoTicks = 0
				a.checkWindowGeometryChanged()
			}

			actID := a.ActiveID()
			if actID != "" {
				if a.ctx != nil {
					stats := a.GetStats(actID)
					wruntime.EventsEmit(a.ctx, "stats:tick", stats)
				}
				flushTicks++
				if flushTicks >= 15 {
					flushTicks = 0
					a.saveFile.Update(a.items)
				}
			}
		}
	}
}


func (a *App) GetConnections() []ConnectionDTO {
	allItems := a.items.All()
	dtos := make([]ConnectionDTO, len(allItems))
	activeID := a.ActiveID()

	for i, item := range allItems {
		cfg := item.XRayConfig()
		bytesRead := item.BytesRead()
		bytesWritten := item.BytesWritten()
		dtos[i] = ConnectionDTO{
			ID:             item.ID(),
			SubscriptionID: item.SubscriptionID(),
			Label:          item.Label(),
			Link:           item.Link(),
			Active:       item.Active() || (activeID != "" && activeID == item.ID()),
			Address:      cfg["Address"],
			Port:         cfg["Port"],
			Protocol:     cfg["Protocol"],
			TLS:          cfg["TLS"],
			Flow:         cfg["Flow"],
			Network:      cfg["Network"],
			Security:     cfg["Security"],
			ConfigMap:    cfg,
			BytesRead:    bytesRead,
			BytesWritten: bytesWritten,
			TotalBytes:   bytesRead + bytesWritten,
		}
	}

	return dtos
}

func (a *App) AddConnection(label, link string) (*ConnectionDTO, error) {
	label = strings.TrimSpace(label)
	link = strings.TrimSpace(link)

	// In case caller swapped label and link
	if wireguard.IsConfContent(label) || strings.Contains(label, "://") {
		if !wireguard.IsConfContent(link) && !strings.Contains(link, "://") {
			label, link = link, label
		}
	}

	if link == "" {
		return nil, errors.New("link cannot be empty")
	}

	if wireguard.IsConfContent(link) {
		cfg, err := wireguard.ParseConf(link)
		if err != nil {
			return nil, fmt.Errorf("invalid wireguard config: %w", err)
		}
		if label == "" {
			label = "Server"
		}
		link = cfg.ToURI(label)
	}

	if label == "" {
		if idx := strings.Index(link, "#"); idx != -1 && idx+1 < len(link) {
			if unescaped, err := url.QueryUnescape(link[idx+1:]); err == nil && unescaped != "" {
				label = unescaped
			} else {
				label = link[idx+1:]
			}
		}
		if label == "" {
			label = "Server"
		}
	}

	if wireguard.IsAWGLink(link) {
		if _, _, err := wireguard.ParseLink(link); err != nil {
			return nil, fmt.Errorf("parse awg protocol: %w", err)
		}
	} else {
		proto, err := (&xray3.Core{}).CreateProtocol(link)
		if err != nil {
			return nil, fmt.Errorf("create xray protocol: %w", err)
		}
		if err := proto.Parse(); err != nil {
			return nil, fmt.Errorf("parse xray protocol: %w", err)
		}
	}

	if err := a.items.AddItem(label, link); err != nil {
		return nil, err
	}

	all := a.GetConnections()
	if len(all) > 0 {
		return &all[len(all)-1], nil
	}
	return nil, nil
}

func (a *App) ImportWireguardConfig(content, label string) (*ConnectionDTO, error) {
	cfg, err := wireguard.ParseConf(content)
	if err != nil {
		return nil, fmt.Errorf("parse wireguard config: %w", err)
	}
	link := cfg.ToURI(label)
	return a.AddConnection(label, link)
}

func (a *App) UpdateConnection(id string, label, link string) error {
	if label == "" || link == "" {
		return errors.New("label and link cannot be empty")
	}

	if wireguard.IsConfContent(link) {
		if cfg, err := wireguard.ParseConf(link); err == nil {
			link = cfg.ToURI(label)
		}
	}

	item := a.items.FindByID(id)
	if item == nil {
		return errors.New("item not found")
	}

	if item.Active() {
		return errors.New("disconnect before editing")
	}

	return item.Update(link, label)
}

func (a *App) DeleteConnection(id string) error {
	item := a.items.FindByID(id)
	if item == nil {
		return errors.New("item not found")
	}

	if item.Active() {
		return errors.New("disconnect before deleting")
	}

	if a.ActiveID() == id {
		a.SetActiveID("")
	}

	a.items.RemoveItem(item)
	return nil
}

func (a *App) SwapConnections(id1, id2 int) error {
	allItems := a.items.All()
	if id1 < 0 || id1 >= len(allItems) || id2 < 0 || id2 >= len(allItems) {
		return errors.New("invalid item indices")
	}

	return a.items.SwapItems(allItems[id1], allItems[id2])
}

func (a *App) ReorderConnections(from, to int) error {
	return a.items.MoveItem(from, to)
}

func (a *App) Connect(id string) error {
	a.isReconnecting.Store(false)
	currentActive := a.ActiveID()
	// If already active on this one, disconnect
	if currentActive == id {
		return a.Disconnect()
	}

	// Pre-check network privileges for TUN-based modes to prevent crashes and alert the user
	currentMode := a.GetTunnelMode()
	if currentMode != "proxy" {
		if has, err := root.HasNetworkPrivileges(); !has {
			_, fixCmd := root.GetPrivilegeFixCommand()
			errMsg := fmt.Sprintf("Missing network privileges for %s mode. Please run: %s", currentMode, fixCmd)
			if err != nil {
				errMsg = fmt.Sprintf("Missing network privileges (%s). Run: %s", err.Error(), fixCmd)
			}
			slog.Error("cannot connect due to missing network privileges", "error", err, "command", fixCmd)
			if a.ctx != nil {
				wruntime.EventsEmit(a.ctx, "network:privileges_required", map[string]any{
					"error":   errMsg,
					"command": fixCmd,
				})
				wruntime.EventsEmit(a.ctx, "connection:status", map[string]any{
					"status":  "error",
					"id":      id,
					"error":   errMsg,
					"command": fixCmd,
				})
			}
			return errors.New(errMsg)
		}
	}

	return a.connectInternal(id)
}

func (a *App) connectInternal(id string) error {
	a.connectMu.Lock()
	defer a.connectMu.Unlock()

	target := a.items.FindByID(id)
	if target == nil {
		return errors.New("connection not found")
	}

	currentActive := a.ActiveID()
	// Disconnect existing active
	if currentActive != "" && currentActive != id {
		if prev := a.items.FindByID(currentActive); prev != nil {
			_ = prev.Disconnect()
			prev.SetActive(false)
		}
	}

	currentMode := a.GetTunnelMode()
	var tMode client.TunnelMode
	switch currentMode {
	case "proxy":
		tMode = client.TunnelModeProxy
	case "bridge":
		tMode = client.TunnelModeBridge
	case "per_app":
		tMode = client.TunnelModePerApp
	default:
		tMode = client.TunnelModeTunnel
	}
	if tMode == client.TunnelModeBridge {
		target.SetBridgeDialerFactory(func(defaultSocksAddr string) tproxy.Dialer {
			return bridge.NewBridgeDialer(defaultSocksAddr, a.saveFile.GetBridgeRules, slog.Default(), a.saveFile.GetBridgeGroups)
		})
	}
	target.SetConfigPath(a.saveFile.FilePath())
	devIP, dns := a.saveFile.GetTunnelSettings()
	target.SetTunnelSettings(devIP, dns)
	target.SetBypassIPs(a.collectAllProfileIPs())
	if err := target.ConnectWithMode(tMode); err != nil {
		slog.Error("failed to connect", "error", err)
		a.SetActiveID("")
		if a.onTrayUpdate != nil {
			a.onTrayUpdate()
		}
		if a.ctx != nil {
			wruntime.EventsEmit(a.ctx, "connection:status", map[string]any{
				"status": "error",
				"id":     id,
				"error":  err.Error(),
			})
		}
		return err
	}

	a.SetActiveID(id)
	target.SetActive(true)

	if currentMode == "proxy" {
		_ = proxy.SetSystemProxy(true, "127.0.0.1", client.DefaultHTTPPort, client.DefaultSocksPort)
		a.systemProxyOn = true
	} else if a.systemProxyOn {
		_ = proxy.SetSystemProxy(false, "127.0.0.1", client.DefaultHTTPPort, client.DefaultSocksPort)
		a.systemProxyOn = false
	}

	if a.onTrayUpdate != nil {
		a.onTrayUpdate()
	}
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "connections:changed", a.GetConnections())
		wruntime.EventsEmit(a.ctx, "proxy:status", a.systemProxyOn)
		wruntime.EventsEmit(a.ctx, "connection:status", map[string]any{
			"status": "connected",
			"id":     id,
			"mode":   a.GetTunnelMode(),
		})
	}

	go func() {
		time.Sleep(150 * time.Millisecond)
		a.PingConnection(id)
	}()

	return nil
}

func (a *App) Disconnect() error {
	a.isReconnecting.Store(false)
	a.connectMu.Lock()
	defer a.connectMu.Unlock()

	currentActive := a.ActiveID()
	if currentActive != "" {
		if item := a.items.FindByID(currentActive); item != nil {
			if err := item.Disconnect(); err != nil {
				slog.Error("error disconnecting", "error", err)
			}
			item.SetActive(false)
		}
	}

	if a.systemProxyOn {
		_ = proxy.SetSystemProxy(false, "127.0.0.1", client.DefaultHTTPPort, client.DefaultSocksPort)
		a.systemProxyOn = false
	}

	if currentActive == "" {
		return nil
	}

	prevID := currentActive
	a.SetActiveID("")

	if a.onTrayUpdate != nil {
		a.onTrayUpdate()
	}
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "connections:changed", a.GetConnections())
		wruntime.EventsEmit(a.ctx, "connection:status", map[string]any{
			"status": "disconnected",
			"id":     prevID,
		})
	}

	return nil
}

func (a *App) ClearStuckTun() error {
	a.connectMu.Lock()
	defer a.connectMu.Unlock()

	for _, item := range a.items.All() {
		if item != nil {
			_ = item.Disconnect()
			item.SetActive(false)
		}
	}
	a.SetActiveID("")

	err := clean.ClearStuckNetwork()

	if a.onTrayUpdate != nil {
		a.onTrayUpdate()
	}
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "connection:status", map[string]any{
			"status": "disconnected",
		})
		wruntime.EventsEmit(a.ctx, "connections:changed", a.GetConnections())
	}

	return err
}

func (a *App) Quit() {
	slog.Info("Quit requested, terminating application...")
	a.SaveWindowGeometry()
	a.quitting.Store(true)
	select {
	case <-a.stopTicker:
	default:
		close(a.stopTicker)
	}

	go func() {
		// Set a deadline for graceful cleanup
		done := make(chan struct{})
		go func() {
			_ = a.Disconnect()
			_ = clean.ClearStuckNetwork()
			systray.Quit()
			close(done)
		}()

		select {
		case <-done:
			slog.Info("Graceful cleanup completed")
		case <-time.After(1500 * time.Millisecond):
			slog.Warn("Cleanup timed out, forcing exit")
		}

		if a.ctx != nil {
			wruntime.Quit(a.ctx)
		}
		time.Sleep(50 * time.Millisecond)
		os.Exit(0)
	}()
}

func (a *App) GetStats(id string) StatsDTO {
	item := a.items.FindByID(id)
	if item == nil {
		return StatsDTO{ID: id}
	}

	readHist := item.Read()
	writeHist := item.Written()

	var upSpeed, downSpeed float64
	if len(readHist) > 0 {
		upSpeed = readHist[len(readHist)-1] * 1024 // convert MB/s to KB/s
	}
	if len(writeHist) > 0 {
		downSpeed = writeHist[len(writeHist)-1] * 1024
	}

	bytesRead := item.BytesRead()
	bytesWritten := item.BytesWritten()

	return StatsDTO{
		ID:            id,
		Active:        item.Active(),
		BytesRead:     bytesRead,
		BytesWritten:  bytesWritten,
		TotalBytes:    bytesRead + bytesWritten,
		UploadSpeed:   upSpeed,
		DownloadSpeed: downSpeed,
		ReadHistory:   readHist,
		WriteHistory:  writeHist,
	}
}

func (a *App) ResetTraffic(id string) error {
	item := a.items.FindByID(id)
	if item == nil {
		return errors.New("connection not found")
	}

	item.ResetTraffic()
	a.saveFile.Update(a.items)
	return nil
}

var appVersion = "1.4.2"

func (a *App) GetAppInfo() AppInfoDTO {
	return AppInfoDTO{
		Name:        "Kite",
		Version:     appVersion,
		RepoURL:     "https://github.com/KiteXRay/desktop",
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		Description: "Fast, minimal, and transparent desktop VPN client.",
	}
}

func (a *App) CheckNetworkPrivileges() NetworkPrivilegesDTO {
	currentMode := a.GetTunnelMode()
	var has bool
	var err error
	if currentMode == "proxy" {
		has = true
	} else {
		has, err = root.HasNetworkPrivileges()
	}
	exePath, cmd := root.GetPrivilegeFixCommand()
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	return NetworkPrivilegesDTO{
		HasPrivileges: has,
		OS:            runtime.GOOS,
		ExePath:       exePath,
		Command:       cmd,
		Error:         errStr,
	}
}

func (a *App) GrantNetworkPrivileges() (bool, error) {
	if err := root.GrantPrivilegesAndRestart(); err != nil {
		return false, err
	}
	return true, nil
}

func (a *App) OpenURL(targetURL string) {
	if a.ctx != nil {
		wruntime.BrowserOpenURL(a.ctx, targetURL)
	}
}

func (a *App) GetClipboardText() (string, error) {
	if a.ctx == nil {
		return "", errors.New("app context not ready")
	}
	return wruntime.ClipboardGetText(a.ctx)
}

func pingRoutedConnection(link string, timeout time.Duration) (latency int64) {
	if link == "" {
		return -1
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic in pingRoutedConnection", "panic", r)
			latency = -1
		}
	}()

	if wireguard.IsAWGLink(link) {
		awgCfg, _, err := wireguard.ParseLink(link)
		if err != nil {
			return -1
		}
		return awg.Ping(awgCfg, timeout)
	}

	coreService := xray3.NewXrayService(false, true)
	proto, err := coreService.CreateProtocol(link)
	if err != nil {
		return -1
	}

	if err := proto.Parse(); err != nil {
		return -1
	}

	client, instance, err := coreService.MakeHttpClient(proto, timeout)
	if err != nil {
		return -1
	}
	defer instance.Close()

	if tr, ok := client.Transport.(*http.Transport); ok {
		tr.DisableKeepAlives = false
	}

	var connStart, connDone time.Time
	trace := &httptrace.ClientTrace{
		ConnectStart: func(network, addr string) {
			connStart = time.Now()
		},
		ConnectDone: func(network, addr string, err error) {
			connDone = time.Now()
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	targetURL := "https://www.google.com/generate_204"

	// 1. Establish connection through VPN tunnel to verify credentials and routing
	req1, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), "HEAD", targetURL, nil)
	if err != nil {
		return -1
	}
	resp1, err := client.Do(req1)
	if err != nil {
		return -1
	}
	_ = resp1.Body.Close()

	dialMs := connDone.Sub(connStart).Milliseconds()

	// 2. Measure pure 1-RTT latency over the established tunnel (without core spawn / cold handshake overhead)
	t0 := time.Now()
	req2, err := http.NewRequestWithContext(ctx, "HEAD", targetURL, nil)
	if err == nil {
		resp2, err2 := client.Do(req2)
		if err2 == nil {
			_ = resp2.Body.Close()
			warmMs := time.Since(t0).Milliseconds()
			if warmMs > 0 {
				return warmMs
			}
		}
	}

	// Fallback to pure dial latency if warm request fails
	if dialMs > 0 {
		return dialMs
	}
	return 1
}

func extractServerHost(link string) (string, error) {
	if wireguard.IsAWGLink(link) {
		cfg, _, err := wireguard.ParseLink(link)
		if err != nil {
			return "", err
		}
		host := cfg.Endpoint
		if h, _, err := net.SplitHostPort(cfg.Endpoint); err == nil {
			host = h
		}
		return host, nil
	}
	proto, err := (&xray3.Core{}).CreateProtocol(link)
	if err != nil {
		return "", err
	}
	if err := proto.Parse(); err != nil {
		return "", err
	}
	gen := proto.ConvertToGeneralConfig()
	if gen.Address != "" {
		return gen.Address, nil
	}
	return "", errors.New("no address found in link")
}

func resolveServerIP(host string) (string, error) {
	if ip := net.ParseIP(host); ip != nil {
		return ip.String(), nil
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return "", fmt.Errorf("resolve %s: %w", host, err)
	}
	for _, ip := range ips {
		if ipv4 := ip.To4(); ipv4 != nil {
			return ipv4.String(), nil
		}
	}
	return ips[0].String(), nil
}

func (a *App) collectAllProfileIPs() []string {
	seen := make(map[string]bool)
	var ips []string
	for _, itm := range a.items.All() {
		if itm == nil {
			continue
		}
		host, err := extractServerHost(itm.Link())
		if err != nil {
			continue
		}
		ip, err := resolveServerIP(host)
		if err != nil {
			continue
		}
		if !seen[ip] {
			seen[ip] = true
			ips = append(ips, ip)
		}
	}
	return ips
}

func (a *App) pingActiveConnection(timeout time.Duration) int64 {
	if a.ActiveID() == "" {
		return -1
	}
	targets := []string{"https://www.google.com/generate_204", "http://cp.cloudflare.com/generate_204"}

	// 1. Try dialing through SOCKS5 proxy (127.0.0.1:10808)
	if dialer, err := socks5proxy.SOCKS5("tcp", fmt.Sprintf("127.0.0.1:%d", client.DefaultSocksPort), nil, &net.Dialer{Timeout: timeout}); err == nil {
		if cd, ok := dialer.(socks5proxy.ContextDialer); ok {
			httpClient := &http.Client{
				Transport: &http.Transport{
					DialContext:       cd.DialContext,
					DisableKeepAlives: false,
				},
				Timeout: timeout,
			}
			for _, target := range targets {
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				req, err := http.NewRequestWithContext(ctx, "HEAD", target, nil)
				if err == nil {
					t0 := time.Now()
					resp, err := httpClient.Do(req)
					cancel()
					if err == nil {
						_ = resp.Body.Close()
						firstMs := time.Since(t0).Milliseconds()

						// Warm 1-RTT measurement over established tunnel connection
						ctxWarm, cancelWarm := context.WithTimeout(context.Background(), timeout)
						reqWarm, errWarm := http.NewRequestWithContext(ctxWarm, "HEAD", target, nil)
						if errWarm == nil {
							tWarm := time.Now()
							respWarm, errWarm := httpClient.Do(reqWarm)
							cancelWarm()
							if errWarm == nil {
								_ = respWarm.Body.Close()
								warmMs := time.Since(tWarm).Milliseconds()
								if warmMs > 0 {
									return warmMs
								}
							}
						} else {
							cancelWarm()
						}

						if firstMs > 0 {
							return firstMs
						}
						return 1
					}
				} else {
					cancel()
				}
			}
		}
	}

	// 2. Direct HTTP dial fallback (works when system default route points to TUN)
	directClient := &http.Client{
		Timeout: timeout,
	}
	for _, target := range targets {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		req, err := http.NewRequestWithContext(ctx, "HEAD", target, nil)
		if err == nil {
			t0 := time.Now()
			resp, err := directClient.Do(req)
			cancel()
			if err == nil {
				_ = resp.Body.Close()
				firstMs := time.Since(t0).Milliseconds()
				if firstMs > 0 {
					return firstMs
				}
				return 1
			}
		} else {
			cancel()
		}
	}

	return -1
}

func (a *App) PingConnection(id string) int64 {
	item := a.items.FindByID(id)
	if item == nil {
		return -1
	}
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "ping:start", id)
	}

	var res int64
	if item.Active() {
		res = a.pingActiveConnection(2500 * time.Millisecond)
	} else {
		res = pingRoutedConnection(item.Link(), 2500*time.Millisecond)
	}

	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "ping:result", PingResultDTO{
			ID:     id,
			PingMs: res,
		})
	}
	return res
}

func (a *App) PingAll() map[string]int64 {
	allItems := a.items.All()
	results := make(map[string]int64)

	// Sequential execution (1 connection at a time) to prevent:
	// 1. High CPU usage from spawning multiple in-memory Xray cores concurrently
	// 2. gRPC ENHANCE_YOUR_CALM (too_many_pings) error from parallel streams to the same server
	for _, itm := range allItems {
		if itm == nil {
			continue
		}
		id := itm.ID()
		link := itm.Link()

		if a.ctx != nil {
			wruntime.EventsEmit(a.ctx, "ping:start", id)
		}

		var latency int64
		if itm.Active() {
			latency = a.pingActiveConnection(2500 * time.Millisecond)
		} else {
			latency = pingRoutedConnection(link, 2500*time.Millisecond)
		}
		results[id] = latency

		if a.ctx != nil {
			wruntime.EventsEmit(a.ctx, "ping:result", PingResultDTO{
				ID:     id,
				PingMs: latency,
			})
		}
	}

	return results
}

func (a *App) CheckForUpdate() (*updater.ReleaseInfo, error) {
	appInfo := a.GetAppInfo()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	info, err := updater.CheckForUpdate(ctx, appInfo.RepoURL, appInfo.Version)
	if err == nil && info != nil {
		a.updateMu.Lock()
		a.latestRelease = info
		a.updateMu.Unlock()
	}
	return info, err
}

func (a *App) CancelUpdate() error {
	a.updateMu.Lock()
	defer a.updateMu.Unlock()
	if a.updateCancel != nil {
		a.updateCancel()
		a.updateCancel = nil
	}
	return nil
}

func (a *App) InstallUpdate(assetURL, releaseURL string) error {
	a.updateMu.Lock()
	if a.isUpdating {
		a.updateMu.Unlock()
		return errors.New("update is already in progress")
	}
	a.isUpdating = true
	a.updateMu.Unlock()

	go func() {
		destPath := ""
		defer func() {
			a.updateMu.Lock()
			a.isUpdating = false
			a.updateCancel = nil
			a.updateMu.Unlock()
			if destPath != "" {
				_ = os.Remove(destPath)
			}
		}()

		if assetURL == "" {
			assetURL = releaseURL
		}

		// If assetURL is not a downloadable file (or is the release webpage), open browser
		if strings.HasPrefix(assetURL, "https://github.com/") && strings.Contains(assetURL, "/releases/tag/") {
			if a.ctx != nil {
				wruntime.BrowserOpenURL(a.ctx, releaseURL)
			}
			return
		}

		baseName := filepath.Base(assetURL)
		if idx := strings.Index(baseName, "?"); idx != -1 {
			baseName = baseName[:idx]
		}
		if baseName == "" || baseName == "." {
			baseName = "kite-update"
		}
		destPath = filepath.Join(os.TempDir(), fmt.Sprintf("kite_%d_%s", time.Now().Unix(), baseName))

		emitProgress := func(status string, pct float64, downloaded, total int64, errMsg string) {
			if a.ctx != nil {
				wruntime.EventsEmit(a.ctx, "update:progress", map[string]any{
					"status":     status,
					"percentage": pct,
					"downloaded": downloaded,
					"total":      total,
					"error":      errMsg,
				})
			}
		}

		emitProgress("downloading", 0, 0, 0, "")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		a.updateMu.Lock()
		a.updateCancel = cancel
		expectedSHA := ""
		if a.latestRelease != nil && (a.latestRelease.AssetURL == assetURL || a.latestRelease.AssetName == baseName) {
			expectedSHA = a.latestRelease.ExpectedSHA
		}
		if expectedSHA == "" {
			if sha, errFetch := updater.FetchAssetChecksum(ctx, a.GetAppInfo().RepoURL, baseName); errFetch == nil && sha != "" {
				expectedSHA = sha
			}
		}
		a.updateMu.Unlock()
		defer cancel()

		var lastEmit time.Time
		err := updater.DownloadFile(ctx, assetURL, destPath, func(dl, tot int64) {
			now := time.Now()
			if now.Sub(lastEmit) > 100*time.Millisecond || dl == tot {
				lastEmit = now
				pct := float64(0)
				if tot > 0 {
					pct = float64(dl) / float64(tot) * 100
				}
				emitProgress("downloading", pct, dl, tot, "")
			}
		})

		if err != nil {
			if errors.Is(err, context.Canceled) {
				slog.Info("Update download cancelled by user")
				emitProgress("cancelled", 0, 0, 0, "Update cancelled")
				return
			}
			slog.Error("Failed to download update", "error", err)
			emitProgress("error", 0, 0, 0, err.Error())
			return
		}

		// Verify SHA256 checksum if available
		if expectedSHA != "" {
			if errChk := updater.VerifyFileSHA256(destPath, expectedSHA); errChk != nil {
				slog.Error("Failed checksum verification for update payload", "error", errChk)
				emitProgress("error", 0, 0, 0, fmt.Sprintf("Security check failed: %v", errChk))
				return
			}
			slog.Info("Checksum verification passed for update payload", "sha256", expectedSHA)
		}

		emitProgress("applying", 100, 0, 0, "")

		err = updater.ApplyDownloadedUpdate(destPath, releaseURL, func() {
			_ = a.Disconnect()
			_ = clean.ClearStuckNetwork()
		})

		if err != nil {
			slog.Error("Failed to apply update", "error", err)
			emitProgress("error", 0, 0, 0, err.Error())
			return
		}

		emitProgress("completed", 100, 0, 0, "")
	}()

	return nil
}

func (a *App) ParseLinkPreview(link string) (map[string]string, error) {
	if wireguard.IsAWGLink(link) {
		cfg, remark, err := wireguard.ParseLink(link)
		if err != nil {
			return nil, fmt.Errorf("parse awg link: %w", err)
		}
		return cfg.ToMap(remark), nil
	}

	proto, err := (&xray3.Core{}).CreateProtocol(link)
	if err != nil {
		return nil, fmt.Errorf("create protocol: %w", err)
	}
	if err := proto.Parse(); err != nil {
		return nil, fmt.Errorf("parse protocol: %w", err)
	}

	gen := proto.ConvertToGeneralConfig()
	result := map[string]string{
		"Protocol":       gen.Protocol,
		"Address":        gen.Address,
		"Port":           gen.Port,
		"Security":       gen.Security,
		"TLS":            gen.TLS,
		"Network":        gen.Network,
		"Remark":         gen.Remark,
		"ID":             gen.ID,
		"Path":           gen.Path,
		"Host":           gen.Host,
		"SNI":            gen.SNI,
		"ALPN":           gen.ALPN,
		"TlsFingerprint": gen.TlsFingerprint,
		"Authority":      gen.Authority,
		"ServiceName":    gen.ServiceName,
		"Mode":           gen.Mode,
		"Type":           gen.Type,
	}

	// Extract protocol-specific fields via JSON marshaling
	if b, err := json.Marshal(proto); err == nil {
		var rawMap map[string]any
		if err := json.Unmarshal(b, &rawMap); err == nil {
			for k, v := range rawMap {
				strVal := fmt.Sprintf("%v", v)
				if strVal == "" || strVal == "<nil>" {
					continue
				}
				lowerK := strings.ToLower(k)
				switch lowerK {
				case "add":
					if result["Address"] == "" {
						result["Address"] = strVal
					}
				case "ps":
					if result["Remark"] == "" {
						result["Remark"] = strVal
					}
				case "fp":
					result["TlsFingerprint"] = strVal
				case "pbk":
					result["Pbk"] = strVal
				case "sid":
					result["Sid"] = strVal
				case "spx":
					result["Spx"] = strVal
				case "flow":
					result["Flow"] = strVal
				case "headertype":
					result["HeaderType"] = strVal
				case "encryption":
					result["Encryption"] = strVal
				case "net":
					if result["Network"] == "" {
						result["Network"] = strVal
					}
				default:
					if len(k) > 0 {
						capKey := strings.ToUpper(k[:1]) + k[1:]
						if _, exists := result[capKey]; !exists {
							result[capKey] = strVal
						}
					}
				}
			}
		}
	}

	return result, nil
}

func (a *App) BuildLinkFromConfig(cfg map[string]string) (string, error) {
	return buildLinkFromMap(cfg)
}

func (a *App) GetTunnelMode() string {
	a.tunnelModeMu.RLock()
	defer a.tunnelModeMu.RUnlock()
	switch a.tunnelMode {
	case "proxy":
		return "proxy"
	case "bridge", "per_app":
		return "bridge"
	default:
		return "tunnel"
	}
}

func (a *App) SetTunnelMode(mode string) error {
	a.connectMu.Lock()
	defer a.connectMu.Unlock()

	switch mode {
	case "proxy":
		mode = "proxy"
	case "bridge", "per_app":
		mode = "bridge"
	default:
		mode = "tunnel"
	}

	a.tunnelModeMu.Lock()
	if a.tunnelMode == mode {
		a.tunnelModeMu.Unlock()
		return nil
	}
	a.tunnelMode = mode
	a.tunnelModeMu.Unlock()

	a.saveFile.SetTunnelMode(mode)
	a.saveFile.Update(a.items)

	// If currently connected, reconnect with the new mode
	activeID := a.ActiveID()
	if activeID != "" {
		item := a.items.FindByID(activeID)
		if item != nil {
			_ = item.Disconnect()

			var tMode client.TunnelMode
			switch mode {
			case "proxy":
				tMode = client.TunnelModeProxy
			case "bridge":
				tMode = client.TunnelModeBridge
			default:
				tMode = client.TunnelModeTunnel
			}
			if tMode != client.TunnelModeProxy {
				if has, err := root.HasNetworkPrivileges(); !has {
					_, fixCmd := root.GetPrivilegeFixCommand()
					errMsg := fmt.Sprintf("Missing network privileges for %s mode. Please run: %s", mode, fixCmd)
					if err != nil {
						errMsg = fmt.Sprintf("Missing network privileges (%s). Run: %s", err.Error(), fixCmd)
					}
					slog.Error("cannot switch mode due to missing network privileges", "error", err, "command", fixCmd)
					item.SetActive(false)
					a.SetActiveID("")
					if a.onTrayUpdate != nil {
						a.onTrayUpdate()
					}
					if a.ctx != nil {
						wruntime.EventsEmit(a.ctx, "network:privileges_required", map[string]any{
							"error":   errMsg,
							"command": fixCmd,
						})
						wruntime.EventsEmit(a.ctx, "connection:status", map[string]any{
							"status":  "error",
							"id":      activeID,
							"error":   errMsg,
							"command": fixCmd,
						})
					}
					return errors.New(errMsg)
				}
			}
			if tMode == client.TunnelModeBridge {
				item.SetBridgeDialerFactory(func(defaultSocksAddr string) tproxy.Dialer {
					return bridge.NewBridgeDialer(defaultSocksAddr, a.saveFile.GetBridgeRules, slog.Default(), a.saveFile.GetBridgeGroups)
				})
			}
			item.SetConfigPath(a.saveFile.FilePath())
			if err := item.ConnectWithMode(tMode); err != nil {
				slog.Error("failed to reconnect with new mode", "error", err)
				item.SetActive(false)
				a.SetActiveID("")
				if a.onTrayUpdate != nil {
					a.onTrayUpdate()
				}
				if a.ctx != nil {
					wruntime.EventsEmit(a.ctx, "connection:status", map[string]any{
						"status": "error",
						"id":     activeID,
						"error":  err.Error(),
					})
				}
				return err
			}
			item.SetActive(true)

			if mode == "proxy" {
				_ = proxy.SetSystemProxy(true, "127.0.0.1", client.DefaultHTTPPort, client.DefaultSocksPort)
				a.systemProxyOn = true
			} else if a.systemProxyOn {
				_ = proxy.SetSystemProxy(false, "127.0.0.1", client.DefaultHTTPPort, client.DefaultSocksPort)
				a.systemProxyOn = false
			}

			if a.ctx != nil {
				wruntime.EventsEmit(a.ctx, "connections:changed", a.GetConnections())
				wruntime.EventsEmit(a.ctx, "proxy:status", a.systemProxyOn)
				wruntime.EventsEmit(a.ctx, "connection:status", map[string]any{
					"status": "connected",
					"id":     activeID,
					"mode":   a.GetTunnelMode(),
				})
			}
			go func() {
				time.Sleep(150 * time.Millisecond)
				a.PingConnection(activeID)
			}()
		}
	} else if a.systemProxyOn {
		_ = proxy.SetSystemProxy(false, "127.0.0.1", client.DefaultHTTPPort, client.DefaultSocksPort)
		a.systemProxyOn = false
	}

	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "proxy:status", a.systemProxyOn)
		wruntime.EventsEmit(a.ctx, "mode:changed", mode)
	}

	if a.onTrayUpdate != nil {
		a.onTrayUpdate()
	}

	return nil
}

func (a *App) GetProxyEndpoints() ProxyEndpointsDTO {
	return ProxyEndpointsDTO{
		Socks5Host: "127.0.0.1",
		Socks5Port: client.DefaultSocksPort,
		HTTPHost:   "127.0.0.1",
		HTTPPort:   client.DefaultHTTPPort,
		Socks5URL:  fmt.Sprintf("socks5://127.0.0.1:%d", client.DefaultSocksPort),
		HTTPURL:    fmt.Sprintf("http://127.0.0.1:%d", client.DefaultHTTPPort),
	}
}

func (a *App) SetSystemProxy(enabled bool) error {
	err := proxy.SetSystemProxy(enabled, "127.0.0.1", client.DefaultHTTPPort, client.DefaultSocksPort)
	if err == nil {
		a.systemProxyOn = enabled
	}
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "proxy:status", a.systemProxyOn)
	}
	return err
}

func (a *App) GetSystemProxyStatus() bool {
	return proxy.IsSystemProxyEnabled()
}

func (a *App) LaunchAppWithProxy(appName, targetPath string) error {
	return launchAppWithProxy(appName, targetPath)
}

func (a *App) GetInstalledApps() ([]appscan.AppInfo, error) {
	return appscan.GetInstalledApps()
}

func (a *App) SelectExecutableDialog() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("app context not ready")
	}

	filters := []wruntime.FileFilter{}
	if runtime.GOOS == "windows" {
		filters = append(filters, wruntime.FileFilter{
			DisplayName: "Executable Files (*.exe, *.bat, *.cmd)",
			Pattern:     "*.exe;*.bat;*.cmd",
		})
	}

	file, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:   "Select Application Executable to Tunnel",
		Filters: filters,
	})
	if err != nil {
		return "", err
	}
	return file, nil
}

func (a *App) LaunchAndRouteApp(connectionID string, exePath string) error {
	if exePath == "" {
		return fmt.Errorf("no executable specified")
	}

	// 1. Ensure connected in per_app mode
	if a.ActiveID() != connectionID {
		_ = a.SetTunnelMode("per_app")
		if err := a.Connect(connectionID); err != nil {
			return fmt.Errorf("connect to profile: %w", err)
		}
	} else if a.GetTunnelMode() != "per_app" {
		_ = a.SetTunnelMode("per_app")
	}

	// 2. Launch target executable configured with proxy
	return a.LaunchAppWithProxy("", exePath)
}

func (a *App) startSleepWatcher() {
	a.sleepWatcher = sleepwatch.New(sleepwatch.Config{
		Logger: slog.Default(),
		OnSleep: func() {
			slog.Info("System is preparing to sleep")
		},
		OnWake: func() {
			go a.handleSystemWakeUp()
		},
	})
	a.sleepWatcher.Start()
}

func (a *App) handleSystemWakeUp() {
	activeID := a.ActiveID()
	if activeID == "" {
		slog.Info("System woke up: no active VPN connection to restore")
		return
	}

	if a.isReconnecting.Swap(true) {
		slog.Info("Wake-up reconnect already in progress, skipping duplicate trigger")
		return
	}
	defer a.isReconnecting.Store(false)

	slog.Info("System woke up from sleep: restoring active VPN connection", "id", activeID)

	// 1. Notify frontend immediately
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "connection:status", map[string]any{
			"status":  "reconnecting",
			"id":      activeID,
			"mode":    a.GetTunnelMode(),
			"message": "Waking from sleep: Reconnecting secure VPN...",
		})
	}

	// 2. CRITICAL: Disconnect stale session & clean routing tables FIRST!
	// This unmounts tun0 and removes 0.0.0.0/1 so host networking and DNS work normally.
	a.connectMu.Lock()
	if target := a.items.FindByID(activeID); target != nil {
		_ = target.Disconnect()
		target.SetActive(false)
	}
	a.SetActiveID("")
	if a.systemProxyOn {
		_ = proxy.SetSystemProxy(false, "127.0.0.1", client.DefaultHTTPPort, client.DefaultSocksPort)
		a.systemProxyOn = false
	}
	_ = clean.ClearStuckNetwork()
	a.connectMu.Unlock()

	// 3. Wait for host physical network interface to acquire DHCP and gateway
	netCtx, netCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer netCancel()
	netReady := networkready.WaitUntilReady(netCtx)
	if !netReady {
		slog.Warn("Physical network wait timed out after wake-up, attempting reconnect")
	} else {
		slog.Info("Physical network ready after wake-up, re-establishing VPN connection")
	}

	// 4. Reconnect with retry (up to 4 attempts)
	var lastErr error
	for attempt := 1; attempt <= 4; attempt++ {
		if !a.isReconnecting.Load() {
			slog.Info("Reconnect aborted: user disconnected or switched profile")
			return
		}
		slog.Info("Reconnecting VPN session after sleep", "attempt", attempt, "id", activeID)
		err := a.connectInternal(activeID)
		if err == nil {
			// Verify that traffic actually passes through the tunnel to the server!
			time.Sleep(800 * time.Millisecond)
			if a.verifyTunnelConnectivity(3500 * time.Millisecond) {
				slog.Info("VPN session successfully restored and verified after wake-up", "id", activeID)
				return
			}
			slog.Warn("VPN connected locally but outbound tunnel verification probe failed, retrying...", "attempt", attempt)
			err = errors.New("outbound tunnel probe failed")
		}
		lastErr = err
		slog.Warn("Reconnect attempt failed", "attempt", attempt, "error", err)

		// On failure, ALWAYS ensure target is fully disconnected and network is clean before retrying!
		a.connectMu.Lock()
		if target := a.items.FindByID(activeID); target != nil {
			_ = target.Disconnect()
			target.SetActive(false)
		}
		a.SetActiveID("")
		if a.systemProxyOn {
			_ = proxy.SetSystemProxy(false, "127.0.0.1", client.DefaultHTTPPort, client.DefaultSocksPort)
			a.systemProxyOn = false
		}
		_ = clean.ClearStuckNetwork()
		a.connectMu.Unlock()

		// Wait briefly for network to settle before next retry
		waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Duration(attempt+1)*time.Second)
		_ = networkready.WaitUntilReady(waitCtx)
		waitCancel()
	}

	slog.Error("Failed to restore VPN session after wake-up", "error", lastErr)
	a.SetActiveID("")
	if a.onTrayUpdate != nil {
		a.onTrayUpdate()
	}
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "connection:status", map[string]any{
			"status": "error",
			"id":     activeID,
			"error":  fmt.Sprintf("Failed to auto-reconnect after sleep: %v", lastErr),
		})
	}
}

func (a *App) startHealthWatchdog() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var lastGateway net.IP
	consecutiveFails := 0

	for {
		select {
		case <-a.stopTicker:
			return
		case <-ticker.C:
			actID := a.ActiveID()
			if actID == "" || a.isReconnecting.Load() {
				consecutiveFails = 0
				continue
			}

			// 1. Detect if default gateway changed (e.g. switched Wi-Fi networks)
			if currentGW, err := gateway.DiscoverGateway(); err == nil && currentGW != nil && !currentGW.IsUnspecified() {
				if lastGateway != nil && !lastGateway.Equal(currentGW) {
					slog.Info("Default gateway changed while connected, triggering reconnect", "old", lastGateway, "new", currentGW)
					lastGateway = currentGW
					consecutiveFails = 0
					go a.handleSystemWakeUp()
					continue
				}
				lastGateway = currentGW
			}

			// 2. Active health check: verify that traffic is passing through the tunnel!
			// If probe fails 3 consecutive ticks (15s), auto-heal connection!
			if !a.verifyTunnelConnectivity(3 * time.Second) {
				consecutiveFails++
				slog.Warn("HealthWatchdog: tunnel connectivity probe failed", "consecutiveFails", consecutiveFails, "id", actID)
				if consecutiveFails >= 3 {
					slog.Warn("HealthWatchdog: tunnel dead or unrouted, triggering auto-reconnect", "id", actID)
					consecutiveFails = 0
					go a.handleSystemWakeUp()
				}
			} else {
				consecutiveFails = 0
			}
		}
	}
}

func (a *App) verifyTunnelConnectivity(timeout time.Duration) bool {
	dialer, err := socks5proxy.SOCKS5("tcp", fmt.Sprintf("127.0.0.1:%d", client.DefaultSocksPort), nil, &net.Dialer{Timeout: timeout})
	if err != nil {
		return false
	}

	targets := []string{"1.1.1.1:443", "8.8.8.8:53", "1.1.1.1:53", "cp.cloudflare.com:80", "connectivitycheck.gstatic.com:80"}
	if cd, ok := dialer.(socks5proxy.ContextDialer); ok {
		for _, target := range targets {
			subCtx, subCancel := context.WithTimeout(context.Background(), timeout)
			conn, err := cd.DialContext(subCtx, "tcp", target)
			subCancel()
			if err == nil {
				_ = conn.Close()
				return true
			}
		}
	} else {
		for _, target := range targets {
			if conn, err := dialer.Dial("tcp", target); err == nil {
				_ = conn.Close()
				return true
			}
		}
	}
	return false
}

func (a *App) GetTunnelSettings() TunnelSettingsDTO {
	devIP, dns := a.saveFile.GetTunnelSettings()
	return TunnelSettingsDTO{
		DeviceIP: devIP,
		DNS:      dns,
	}
}

func (a *App) SetTunnelSettings(deviceIP, dns string) error {
	a.saveFile.SetTunnelSettings(deviceIP, dns)
	curDevIP, curDNS := a.saveFile.GetTunnelSettings()
	a.items.SetTunnelSettings(curDevIP, curDNS)
	a.saveFile.Update(a.items)
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "tunnel:settings_changed", TunnelSettingsDTO{
			DeviceIP: curDevIP,
			DNS:      curDNS,
		})
	}
	return nil
}

func (a *App) GetSubscriptions() []subscription.Subscription {
	subs := a.saveFile.GetSubscriptions()
	changed := false
	for i := range subs {
		if subs[i].SubID == "" {
			subs[i].SubID = subscription.ExtractSubIDFromURL(subs[i].URL)
			if subs[i].SubID != "" {
				changed = true
			}
		}
		if subs[i].SubID != "" && (subs[i].Label == "" || !strings.HasPrefix(subs[i].Label, "Subscription-")) {
			subs[i].Label = fmt.Sprintf("Subscription-%s", subs[i].SubID)
			changed = true
		}
	}
	if changed {
		a.saveFile.SetSubscriptions(subs)
	}
	return subs
}

func (a *App) AddConnectionOrSubscription(input, label string) (map[string]any, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, errors.New("input cannot be empty")
	}

	// 1. Check if input is a subscription URL
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		links, sub, err := subscription.FetchSubscription(input, label)
		if err != nil {
			return nil, fmt.Errorf("fetch subscription failed: %w", err)
		}

		subs := a.saveFile.GetSubscriptions()
		subs = append(subs, *sub)
		a.saveFile.SetSubscriptions(subs)

		addedCount := 0
		for _, link := range links {
			if !wireguard.IsAWGLink(link) {
				if _, err := (&xray3.Core{}).CreateProtocol(link); err != nil {
					continue
				}
			} else {
				if _, _, err := wireguard.ParseLink(link); err != nil {
					continue
				}
			}
			itmLabel := subscription.ExtractLabelFromLink(link)
			if itmLabel == "" {
				itmLabel = sub.Label
			}
			if err := a.items.AddItemWithSubscription("", itmLabel, link, sub.ID, 0, 0); err == nil {
				addedCount++
			}
		}

		a.saveFile.Update(a.items)
		if a.ctx != nil {
			wruntime.EventsEmit(a.ctx, "connections:changed", a.GetConnections())
		}

		return map[string]any{
			"type":        "subscription",
			"id":          sub.ID,
			"subId":       sub.SubID,
			"label":       sub.Label,
			"count":       addedCount,
			"lastUpdated": sub.LastUpdated,
		}, nil
	}

	// 2. Single connection link
	dto, err := a.AddConnection(label, input)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"type":       "connection",
		"connection": dto,
	}, nil
}

func (a *App) UpdateSubscription(id string) error {
	subs := a.saveFile.GetSubscriptions()
	var targetSub *subscription.Subscription
	targetIdx := -1
	for i, s := range subs {
		if s.ID == id {
			targetSub = &subs[i]
			targetIdx = i
			break
		}
	}
	if targetSub == nil {
		return fmt.Errorf("subscription %s not found", id)
	}

	links, fetched, err := subscription.FetchSubscription(targetSub.URL, targetSub.Label)
	if err != nil {
		return fmt.Errorf("fetch subscription failed: %w", err)
	}

	fetched.ID = targetSub.ID
	subs[targetIdx] = *fetched
	a.saveFile.SetSubscriptions(subs)

	// Collect existing items belonging to this subscription
	allItems := a.items.All()
	var existing []*connlist.Item
	for _, item := range allItems {
		if item.SubscriptionID() == id {
			existing = append(existing, item)
		}
	}

	activeID := a.ActiveID()
	wasActive := false
	var activeItem *connlist.Item

	// Update existing items in-place to preserve IDs, traffic stats, and order
	for i := 0; i < len(links); i++ {
		link := links[i]
		if !wireguard.IsAWGLink(link) {
			if _, err := (&xray3.Core{}).CreateProtocol(link); err != nil {
				continue
			}
		} else {
			if _, _, err := wireguard.ParseLink(link); err != nil {
				continue
			}
		}
		lbl := subscription.ExtractLabelFromLink(link)
		if lbl == "" {
			lbl = fetched.Label
		}
		if i < len(existing) {
			item := existing[i]
			if item.ID() == activeID {
				wasActive = true
				activeItem = item
				_ = a.Disconnect()
			}
			_ = item.Update(link, lbl)
		} else {
			_ = a.items.AddItemWithSubscription("", lbl, link, id, 0, 0)
		}
	}

	// Remove excess existing items if fewer links returned
	if len(existing) > len(links) {
		for i := len(links); i < len(existing); i++ {
			item := existing[i]
			if item.ID() == activeID {
				_ = a.Disconnect()
			}
			a.items.RemoveItem(item)
		}
	}

	// Reconnect if it was active
	if wasActive && activeItem != nil {
		_ = a.Connect(activeItem.ID())
	}

	a.saveFile.Update(a.items)
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "connections:changed", a.GetConnections())
	}
	return nil
}

func (a *App) DeleteSubscription(id string) error {
	subs := a.saveFile.GetSubscriptions()
	newSubs := make([]subscription.Subscription, 0, len(subs))
	found := false
	for _, s := range subs {
		if s.ID == id {
			found = true
			continue
		}
		newSubs = append(newSubs, s)
	}
	if !found {
		return fmt.Errorf("subscription %s not found", id)
	}
	a.saveFile.SetSubscriptions(newSubs)

	// Remove all items belonging to this subscription
	allItems := a.items.All()
	activeID := a.ActiveID()
	for _, item := range allItems {
		if item.SubscriptionID() == id {
			if item.ID() == activeID {
				_ = a.Disconnect()
			}
			a.items.RemoveItem(item)
		}
	}

	a.saveFile.Update(a.items)
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "connections:changed", a.GetConnections())
	}
	return nil
}

func (a *App) UpdateAllSubscriptions() {
	subs := a.saveFile.GetSubscriptions()
	for _, s := range subs {
		if err := a.UpdateSubscription(s.ID); err != nil {
			slog.Warn("failed to auto-update subscription", "id", s.ID, "url", s.URL, "error", err)
		}
	}
}

func (a *App) GetBridgeGroups() []bridge.BridgeGroup {
	return a.saveFile.GetBridgeGroups()
}

func (a *App) SaveBridgeGroups(groups []bridge.BridgeGroup) error {
	a.saveFile.SetBridgeGroups(groups)
	a.saveFile.Update(a.items)
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "bridge:groups_changed", groups)
	}
	return nil
}

func (a *App) GetBridgeRules() []bridge.BridgeRule {
	return a.saveFile.GetBridgeRules()
}

func (a *App) SaveBridgeRules(rules []bridge.BridgeRule) error {
	a.saveFile.SetBridgeRules(rules)
	a.saveFile.Update(a.items)
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "bridge:rules_changed", rules)
	}
	return nil
}

func (a *App) CheckRunningBridgeProcesses() map[string]int {
	rules := a.saveFile.GetBridgeRules()
	groups := a.saveFile.GetBridgeGroups()
	return bridge.CheckRunningProcesses(rules, groups)
}

func (a *App) LaunchBridgeRule(ruleID string, exePath string) error {
	rules := a.saveFile.GetBridgeRules()
	var rule *bridge.BridgeRule
	for i := range rules {
		if rules[i].ID == ruleID {
			rule = &rules[i]
			break
		}
	}
	if rule == nil {
		return fmt.Errorf("rule %s not found", ruleID)
	}

	target := exePath
	if target == "" {
		target = rule.Pattern
	}
	return bridge.LaunchWithProxy(target, rule.ProxyTarget, rule.ProxyType)
}

func (a *App) SaveWindowGeometry() {
	if a.ctx == nil || a.quitting.Load() {
		return
	}
	a.windowMu.Lock()
	if !a.windowVisible {
		a.windowMu.Unlock()
		return
	}
	a.windowMu.Unlock()

	if wruntime.WindowIsMinimised(a.ctx) {
		return
	}
	w, h := wruntime.WindowGetSize(a.ctx)
	if w < 400 || h < 500 {
		return
	}
	isMax := wruntime.WindowIsMaximised(a.ctx)
	x, y := wruntime.WindowGetPosition(a.ctx)

	geom := a.saveFile.GetWindowGeometry()
	if geom == nil {
		geom = &WindowGeometry{Width: 1024, Height: 700}
	}
	geom.Maximized = isMax
	geom.HasPosition = true
	geom.X = x
	geom.Y = y
	if !isMax {
		geom.Width = w
		geom.Height = h
	}
	a.saveFile.SetWindowGeometry(*geom)
	a.saveFile.SaveWindowAndSettings()
}

func (a *App) checkWindowGeometryChanged() {
	if a.ctx == nil || a.quitting.Load() {
		return
	}
	a.windowMu.Lock()
	if !a.windowVisible {
		a.windowMu.Unlock()
		return
	}
	a.windowMu.Unlock()

	if wruntime.WindowIsMinimised(a.ctx) {
		return
	}
	w, h := wruntime.WindowGetSize(a.ctx)
	if w < 400 || h < 500 {
		return
	}
	isMax := wruntime.WindowIsMaximised(a.ctx)
	x, y := wruntime.WindowGetPosition(a.ctx)

	a.windowMu.Lock()
	changed := false
	if a.lastGeom.Maximized != isMax {
		changed = true
	}
	if !isMax && (a.lastGeom.Width != w || a.lastGeom.Height != h || a.lastGeom.X != x || a.lastGeom.Y != y) {
		changed = true
	}
	if changed {
		a.lastGeom = WindowGeometry{
			Width:       w,
			Height:      h,
			X:           x,
			Y:           y,
			Maximized:   isMax,
			HasPosition: true,
		}
		a.windowMu.Unlock()
		a.saveFile.SetWindowGeometry(a.lastGeom)
		a.saveFile.SaveWindowAndSettings()
	} else {
		a.windowMu.Unlock()
	}
}

func (a *App) ToggleWindow() {
	if a.ctx == nil {
		return
	}
	if wruntime.WindowIsMinimised(a.ctx) {
		a.ShowWindow()
		return
	}
	a.windowMu.Lock()
	visible := a.windowVisible
	a.windowMu.Unlock()

	if visible {
		a.HideWindow()
	} else {
		a.ShowWindow()
	}
}

func (a *App) NotifyWindowVisibility(visible bool) {
	a.windowMu.Lock()
	a.windowVisible = visible
	a.windowMu.Unlock()
	if !visible {
		a.SaveWindowGeometry()
	}
}

func (a *App) ShowWindow() {
	if a.ctx == nil {
		return
	}
	a.windowMu.Lock()
	a.windowVisible = true
	a.windowMu.Unlock()
	wruntime.WindowShow(a.ctx)
	wruntime.WindowUnminimise(a.ctx)
}

func (a *App) HideWindow() {
	if a.ctx == nil {
		return
	}
	a.SaveWindowGeometry()
	a.windowMu.Lock()
	a.windowVisible = false
	a.windowMu.Unlock()
	wruntime.WindowHide(a.ctx)
}

func (a *App) ToggleActiveConnection() {
	actID := a.ActiveID()
	if actID != "" {
		_ = a.Disconnect()
		return
	}
	items := a.items.All()
	if len(items) == 0 {
		return
	}
	_ = a.Connect(items[0].ID())
}

func (a *App) GetHotkeySettings() HotkeySettingsDTO {
	cfg := a.saveFile.GetHotkeyConfig()
	if cfg == nil {
		defWin := "Ctrl+Shift+K"
		defConn := "Ctrl+Shift+C"
		if runtime.GOOS == "darwin" {
			defWin = "Cmd+Shift+K"
			defConn = "Cmd+Shift+C"
		}
		return HotkeySettingsDTO{
			Enabled:       true,
			ToggleWindow:  defWin,
			ToggleConnect: defConn,
		}
	}
	return HotkeySettingsDTO{
		Enabled:       cfg.Enabled,
		ToggleWindow:  cfg.ToggleWindow,
		ToggleConnect: cfg.ToggleConnect,
	}
}

func (a *App) SetHotkeySettings(settings HotkeySettingsDTO) error {
	cfg := HotkeyConfig{
		Enabled:       settings.Enabled,
		ToggleWindow:  strings.TrimSpace(settings.ToggleWindow),
		ToggleConnect: strings.TrimSpace(settings.ToggleConnect),
	}
	a.saveFile.SetHotkeyConfig(cfg)
	a.saveFile.SaveWindowAndSettings()

	a.setupHotkeys()
	return nil
}

func (a *App) setupHotkeys() {
	if a.hotkeyMgr == nil {
		return
	}
	a.hotkeyMgr.UnregisterAll()

	cfg := a.saveFile.GetHotkeyConfig()
	if cfg == nil || !cfg.Enabled {
		return
	}

	if cfg.ToggleWindow != "" {
		err := a.hotkeyMgr.Register(cfg.ToggleWindow, func() {
			a.ToggleWindow()
		})
		if err != nil {
			slog.Warn("Failed to register toggle window hotkey", "shortcut", cfg.ToggleWindow, "error", err)
		}
	}

	if cfg.ToggleConnect != "" {
		err := a.hotkeyMgr.Register(cfg.ToggleConnect, func() {
			a.ToggleActiveConnection()
		})
		if err != nil {
			slog.Warn("Failed to register toggle connection hotkey", "shortcut", cfg.ToggleConnect, "error", err)
		}
	}
}

func (a *App) GetCompactMode() bool {
	return a.saveFile.GetCompactMode()
}

func (a *App) SetCompactMode(compact bool) {
	a.saveFile.SetCompactMode(compact)
	a.saveFile.SaveWindowAndSettings()
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "compact_mode:changed", compact)
	}
}


