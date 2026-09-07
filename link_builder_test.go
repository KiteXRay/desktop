package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildLinkFromMap_VLESS(t *testing.T) {
	cfg := map[string]string{
		"Protocol":       "vless",
		"Address":        "127.0.0.1",
		"Port":           "8080",
		"ID":             "h1px412i-9138-s9m5-9b86-d47d74dd8541",
		"Remark":         "TestVless",
		"Security":       "reality",
		"Network":        "tcp",
		"Flow":           "xtls-rprx-vision",
		"SNI":            "example.com",
		"TlsFingerprint": "chrome",
		"Pbk":            "4442383675fc0fb574c3e50abbe7d4c5",
		"Sid":            "0c",
		"Spx":            "/",
	}

	link, err := buildLinkFromMap(cfg)
	require.NoError(t, err)
	assert.Contains(t, link, "vless://")
	assert.Contains(t, link, "security=reality")
	assert.Contains(t, link, "sni=example.com")
	assert.Contains(t, link, "TestVless")
}

func TestBuildLinkFromMap_VMess(t *testing.T) {
	cfg := map[string]string{
		"Protocol": "vmess",
		"Address":  "127.0.0.1",
		"Port":     "443",
		"ID":       "h1px412i-9138-s9m5-9b86-d47d74dd8541",
		"Remark":   "TestVmess",
		"Network":  "ws",
		"Path":     "/ws",
	}

	link, err := buildLinkFromMap(cfg)
	require.NoError(t, err)
	assert.Contains(t, link, "vmess://")
}

func TestBuildLinkFromMap_Trojan(t *testing.T) {
	cfg := map[string]string{
		"Protocol": "trojan",
		"Address":  "127.0.0.1",
		"Port":     "443",
		"ID":       "secretpassword",
		"Remark":   "TestTrojan",
		"SNI":      "example.com",
	}

	link, err := buildLinkFromMap(cfg)
	require.NoError(t, err)
	assert.Contains(t, link, "trojan://secretpassword@127.0.0.1:443")
}

func TestBuildLinkFromMap_Wireguard(t *testing.T) {
	cfg := map[string]string{
		"Protocol":     "wireguard",
		"Address":      "198.51.100.1",
		"Port":         "51820",
		"SecretKey":    "yAnz5TF+KmRqDCBgMW10geXdDaFnBP9TeQoHGnRzhlk=",
		"PublicKey":    "xIxIPEwq9uvWdyNG6lwA6n86d2k4fAenJm0+vV9/wCc=",
		"LocalAddress": "10.0.0.2/32",
		"MTU":          "1420",
		"Remark":       "TestWireguard",
	}

	link, err := buildLinkFromMap(cfg)
	require.NoError(t, err)
	assert.Contains(t, link, "wireguard://")
	assert.Contains(t, link, "198.51.100.1:51820")
	assert.Contains(t, link, "publickey=xIxIPEwq9uvWdyNG6lwA6n86d2k4fAenJm0%2BvV9%2FwCc%3D")
	assert.Contains(t, link, "address=10.0.0.2%2F32")
	assert.Contains(t, link, "mtu=1420")
	assert.Contains(t, link, "#TestWireguard")
}

func TestBuildLinkFromMap_AmneziaWG(t *testing.T) {
	cfg := map[string]string{
		"Protocol":     "awg",
		"Address":      "206.223.242.81",
		"Port":         "51820",
		"SecretKey":    "6HOx5aRXR5CaLHcu7RaD8VYxPzPCU5TdbMbzdzUaK0g=",
		"PublicKey":    "k2Jy+Kby5V+NC/6ZTSXevPsyjcinZ/dillHc1y1BD2g=",
		"PreSharedKey": "kUpoIYx7GyKe93xKk4w9SvWBw4y64s3hIlNNm7Cv60c=",
		"LocalAddress": "10.8.0.8/24",
		"Jc":           "5",
		"Jmin":         "45",
		"Jmax":         "103",
		"S1":           "97",
		"S2":           "21",
		"H1":           "1050280201",
		"H2":           "2061389574",
		"H3":           "201794519",
		"H4":           "820786197",
		"Remark":       "morozov-laptop",
	}

	link, err := buildLinkFromMap(cfg)
	require.NoError(t, err)
	assert.Contains(t, link, "awg://")
	assert.Contains(t, link, "206.223.242.81:51820")
	assert.Contains(t, link, "h1=1050280201")
	assert.Contains(t, link, "h2=2061389574")
	assert.Contains(t, link, "#morozov-laptop")
}

func TestApp_AddAndParseMorozovLaptop(t *testing.T) {
	app := NewApp()
	confContent := `[Interface]
PrivateKey = 6HOx5aRXR5CaLHcu7RaD8VYxPzPCU5TdbMbzdzUaK0g=
Address = 10.8.0.8/24
DNS = 1.1.1.1, 1.0.0.1
Jc = 5
Jmin = 45
Jmax = 103
S1 = 97
S2 = 21
H1 = 1050280201
H2 = 2061389574
H3 = 201794519
H4 = 820786197

[Peer]
PublicKey = k2Jy+Kby5V+NC/6ZTSXevPsyjcinZ/dillHc1y1BD2g=
PresharedKey = kUpoIYx7GyKe93xKk4w9SvWBw4y64s3hIlNNm7Cv60c=
Endpoint = 206.223.242.81:51820
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = 25
`
	// 1. Test ImportWireguardConfig
	dto, err := app.ImportWireguardConfig(confContent, "morozov-laptop")
	require.NoError(t, err)
	require.NotNil(t, dto)
	assert.Equal(t, "morozov-laptop", dto.Label)
	assert.Equal(t, "awg", dto.Protocol)
	assert.Equal(t, "206.223.242.81", dto.Address)
	assert.Equal(t, "51820", dto.Port)

	// 2. Test ParseLinkPreview
	preview, err := app.ParseLinkPreview(dto.Link)
	require.NoError(t, err)
	assert.Equal(t, "awg", preview["Protocol"])
	assert.Equal(t, "1050280201", preview["H1"])
	assert.Equal(t, "2061389574", preview["H2"])
	assert.Equal(t, "5", preview["Jc"])

	// 3. Test AddConnection directly with .conf content as link
	dto2, err := app.AddConnection(confContent, "from-conf-content")
	require.NoError(t, err)
	require.NotNil(t, dto2)
	assert.Equal(t, "from-conf-content", dto2.Label)
	assert.Equal(t, "awg", dto2.Protocol)
}

func TestApp_ImportActualFile(t *testing.T) {
	filePath := "/home/evgeny/Downloads/Telegram Desktop/morozov-laptop.conf"
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Skip("test file not found")
	}
	app := NewApp()
	dto, err := app.ImportWireguardConfig(string(data), "morozov-laptop")
	require.NoError(t, err)
	require.NotNil(t, dto)
	assert.Equal(t, "morozov-laptop", dto.Label)
	assert.Equal(t, "awg", dto.Protocol)
	assert.Equal(t, "206.223.242.81", dto.Address)
	assert.Equal(t, "51820", dto.Port)
}
