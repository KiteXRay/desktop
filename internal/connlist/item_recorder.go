//go:generate mockgen -destination=mocks/item_recorder_mock.go -source=item_recorder.go -package=mocks -typed

package connlist

import (
	"time"
)

type NetworkRecorder interface {
	// Start should start recording data.
	Start()
	// Stop stops the recorder and cleans up background goroutines.
	Stop()
	// Read should return values for uplink for each previous RecordInterval.
	// Number of values returned must match Written.
	Read() []float64
	// Written should return values for downlink for each previous RecordInterval.
	// Number of values returned must match Written.
	Written() []float64
	// BytesRead should return the total number of bytes for uplink.
	BytesRead() int
	// BytesWritten should return the total number of bytes for downlink.
	BytesWritten() int
	RecordInterval() time.Duration
}

func (c *Item) Read() []float64 {
	return c.recorder.Read()
}

func (c *Item) Written() []float64 {
	return c.recorder.Written()
}

func (c *Item) BytesRead() int64 {
	c.sampleTraffic()
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.totalRead
}

func (c *Item) BytesWritten() int64 {
	c.sampleTraffic()
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.totalWritten
}

func (c *Item) RecordInterval() time.Duration {
	return c.recorder.RecordInterval()
}
