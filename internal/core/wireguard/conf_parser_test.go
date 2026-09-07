package wireguard

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConf_StandardWireguard(t *testing.T) {
	raw := `
[Interface]
PrivateKey = aaaaaa111111bbbbbb222222cccccc333333dddddd44=
Address = 10.2.0.2/32, fd00::2/128
DNS = 1.1.1.1, 8.8.8.8
MTU = 1360

[Peer]
PublicKey = xxxxxx111111yyyyyy222222zzzzzz333333wwwwww44=
PresharedKey = ppppp111111qqqqqq222222rrrrrr333333ssssss44=
Endpoint = 198.51.100.25:51820
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = 25
`

	assert.True(t, IsConfContent(raw))

	cfg, err := ParseConf(raw)
	require.NoError(t, err)
	assert.False(t, cfg.IsAmnezia)
	assert.Equal(t, "aaaaaa111111bbbbbb222222cccccc333333dddddd44=", cfg.PrivateKey)
	assert.Equal(t, "xxxxxx111111yyyyyy222222zzzzzz333333wwwwww44=", cfg.PublicKey)
	assert.Equal(t, "198.51.100.25:51820", cfg.Endpoint)
	assert.Equal(t, 1360, cfg.MTU)

	uri := cfg.ToURI("MyWG")
	assert.Contains(t, uri, "wireguard://aaaaaa111111bbbbbb222222cccccc333333dddddd44=@198.51.100.25:51820")
	assert.Contains(t, uri, "publickey=xxxxxx111111yyyyyy222222zzzzzz333333wwwwww44%3D")
	assert.Contains(t, uri, "mtu=1360")
	assert.Contains(t, uri, "#MyWG")
}

func TestParseConf_AmneziaWG(t *testing.T) {
	raw := `
[Interface]
PrivateKey = aaaaaa111111bbbbbb222222cccccc333333dddddd44=
Address = 10.2.0.2/32
Jc = 4
Jmin = 40
Jmax = 70
S1 = 15
S2 = 30
H1 = 1
H2 = 2
H3 = 3
H4 = 4

[Peer]
PublicKey = xxxxxx111111yyyyyy222222zzzzzz333333wwwwww44=
Endpoint = 198.51.100.25:51820
AllowedIPs = 0.0.0.0/0
`

	assert.True(t, IsConfContent(raw))

	cfg, err := ParseConf(raw)
	require.NoError(t, err)
	assert.True(t, cfg.IsAmnezia)
	assert.Equal(t, 4, cfg.Jc)
	assert.Equal(t, 40, cfg.Jmin)
	assert.Equal(t, 70, cfg.Jmax)
	assert.Equal(t, 15, cfg.S1)
	assert.Equal(t, 30, cfg.S2)
	assert.Equal(t, int64(1), cfg.H1)
	assert.Equal(t, int64(2), cfg.H2)
	assert.Equal(t, int64(3), cfg.H3)
	assert.Equal(t, int64(4), cfg.H4)

	uri := cfg.ToURI("MyAWG")
	assert.Contains(t, uri, "awg://")
	assert.Contains(t, uri, "jc=4")
	assert.Contains(t, uri, "s1=15")
	assert.Contains(t, uri, "h4=4")
	assert.Contains(t, uri, "#MyAWG")
}

func TestParseConf_MorozovLaptop(t *testing.T) {
	raw := `[Interface]
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
	assert.True(t, IsConfContent(raw))

	cfg, err := ParseConf(raw)
	require.NoError(t, err)
	assert.True(t, cfg.IsAmnezia)
	assert.Equal(t, "6HOx5aRXR5CaLHcu7RaD8VYxPzPCU5TdbMbzdzUaK0g=", cfg.PrivateKey)
	assert.Equal(t, "10.8.0.8/24", cfg.Address)
	assert.Equal(t, 5, cfg.Jc)
	assert.Equal(t, 45, cfg.Jmin)
	assert.Equal(t, 103, cfg.Jmax)
	assert.Equal(t, 97, cfg.S1)
	assert.Equal(t, 21, cfg.S2)
	assert.Equal(t, int64(1050280201), cfg.H1)
	assert.Equal(t, int64(2061389574), cfg.H2)
	assert.Equal(t, int64(201794519), cfg.H3)
	assert.Equal(t, int64(820786197), cfg.H4)
	assert.Equal(t, "k2Jy+Kby5V+NC/6ZTSXevPsyjcinZ/dillHc1y1BD2g=", cfg.PublicKey)
	assert.Equal(t, "kUpoIYx7GyKe93xKk4w9SvWBw4y64s3hIlNNm7Cv60c=", cfg.PreSharedKey)
	assert.Equal(t, "206.223.242.81:51820", cfg.Endpoint)

	uri := cfg.ToURI("morozov-laptop")
	assert.True(t, IsAWGLink(uri))
	assert.Contains(t, uri, "awg://")
	assert.Contains(t, uri, "h1=1050280201")
	assert.Contains(t, uri, "h2=2061389574")
	assert.Contains(t, uri, "#morozov-laptop")

	parsedCfg, remark, err := ParseLink(uri)
	require.NoError(t, err)
	assert.Equal(t, "morozov-laptop", remark)
	assert.Equal(t, cfg.H1, parsedCfg.H1)
	assert.Equal(t, cfg.H2, parsedCfg.H2)
	assert.Equal(t, cfg.H3, parsedCfg.H3)
	assert.Equal(t, cfg.H4, parsedCfg.H4)
	assert.Equal(t, cfg.Endpoint, parsedCfg.Endpoint)
}
