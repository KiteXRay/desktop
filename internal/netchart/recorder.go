//go:generate mockgen -destination=mocks/recorder_mocks.go -source=recorder.go -package=mocks -typed

package netchart

import (
	"context"
	"slices"
	"sync"
	"time"
)

const bytesToMB = 1024 * 1024

type Source interface {
	BytesRead() int
	BytesWritten() int
}

type Recorder struct {
	base     Source
	interval time.Duration
	mu       sync.RWMutex

	stopRecording   func()
	done            chan struct{}
	recordedRead    []float64
	recordedWritten []float64
	recordLimit     int
	totalRead       int
	totalWrite      int
}

// NewRecorder creates a default Recorder
// TODO: Decrease detalization for old data as the time goes on to allow for longer ranges charts.
func NewRecorder(s Source) *Recorder {
	return &Recorder{
		base:        s,
		interval:    time.Second, // store data value per interval
		recordLimit: 60,          // store and record last 60 seconds (1 minute rolling window)
		done:        make(chan struct{}, 1),
	}
}

func (r *Recorder) Read() []float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]float64, len(r.recordedRead))
	copy(res, r.recordedRead)
	return res
}

func (r *Recorder) Written() []float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]float64, len(r.recordedWritten))
	copy(res, r.recordedWritten)
	return res
}

func (r *Recorder) RecordInterval() time.Duration {
	return r.interval
}

func (r *Recorder) Start() {
	var ctx context.Context
	ctx, r.stopRecording = context.WithCancel(context.Background())
	r.done = make(chan struct{}, 1)
	ticker := time.NewTicker(r.interval)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				select {
				case r.done <- struct{}{}:
				default:
				}
				return
			case <-ticker.C:
				r.mu.Lock()
				rawRead := float64(r.ReadSinceLast()) / float64(bytesToMB)
				rawWritten := float64(r.WrittenSinceLast()) / float64(bytesToMB)

				if len(r.recordedRead) >= r.recordLimit {
					r.recordedRead = slices.Delete(r.recordedRead, 0, 1)
				}
				r.recordedRead = append(r.recordedRead, rawRead)

				if len(r.recordedWritten) >= r.recordLimit {
					r.recordedWritten = slices.Delete(r.recordedWritten, 0, 1)
				}
				r.recordedWritten = append(r.recordedWritten, rawWritten)
				r.mu.Unlock()
			}
		}
	}()
}

func (r *Recorder) Stop() {
	if r.stopRecording != nil {
		r.stopRecording()
		select {
		case <-r.done:
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (r *Recorder) BytesRead() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.totalRead
}

func (r *Recorder) BytesWritten() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.totalWrite
}

// ReadSinceLast returns bytes read from last call (upload).
// Must be called with r.mu locked or during single-threaded initialization.
func (r *Recorder) ReadSinceLast() int {
	if r.base == nil {
		return 0
	}
	baseRead := r.base.BytesRead()
	readSinceLast := baseRead - r.totalRead
	r.totalRead = baseRead

	return max(readSinceLast, 0)
}

// WrittenSinceLast returns bytes written from last call (download).
// Must be called with r.mu locked or during single-threaded initialization.
func (r *Recorder) WrittenSinceLast() int {
	if r.base == nil {
		return 0
	}
	baseWritten := r.base.BytesWritten()
	writtenSinceLast := baseWritten - r.totalWrite
	r.totalWrite = baseWritten

	return max(writtenSinceLast, 0)
}
