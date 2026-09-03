package sleepwatch

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSleepWatcherWakeDebounce(t *testing.T) {
	var wakeCount atomic.Int32
	var sleepCount atomic.Int32

	w := New(Config{
		OnSleep: func() {
			sleepCount.Add(1)
		},
		OnWake: func() {
			wakeCount.Add(1)
		},
	})

	w.triggerSleep()
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), sleepCount.Load())

	// First wake should fire
	w.triggerWake()
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), wakeCount.Load())

	// Immediate second wake should be debounced
	w.triggerWake()
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), wakeCount.Load())
}

func TestSleepWatcherStartStop(t *testing.T) {
	w := New(Config{})
	w.Start()
	time.Sleep(50 * time.Millisecond)
	w.Stop()
}
