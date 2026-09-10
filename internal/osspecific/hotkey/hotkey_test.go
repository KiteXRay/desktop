package hotkey

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseShortcut(t *testing.T) {
	tests := []struct {
		input    string
		wantErr  bool
		wantMods Modifier
		wantKey  string
	}{
		{
			input:    "Ctrl+Shift+K",
			wantErr:  false,
			wantMods: ModCtrl | ModShift,
			wantKey:  "K",
		},
		{
			input:    "ctrl+alt+c",
			wantErr:  false,
			wantMods: ModCtrl | ModAlt,
			wantKey:  "C",
		},
		{
			input:    "Cmd+Shift+K",
			wantErr:  false,
			wantMods: ModMeta | ModShift,
			wantKey:  "K",
		},
		{
			input:   "",
			wantErr: true,
		},
		{
			input:   "Ctrl+Shift+",
			wantErr: true,
		},
		{
			input:   "InvalidMod+K",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			sc, err := ParseShortcut(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, sc)
				assert.Equal(t, tt.wantMods, sc.Modifiers)
				assert.Equal(t, tt.wantKey, sc.Key)
			}
		})
	}
}

func TestNewManager_DoesNotPanic(t *testing.T) {
	mgr := NewManager()
	require.NotNil(t, mgr)
	defer mgr.Close()

	// Register dummy shortcut
	_ = mgr.Register("Ctrl+Shift+K", func() {})
	_ = mgr.Unregister("Ctrl+Shift+K")
	mgr.UnregisterAll()
}

