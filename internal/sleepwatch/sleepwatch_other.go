//go:build !linux

package sleepwatch

func (w *Watcher) startOSMonitor() {
	// On non-Linux platforms, the universal heartbeat time-jump monitor handles sleep/wake detection.
	<-w.ctx.Done()
}
