package netchart

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/goxray/desktop/internal/netchart/mocks"
)

func TestRecorder_NilSource(t *testing.T) {
	rec := NewRecorder(nil)

	require.Equal(t, 0, rec.WrittenSinceLast())
	require.Equal(t, 0, rec.ReadSinceLast())
}

func TestRecorder(t *testing.T) {
	incR, incW := 0, 0
	i := 0
	sourceMock := mocks.NewMockSource(gomock.NewController(t))
	sourceMock.EXPECT().BytesRead().DoAndReturn(func() int {
		i++
		incR += 1 * i * bytesToMB
		return incR
	}).AnyTimes()
	sourceMock.EXPECT().BytesWritten().DoAndReturn(func() int {
		i++
		incW += 1 * i * bytesToMB
		return incW
	}).AnyTimes()

	rec := NewRecorder(sourceMock)
	rec.recordLimit = 10
	rec.interval = time.Millisecond

	require.Equal(t, rec.interval, rec.RecordInterval())

	rec.Start()
	// Concurrently read while recorder is writing
	done := make(chan struct{})
	go func() {
		defer close(done)
		for j := 0; j < 30; j++ {
			_ = rec.Read()
			_ = rec.Written()
			_ = rec.BytesRead()
			_ = rec.BytesWritten()
			time.Sleep(time.Millisecond)
		}
	}()
	<-done
	rec.Stop()

	require.NotEmpty(t, rec.Read())
	require.NotEmpty(t, rec.Written())
	require.LessOrEqual(t, len(rec.Read()), rec.recordLimit+1)
	require.Equal(t, len(rec.Read()), len(rec.Written()))
	require.Positive(t, rec.BytesWritten())
	require.Positive(t, rec.BytesRead())
}
