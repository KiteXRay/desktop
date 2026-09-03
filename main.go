package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
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

func installToOpt() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	optDir := "/opt/kite"
	if err := os.MkdirAll(optDir, 0755); err != nil {
		return fmt.Errorf("create /opt/kite: %w", err)
	}

	targetExe := filepath.Join(optDir, "kite")
	targetIcon := filepath.Join(optDir, "kite.png")

	// 1. Copy binary
	data, err := os.ReadFile(exePath)
	if err != nil {
		return fmt.Errorf("read executable: %w", err)
	}
	if err := os.WriteFile(targetExe, data, 0755); err != nil {
		return fmt.Errorf("write /opt/kite/kite: %w", err)
	}

	// 2. Set capabilities
	if setcapPath, err := exec.LookPath("setcap"); err == nil {
		cmd := exec.Command(setcapPath, "cap_net_raw,cap_net_admin,cap_net_bind_service+eip", targetExe)
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("Warning: setcap failed: %s\n", string(out))
		}
	} else {
		fmt.Println("Warning: setcap tool not found in PATH. Please install libcap2-bin.")
	}

	// 3. Write icons
	_ = os.WriteFile(targetIcon, appIcon, 0644)
	_ = os.MkdirAll("/usr/share/icons/hicolor/512x512/apps", 0755)
	_ = os.WriteFile("/usr/share/icons/hicolor/512x512/apps/kite.png", appIcon, 0644)
	_ = os.MkdirAll("/usr/share/pixmaps", 0755)
	_ = os.WriteFile("/usr/share/pixmaps/kite.png", appIcon, 0644)

	// 4. Desktop entry
	desktopContent := fmt.Sprintf(`[Desktop Entry]
Name=Kite
Comment=Fast, minimal, and transparent desktop VPN client
Exec=%s %%U
Icon=%s
Terminal=false
Type=Application
Categories=Network;VPN;Security;
StartupWMClass=kite
MimeType=x-scheme-handler/vless;x-scheme-handler/vmess;x-scheme-handler/trojan;x-scheme-handler/ss;
`, targetExe, targetIcon)

	_ = os.MkdirAll("/usr/share/applications", 0755)
	_ = os.WriteFile("/usr/share/applications/kite.desktop", []byte(desktopContent), 0644)

	// 5. Symlink /usr/local/bin/kite
	_ = os.Remove("/usr/local/bin/kite")
	_ = os.Symlink(targetExe, "/usr/local/bin/kite")

	// 6. Update desktop & icon caches
	_ = exec.Command("update-desktop-database", "-q", "/usr/share/applications").Run()
	_ = exec.Command("gtk-update-icon-cache", "-q", "/usr/share/icons/hicolor").Run()

	return nil
}

func handleInstallFlag() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	for _, arg := range os.Args[1:] {
		if arg == "--install" || arg == "install" {
			if os.Geteuid() != 0 {
				fmt.Println("Installing Kite to /opt/ requires administrator privileges. Elevating with sudo...")
				exe, _ := os.Executable()
				cmd := exec.Command("sudo", append([]string{exe}, os.Args[1:]...)...)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Stdin = os.Stdin
				if err := cmd.Run(); err != nil {
					fmt.Fprintf(os.Stderr, "Elevated execution failed: %v\n", err)
					os.Exit(1)
				}
				os.Exit(0)
			}
			err := installToOpt()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Installation failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("✓ Kite successfully installed to /opt/kite/kite!")
			fmt.Println("✓ Network capabilities (CAP_NET_ADMIN, CAP_NET_RAW) assigned.")
			fmt.Println("✓ Desktop entry and icons created.")
			fmt.Println("✓ Symlinked to /usr/local/bin/kite")
			os.Exit(0)
		}
	}
	return false
}

func initialize() {
	root.PromptRootAccess()
	ensureDesktopFileLinux()
	dock.SetWindowIconFromPNG(appIcon)

	if has, err := root.HasNetworkPrivileges(); !has {
		_, fixCmd := root.GetPrivilegeFixCommand()
		slog.Warn("Network capabilities missing. Kite needs CAP_NET_ADMIN to configure TUN interfaces.", "cmd", fixCmd, "err", err)
	}
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
	if handleInstallFlag() {
		return
	}
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
