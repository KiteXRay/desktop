package root

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindTunnelBinary_EnvOverride(t *testing.T) {
	tmpDir := t.TempDir()
	fakeTunnel := filepath.Join(tmpDir, "kite-tunnel")
	err := os.WriteFile(fakeTunnel, []byte("#!/bin/sh\nexit 0\n"), 0o755)
	require.NoError(t, err)

	t.Setenv("KITE_TUNNEL_PATH", fakeTunnel)

	bin, err := FindTunnelBinary()
	require.NoError(t, err)
	require.Equal(t, fakeTunnel, bin)
}

func TestFindTunnelBinary_Defaults(t *testing.T) {
	t.Setenv("KITE_TUNNEL_PATH", "")

	bin, err := FindTunnelBinary()
	require.NotEmpty(t, bin)
	// Even if not found on disk in some environments, default path is returned
	if err != nil {
		require.Contains(t, bin, "kite-tunnel")
	}
}
