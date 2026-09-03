package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"github.com/energye/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/goxray/desktop/icon"
	"github.com/goxray/desktop/internal/osspecific/dock"
	"github.com/goxray/desktop/internal/osspecific/root"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

const AppTitleName = "Kite"

func ensureDesktopFileLinux() {
	if runtime.GOOS != "linux" {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	iconDir := filepath.Join(home, ".local", "share", "icons", "hicolor", "512x512", "apps")
	_ = os.MkdirAll(iconDir, 0755)
	_ = os.WriteFile(filepath.Join(iconDir, "kite.png"), appIcon, 0644)

	pixmapDir := filepath.Join(home, ".local", "share", "pixmaps")
	_ = os.MkdirAll(pixmapDir, 0755)
	_ = os.WriteFile(filepath.Join(pixmapDir, "kite.png"), appIcon, 0644)

	appDir := filepath.Join(home, ".local", "share", "applications")
	_ = os.MkdirAll(appDir, 0755)

	exePath, err := os.Executable()
	if err != nil || filepath.Base(exePath) == "wailsbindings" || strings.Contains(exePath, "/tmp/") {
		return
	}

	iconPath := filepath.Join(iconDir, "kite.png")

	content := fmt.Sprintf(`[Desktop Entry]
Name=Kite
Comment=Fast & Minimal Desktop VPN Client
Exec=%s
Icon=%s
Terminal=false
Type=Application
Categories=Network;VPN;
StartupWMClass=kite
`, exePath, iconPath)

	_ = os.WriteFile(filepath.Join(appDir, "kite.desktop"), []byte(content), 0644)
}

func initialize() {
	root.PromptRootAccess()
	ensureDesktopFileLinux()
	dock.SetWindowIconFromPNG(appIcon)
}

type TrayController struct {
	app      *App
	mDisconn *systray.MenuItem
	mOpen    *systray.MenuItem
	mQuit    *systray.MenuItem
	mu       sync.Mutex
	stopTray func()
}

func (tc *TrayController) updateMenu() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	systray.ResetMenu()

	mHeader := systray.AddMenuItem(AppTitleName, "Kite")
	mHeader.Disable()
	systray.AddSeparator()

	tc.mOpen = systray.AddMenuItem("Open Kite", "Show main application window")
	tc.mOpen.Click(func() {
		go func() {
			if tc.app.ctx != nil {
				wruntime.WindowShow(tc.app.ctx)
				wruntime.WindowUnminimise(tc.app.ctx)
			}
		}()
	})
	systray.AddSeparator()

	conns := tc.app.GetConnections()
	actID := tc.app.ActiveID()

	var activeLabel string
	if len(conns) == 0 {
		mEmpty := systray.AddMenuItem("No Connections", "")
		mEmpty.Disable()
	} else {
		for i, conn := range conns {
			connID := conn.ID
			isActive := conn.Active || (actID != "" && actID == connID)
			title := conn.Label
			if title == "" {
				title = fmt.Sprintf("Connection #%d", i+1)
			}
			if isActive {
				activeLabel = title
				title = "● " + title
			} else {
				title = "○ " + title
			}
			item := systray.AddMenuItemCheckbox(title, conn.Link, isActive)
			item.Click(func() {
				go func() {
					if tc.app.ActiveID() == connID {
						_ = tc.app.Disconnect()
					} else {
						_ = tc.app.Connect(connID)
					}
				}()
			})
		}
	}

	systray.AddSeparator()

	tc.mDisconn = systray.AddMenuItem("Disconnect", "Disconnect active VPN")
	if actID == "" {
		tc.mDisconn.Disable()
	}
	tc.mDisconn.Click(func() {
		go func() {
			_ = tc.app.Disconnect()
		}()
	})

	systray.AddSeparator()

	tc.mQuit = systray.AddMenuItem("Quit", "Quit application")
	tc.mQuit.Click(func() {
		go func() {
			tc.app.Quit()
		}()
	})

	if actID != "" {
		systray.SetIcon(icon.LogoActive)
		if activeLabel != "" {
			systray.SetTooltip(fmt.Sprintf("%s - %s", AppTitleName, activeLabel))
		}
	} else {
		systray.SetIcon(icon.LogoPassive)
		systray.SetTooltip(AppTitleName)
	}
}

func setupSystray(app *App) *TrayController {
	tc := &TrayController{
		app: app,
	}

	onReady := func() {
		systray.SetTooltip(AppTitleName)
		systray.SetIcon(icon.LogoPassive)
		dock.HideIconInDock()

		// Left click on tray icon restores/focuses window
		systray.SetOnClick(func(menu systray.IMenu) {
			if app.ctx != nil {
				wruntime.WindowShow(app.ctx)
				wruntime.WindowUnminimise(app.ctx)
			}
		})

		tc.updateMenu()
	}

	onExit := func() {
		_ = app.Disconnect()
	}

	start, end := systray.RunWithExternalLoop(onReady, onExit)
	start()
	tc.stopTray = end

	app.onTrayUpdate = func() {
		tc.updateMenu()
	}

	return tc
}

func main() {
	initialize()

	app := NewApp()
	var tc *TrayController

	// Gracefully handle terminal Ctrl+C and termination signals
	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigChan
		slog.Info("Received terminal shutdown signal (Ctrl+C), cleaning up...")
		app.Quit()
		<-sigChan
		slog.Warn("Received second shutdown signal, forcing exit")
		os.Exit(1)
	}()

	defer func() {
		_ = app.Disconnect()
		if tc != nil && tc.stopTray != nil {
			tc.stopTray()
		}
	}()

	err := wails.Run(&options.App{
		Title:             AppTitleName,
		Width:             1024,
		Height:            700,
		MinWidth:          820,
		MinHeight:         580,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 2, G: 6, B: 23, A: 1}, // Slate-950
		OnStartup: func(ctx context.Context) {
			tc = setupSystray(app)
			app.startup(ctx)
		},
		OnShutdown: func(ctx context.Context) {
			if tc != nil && tc.stopTray != nil {
				tc.stopTray()
			}
			app.shutdown(ctx)
		},
		HideWindowOnClose: true,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "e8b62569-8c9b-4f8a-9d45-7b034e5d4761",
			OnSecondInstanceLaunch: func(secondInstanceData options.SecondInstanceData) {
				slog.Info("Second instance launched, focusing window", "args", secondInstanceData.Args)
				if app.ctx != nil {
					wruntime.Show(app.ctx)
					wruntime.WindowShow(app.ctx)
					wruntime.WindowUnminimise(app.ctx)

					for _, arg := range secondInstanceData.Args {
						arg = strings.TrimSpace(arg)
						if strings.HasPrefix(arg, "vless://") || strings.HasPrefix(arg, "vmess://") ||
							strings.HasPrefix(arg, "trojan://") || strings.HasPrefix(arg, "ss://") {
							_, _ = app.AddConnection("", arg)
						}
					}
				}
			},
		},
		Linux: &linux.Options{
			Icon:        appIcon,
			ProgramName: "kite",
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		slog.Error("error running wails application", "error", err)
	}
}
