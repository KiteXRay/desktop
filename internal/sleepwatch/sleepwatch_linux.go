//go:build linux

package sleepwatch

import (
	"github.com/godbus/dbus/v5"
)

func (w *Watcher) startOSMonitor() {
	conn, err := dbus.SystemBus()
	if err != nil {
		w.cfg.Logger.Warn("SleepWatcher: Failed to connect to system D-Bus, using heartbeat fallback", "err", err)
		return
	}
	defer conn.Close()

	// Subscribe to login1 PrepareForSleep signal
	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.login1.Manager"),
		dbus.WithMatchMember("PrepareForSleep"),
	); err != nil {
		w.cfg.Logger.Warn("SleepWatcher: Failed to add D-Bus match for PrepareForSleep", "err", err)
		return
	}

	sigChan := make(chan *dbus.Signal, 10)
	conn.Signal(sigChan)
	defer conn.RemoveSignal(sigChan)

	w.cfg.Logger.Info("SleepWatcher: Linux D-Bus listener active for login1.PrepareForSleep")

	for {
		select {
		case <-w.ctx.Done():
			return
		case sig, ok := <-sigChan:
			if !ok {
				return
			}
			if sig == nil {
				continue
			}

			if sig.Name == "org.freedesktop.login1.Manager.PrepareForSleep" && len(sig.Body) > 0 {
				goingToSleep, ok := sig.Body[0].(bool)
				if ok {
					if goingToSleep {
						w.triggerSleep()
					} else {
						w.triggerWake()
					}
				}
			}
		}
	}
}
