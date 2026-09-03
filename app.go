package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/energye/systray"
	"github.com/jackpal/gateway"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	socks5proxy "golang.org/x/net/proxy"

	"github.com/goxray/core/client"
	"github.com/goxray/desktop/internal/appscan"
	"github.com/goxray/desktop/internal/connlist"
	"github.com/goxray/desktop/internal/osspecific/clean"
	"github.com/goxray/desktop/internal/osspecific/networkready"
	"github.com/goxray/desktop/internal/osspecific/proxy"
	"github.com/goxray/desktop/internal/sleepwatch"
	xray3 "github.com/lilendian0x00/xray-knife/v3/pkg/xray"
)

type ProxyEndpointsDTO struct {
	Socks5Host string `json:"socks5Host"`
	Socks5Port int    `json:"socks5Port"`
	HTTPHost   string `json:"httpHost"`
	HTTPPort   int    `json:"httpPort"`
	Socks5URL  string `json:"socks5Url"`
	HTTPURL    string `json:"httpUrl"`
}

type ConnectionDTO struct {
	ID           string            `json:"id"`
	Label        string            `json:"label"`
	Link         string            `json:"link"`
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
}

func NewApp() *App {
	items := connlist.New()
	saveFile := NewSaveFile()

	app := &App{
		items:      items,
		saveFile:   saveFile,
		stopTicker: make(chan struct{}),
		tunnelMode: saveFile.GetTunnelMode(),
	}

	saveFile.Load(items)

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
	go a.startStatsTicker()
	a.startSleepWatcher()
	go a.startHealthWatchdog()
}

func (a *App) shutdown(ctx context.Context) {
	if a.sleepWatcher != nil {
		a.sleepWatcher.Stop()
	}
	close(a.stopTicker)
	_ = a.Disconnect()
}

func (a *App) startStatsTicker() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	flushTicks := 0

	for {
		select {
		case <-a.stopTicker:
			return
		case <-ticker.C:
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
			ID:           item.ID(),
			Label:        item.Label(),
			Link:         item.Link(),
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
	if label == "" || link == "" {
		return nil, errors.New("label and link cannot be empty")
	}

	proto, err := (&xray3.Core{}).CreateProtocol(link)
	if err != nil {
		return nil, fmt.Errorf("create xray protocol: %w", err)
	}
	if err := proto.Parse(); err != nil {
		return nil, fmt.Errorf("parse xray protocol: %w", err)
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

func (a *App) UpdateConnection(id string, label, link string) error {
	if label == "" || link == "" {
		return errors.New("label and link cannot be empty")
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
	currentActive := a.ActiveID()
	// If already active on this one, disconnect
	if currentActive == id {
		return a.Disconnect()
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

	tMode := client.TunnelModeSystem
	if a.GetTunnelMode() == "per_app" {
		tMode = client.TunnelModePerApp
	}
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

	// Ensure system proxy remains OFF unless explicitly toggled by user
	_ = proxy.SetSystemProxy(false, "127.0.0.1", client.DefaultHTTPPort, client.DefaultSocksPort)
	a.systemProxyOn = false

	if a.onTrayUpdate != nil {
		a.onTrayUpdate()
	}
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "connections:changed", a.GetConnections())
		wruntime.EventsEmit(a.ctx, "connection:status", map[string]any{
			"status": "connected",
			"id":     id,
			"mode":   a.GetTunnelMode(),
		})
	}

	return nil
}

func (a *App) Disconnect() error {
	a.connectMu.Lock()
	defer a.connectMu.Unlock()

	if a.systemProxyOn {
		_ = proxy.SetSystemProxy(false, "127.0.0.1", client.DefaultHTTPPort, client.DefaultSocksPort)
		a.systemProxyOn = false
	}

	currentActive := a.ActiveID()
	if currentActive == "" {
		return nil
	}

	if item := a.items.FindByID(currentActive); item != nil {
		if err := item.Disconnect(); err != nil {
			slog.Error("error disconnecting", "error", err)
		}
		item.SetActive(false)
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

func (a *App) GetAppInfo() AppInfoDTO {
	return AppInfoDTO{
		Name:        "Kite",
		Version:     "1.0.0",
		RepoURL:     "https://github.com/KiteXRay/desktop",
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		Description: "Fast, minimal, and transparent desktop VPN client.",
	}
}

func (a *App) OpenURL(targetURL string) {
	if a.ctx != nil {
		wruntime.BrowserOpenURL(a.ctx, targetURL)
	}
}

func (a *App) ParseLinkPreview(link string) (map[string]string, error) {
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
	if a.tunnelMode == "" {
		return "system"
	}
	return a.tunnelMode
}

func (a *App) SetTunnelMode(mode string) error {
	a.connectMu.Lock()
	defer a.connectMu.Unlock()

	if mode != "per_app" {
		mode = "system"
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

			tMode := client.TunnelModeSystem
			if mode == "per_app" {
				tMode = client.TunnelModePerApp
			}
			if err := item.ConnectWithMode(tMode); err != nil {
				slog.Error("failed to reconnect with new mode", "error", err)
				item.SetActive(false)
				a.SetActiveID("")
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

			// In both modes, system proxy remains OFF unless explicitly enabled by user
			_ = proxy.SetSystemProxy(false, "127.0.0.1", client.DefaultHTTPPort, client.DefaultSocksPort)
			a.systemProxyOn = false

			if a.onTrayUpdate != nil {
				a.onTrayUpdate()
			}
			if a.ctx != nil {
				wruntime.EventsEmit(a.ctx, "connections:changed", a.GetConnections())
				wruntime.EventsEmit(a.ctx, "connection:status", map[string]any{
					"status": "connected",
					"id":     activeID,
					"mode":   a.GetTunnelMode(),
				})
			}
		}
	} else {
		_ = proxy.SetSystemProxy(false, "127.0.0.1", client.DefaultHTTPPort, client.DefaultSocksPort)
		a.systemProxyOn = false
	}

	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "proxy:status", a.systemProxyOn)
		wruntime.EventsEmit(a.ctx, "mode:changed", mode)
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
		slog.Info("Reconnecting VPN session after sleep", "attempt", attempt, "id", activeID)
		err := a.connectInternal(activeID)
		if err == nil {
			// Verify that traffic actually passes through the tunnel to the server!
			time.Sleep(300 * time.Millisecond)
			if a.verifyTunnelConnectivity(2500 * time.Millisecond) {
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
			// If probe fails 2 consecutive ticks (10s), auto-heal connection!
			if !a.verifyTunnelConnectivity(2 * time.Second) {
				consecutiveFails++
				slog.Warn("HealthWatchdog: tunnel connectivity probe failed", "consecutiveFails", consecutiveFails, "id", actID)
				if consecutiveFails >= 2 {
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
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if cd, ok := dialer.(socks5proxy.ContextDialer); ok {
		if conn, err := cd.DialContext(ctx, "tcp", "1.1.1.1:443"); err == nil {
			_ = conn.Close()
			return true
		}
		if conn, err := cd.DialContext(ctx, "tcp", "8.8.8.8:53"); err == nil {
			_ = conn.Close()
			return true
		}
	} else {
		if conn, err := dialer.Dial("tcp", "1.1.1.1:443"); err == nil {
			_ = conn.Close()
			return true
		}
	}
	return false
}

