//go:build windows

package tun

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wintun"
)

//go:embed wintun.dll
var embeddedWintunDLL []byte

func ensureWintunDLL() {
	if _, err := os.Stat("wintun.dll"); err == nil {
		return
	}
	exe, err := os.Executable()
	if err == nil {
		target := filepath.Join(filepath.Dir(exe), "wintun.dll")
		if _, err := os.Stat(target); err == nil {
			return
		}
		_ = os.WriteFile(target, embeddedWintunDLL, 0755)
	}
}

// Interface is a TUN interface implementation for Windows using Wintun.
type Interface struct {
	name     string
	mtu      int
	adapter  *wintun.Adapter
	session  wintun.Session
	readWait windows.Handle
	closed   atomic.Bool
	mu       sync.RWMutex
}

// New creates a new TUN interface using Wintun driver.
func New(name string, MTU int) (*Interface, error) {
	ensureWintunDLL()

	if name == "" {
		name = "kite0"
	}
	if MTU <= 0 {
		MTU = 1500
	}

	// Always remove any stale adapter left behind from a previous run or crash
	if old, err := wintun.OpenAdapter(name); err == nil && old != nil {
		_ = old.Close()
		time.Sleep(100 * time.Millisecond)
	}

	adapter, err := wintun.CreateAdapter(name, "Kite", nil)
	if err != nil {
		// Attempt to open existing adapter if creation returned already exists error
		adapter, err = wintun.OpenAdapter(name)
		if err != nil {
			return nil, fmt.Errorf("create/open wintun adapter %q: %w", name, err)
		}
	}

	session, err := adapter.StartSession(0x800000) // 8 MiB ring buffer
	if err != nil {
		adapter.Close()
		return nil, fmt.Errorf("start wintun session: %w", err)
	}

	return &Interface{
		name:     name,
		mtu:      MTU,
		adapter:  adapter,
		session:  session,
		readWait: session.ReadWaitEvent(),
	}, nil
}

// Up brings the TUN interface up and assigns IPv4 address and netmask.
func (i *Interface) Up(local *net.IPNet, gw net.IP) error {
	mask := "255.255.255.0"
	if local.Mask != nil && len(local.Mask) == 4 {
		m := fmt.Sprintf("%d.%d.%d.%d", local.Mask[0], local.Mask[1], local.Mask[2], local.Mask[3])
		// Windows netsh rejects 255.255.255.255 (/32). Wintun adapters require a valid subnet (e.g. 255.255.255.0).
		if m != "255.255.255.255" {
			mask = m
		}
	}

	// Use netsh to configure the static IP and netmask on the adapter
	cmd := exec.Command("netsh", "interface", "ipv4", "set", "address",
		fmt.Sprintf("name=%s", i.name),
		"source=static",
		fmt.Sprintf("addr=%s", local.IP.String()),
		fmt.Sprintf("mask=%s", mask),
		"gateway=none",
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		slog.Warn("netsh set address", "err", err, "output", string(out))
	}

	// Set MTU
	mtuCmd := exec.Command("netsh", "interface", "ipv4", "set", "subinterface",
		i.name,
		fmt.Sprintf("mtu=%d", i.mtu),
		"store=active",
	)
	mtuCmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}
	_ = mtuCmd.Run()

	return nil
}

// Name returns the interface name.
func (i *Interface) Name() string {
	return i.name
}

// Read reads an IP packet from the Wintun ring buffer.
func (i *Interface) Read(p []byte) (int, error) {
	for {
		if i.closed.Load() {
			return 0, io.EOF
		}

		i.mu.RLock()
		if i.closed.Load() {
			i.mu.RUnlock()
			return 0, io.EOF
		}

		packet, err := i.session.ReceivePacket()
		if err == nil {
			var n int
			if len(packet) > 0 {
				n = copy(p, packet)
				i.session.ReleaseReceivePacket(packet)
			}
			i.mu.RUnlock()
			return n, nil
		}
		i.mu.RUnlock()

		if errors.Is(err, windows.ERROR_HANDLE_EOF) {
			return 0, io.EOF
		}

		// Wait for next packet or timeout
		event, _ := windows.WaitForSingleObject(i.readWait, 100)
		if event == windows.WAIT_FAILED {
			if i.closed.Load() {
				return 0, io.EOF
			}
		}
	}
}

// Write writes an IP packet into the Wintun ring buffer.
func (i *Interface) Write(p []byte) (int, error) {
	if i.closed.Load() {
		return 0, io.ErrClosedPipe
	}
	if len(p) == 0 {
		return 0, nil
	}

	i.mu.RLock()
	defer i.mu.RUnlock()

	if i.closed.Load() {
		return 0, io.ErrClosedPipe
	}

	packet, err := i.session.AllocateSendPacket(len(p))
	if err != nil {
		if errors.Is(err, windows.ERROR_BUFFER_OVERFLOW) {
			// Ring buffer is full; packet drop is standard behavior under buffer overflow
			return len(p), nil
		}
		return 0, err
	}

	copy(packet, p)
	i.session.SendPacket(packet)
	return len(p), nil
}

// Close closes the Wintun session and adapter.
func (i *Interface) Close() error {
	if i.closed.CompareAndSwap(false, true) {
		windows.SetEvent(i.readWait)

		// Wait for active Read / Write calls to finish before unmapping ring buffer memory
		i.mu.Lock()
		defer i.mu.Unlock()

		i.session.End()
		if i.adapter != nil {
			_ = i.adapter.Close()
		}
	}
	return nil
}

// nameBytes transforms Name() into fixed 16 byte array for compatibility.
func (i *Interface) nameBytes() [16]byte {
	sb := make([]byte, 16)
	copy(sb[:len(i.Name())], i.Name())
	return [16]byte(sb)
}
