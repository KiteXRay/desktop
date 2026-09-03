package sleepwatch

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

type Callback func()

type Config struct {
	OnSleep Callback
	OnWake  Callback
	Logger  *slog.Logger
}

type Watcher struct {
	cfg        Config
	cancel     context.CancelFunc
	ctx        context.Context
	wg         sync.WaitGroup
	lastWake   atomic.Int64 // Unix milli of last wake trigger for debouncing
	isSleeping atomic.Bool
}

func New(cfg Config) *Watcher {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := &Watcher{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}
	return w
}

func (w *Watcher) Start() {
	// 1. Start universal heartbeat time-jump monitor
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.runHeartbeatMonitor()
	}()

	// 2. Start OS-specific monitor (e.g. Linux D-Bus PrepareForSleep)
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.startOSMonitor()
	}()
}

func (w *Watcher) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	w.wg.Wait()
}

func (w *Watcher) triggerSleep() {
	if w.isSleeping.CompareAndSwap(false, true) {
		w.cfg.Logger.Info("SleepWatcher: System sleep detected")
		if w.cfg.OnSleep != nil {
			go w.cfg.OnSleep()
		}
	}
}

func (w *Watcher) triggerWake() {
	now := time.Now().UnixMilli()
	last := w.lastWake.Load()
	// Debounce if multiple signals fire within 4 seconds (e.g. D-Bus + Heartbeat timer)
	if now-last < 4000 {
		return
	}
	w.lastWake.Store(now)
	w.isSleeping.Store(false)

	w.cfg.Logger.Info("SleepWatcher: System wake-up detected, triggering wake callback")
	if w.cfg.OnWake != nil {
		go w.cfg.OnWake()
	}
}

func (w *Watcher) runHeartbeatMonitor() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	lastTime := time.Now()

	for {
		select {
		case <-w.ctx.Done():
			return
		case now := <-ticker.C:
			elapsed := now.Sub(lastTime)
			lastTime = now

			// If >= 3.5 seconds elapsed during a 1-second ticker interval,
			// the OS clock jumped forward due to suspend / sleep.
			if elapsed >= 3500*time.Millisecond {
				w.cfg.Logger.Info("SleepWatcher: Heartbeat time-jump detected", "elapsed", elapsed)
				w.triggerWake()
			}
		}
	}
}
