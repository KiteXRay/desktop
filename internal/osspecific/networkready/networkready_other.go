//go:build !linux

package networkready

import (
	"context"
	"net"
	"time"

	"github.com/jackpal/gateway"
)

func waitUntilReadyOS(ctx context.Context) bool {
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			gw, err := gateway.DiscoverGateway()
			if err != nil || gw == nil || gw.IsUnspecified() {
				continue
			}

			d := net.Dialer{Timeout: 700 * time.Millisecond}
			if conn, err := d.DialContext(ctx, "udp", "1.1.1.1:53"); err == nil {
				_ = conn.Close()
				return true
			}
			if conn, err := d.DialContext(ctx, "tcp", "8.8.8.8:53"); err == nil {
				_ = conn.Close()
				return true
			}
		}
	}
}
