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

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/KiteXRay/desktop/icon"
	"github.com/KiteXRay/desktop/internal/osspecific/dock"
	"github.com/KiteXRay/desktop/internal/osspecific/root"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

const AppTitleName = "Kite"

func init() {
	if runtime.GOOS == "linux" && os.Getenv("GDK_BACKEND") == "" {
		_ = os.Setenv("GDK_BACKEND", "x11")
	}
}

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
	exePath, _ := os.Executable()
	if strings.Contains(exePath, "wailsbindings") || strings.Contains(exePath, "/tmp/") {
		return
	}
	root.PromptRootAccess()
	ensureDesktopFileLinux()
	dock.SetWindowIconFromPNG(appIcon)

	if has, err := root.HasNetworkPrivileges(); !has {
		_, fixCmd := root.GetPrivilegeFixCommand()
		slog.Warn("Network capabilities missing. Kite needs CAP_NET_ADMIN to configure TUN interfaces.", "cmd", fixCmd, "err", err)
	}
}

type TrayController struct {
	app  *App
	tray *application.SystemTray
	mu   sync.Mutex
}

func (tc *TrayController) updateMenu() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	menu := application.NewMenu()

	mMode := menu.AddSubmenu("Mode")
	currentMode := tc.app.GetTunnelMode()

	type modeOption struct {
		mode  string
		label string
	}
	modes := []modeOption{
		{mode: "tunnel", label: "Tunnel"},
		{mode: "proxy", label: "Proxy"},
		{mode: "bridge", label: "Bridge"},
	}

	for _, opt := range modes {
		targetMode := opt.mode
		isActive := currentMode == targetMode
		title := opt.label
		if isActive {
			title = "● " + title
		} else {
			title = "○ " + title
		}
		item := mMode.AddCheckbox(title, isActive)
		item.OnClick(func(ctx *application.Context) {
			tc.switchMode(targetMode)
		})
	}
	menu.AddSeparator()

	conns := tc.app.GetConnections()
	actID := tc.app.ActiveID()

	var activeLabel string
	if len(conns) == 0 {
		mEmpty := menu.Add("No Connections")
		mEmpty.SetEnabled(false)
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
			item := menu.AddCheckbox(title, isActive)
			item.OnClick(func(ctx *application.Context) {
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

	menu.AddSeparator()

	mDisconn := menu.Add("Disconnect")
	if actID == "" {
		mDisconn.SetEnabled(false)
	}
	mDisconn.OnClick(func(ctx *application.Context) {
		go func() {
			_ = tc.app.Disconnect()
		}()
	})

	menu.AddSeparator()

	mQuit := menu.Add("Quit")
	mQuit.OnClick(func(ctx *application.Context) {
		go func() {
			tc.app.Quit()
		}()
	})

	if actID != "" {
		tc.tray.SetIcon(icon.LogoActive)
		if activeLabel != "" {
			tc.tray.SetTooltip(fmt.Sprintf("%s - %s", AppTitleName, activeLabel))
		}
	} else {
		tc.tray.SetIcon(icon.LogoPassive)
		tc.tray.SetTooltip(AppTitleName)
	}
	tc.tray.SetMenu(menu)
}

func (tc *TrayController) switchMode(targetMode string) {
	go func() {
		_ = tc.app.SetTunnelMode(targetMode)
	}()
}

func setupSystray(wailsApp *application.App, app *App) *TrayController {
	tray := wailsApp.SystemTray.New()
	tray.SetTooltip(AppTitleName)
	tray.SetIcon(icon.LogoPassive)
	dock.HideIconInDock()

	if runtime.GOOS != "darwin" {
		tray.OnClick(func() {
			app.ShowWindow()
		})
	}

	tc := &TrayController{
		app:  app,
		tray: tray,
	}

	tc.updateMenu()

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

	geom := app.saveFile.GetWindowGeometry()
	startWidth := 1024
	startHeight := 700
	if geom != nil && geom.Width >= 400 && geom.Height >= 500 {
		startWidth = geom.Width
		startHeight = geom.Height
	}

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

	var tc *TrayController

	wailsApp := application.New(application.Options{
		Name:        AppTitleName,
		Description: "Fast & Minimal Desktop VPN Client",
		Icon:        appIcon,
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
		},
		Linux: application.LinuxOptions{
			ProgramName:                   "kite",
			DisableQuitOnLastWindowClosed: true,
		},
		Services: []application.Service{
			application.NewService(app),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "e8b62569-8c9b-4f8a-9d45-7b034e5d4761",
			OnSecondInstanceLaunch: func(secondInstanceData application.SecondInstanceData) {
				slog.Info("Second instance launched, focusing window", "args", secondInstanceData.Args)
				app.ShowWindow()

				for _, arg := range secondInstanceData.Args {
					arg = strings.TrimSpace(arg)
					if strings.HasPrefix(arg, "vless://") || strings.HasPrefix(arg, "vmess://") ||
						strings.HasPrefix(arg, "trojan://") || strings.HasPrefix(arg, "ss://") {
						_, _ = app.AddConnection("", arg)
					}
				}
			},
		},
		OnShutdown: func() {
			if tc != nil && tc.tray != nil {
				tc.tray.Destroy()
			}
			app.shutdown(context.Background())
		},
	})

	isAutostart := false
	for _, arg := range os.Args[1:] {
		if arg == "--autostart" || arg == "-autostart" {
			isAutostart = true
			break
		}
	}

	winOpts := application.WebviewWindowOptions{
		Name:             "main",
		Title:            AppTitleName,
		Width:            startWidth,
		Height:           startHeight,
		MinWidth:         400,
		MinHeight:        520,
		URL:              "/",
		BackgroundColour: application.RGBA{Red: 2, Green: 6, Blue: 23, Alpha: 255}, // Slate-950
		Hidden:           isAutostart,
	}

	if geom != nil && geom.Maximized {
		winOpts.StartState = application.WindowStateMaximised
	} else if geom != nil && geom.HasPosition {
		winOpts.X = geom.X
		winOpts.Y = geom.Y
		winOpts.InitialPosition = application.WindowXY
	}

	win := wailsApp.Window.NewWithOptions(winOpts)
	if isAutostart {
		app.windowVisible = false
		win.Hide()
	}

	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		win.Hide()
	})

	tc = setupSystray(wailsApp, app)
	dock.SetWindowIconFromPNG(appIcon)
	app.startup(context.Background())

	err := wailsApp.Run()
	if err != nil {
		slog.Error("error running wails application", "error", err)
	}
}
