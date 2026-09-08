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
	assert.Equal(t, "1", cfg.H1)
	assert.Equal(t, "2", cfg.H2)
	assert.Equal(t, "3", cfg.H3)
	assert.Equal(t, "4", cfg.H4)

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
	assert.Equal(t, "1050280201", cfg.H1)
	assert.Equal(t, "2061389574", cfg.H2)
	assert.Equal(t, "201794519", cfg.H3)
	assert.Equal(t, "820786197", cfg.H4)
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

func TestParseLink_StandardWireguard(t *testing.T) {
	rawLink := "wireguard://gIXzCVfmpgvFDAO96zlS99ZxRjTY5g8vMor5q7Te0FI%3D@185.102.139.121:21381?publickey=HSF%2Bi9%2BLBE71WEAZeIisrPHQzH4UHCkh8uOrFpIvJzA%3D&address=10.0.0.2%2F32&mtu=1420#wg-1"
	assert.True(t, IsAWGLink(rawLink))
	assert.True(t, IsWireguardLink(rawLink))

	cfg, remark, err := ParseLink(rawLink)
	require.NoError(t, err)
	assert.Equal(t, "wg-1", remark)
	assert.False(t, cfg.IsAmnezia)
	assert.Equal(t, "gIXzCVfmpgvFDAO96zlS99ZxRjTY5g8vMor5q7Te0FI=", cfg.PrivateKey)
	assert.Equal(t, "HSF+i9+LBE71WEAZeIisrPHQzH4UHCkh8uOrFpIvJzA=", cfg.PublicKey)
	assert.Equal(t, "185.102.139.121:21381", cfg.Endpoint)
	assert.Equal(t, "10.0.0.2/32", cfg.Address)
	assert.Equal(t, 1420, cfg.MTU)

	m := cfg.ToMap(remark)
	assert.Equal(t, "wireguard", m["Protocol"])
	assert.Equal(t, "185.102.139.121", m["Address"])
	assert.Equal(t, "21381", m["Port"])
	assert.Equal(t, "1420", m["Mtu"])
	assert.Equal(t, "wg-1", m["Remark"])
}

func TestParseConf_AWG2_RangesAndObfuscation(t *testing.T) {
	raw := `[Interface]
Address = 10.8.1.32/32
DNS = 1.1.1.1, 1.0.0.1
PrivateKey = dYuAVd/JcBqCyPKSri07a2p8EBEDOD8ZhoexnCGS5HE=
Jc = 5
Jmin = 10
Jmax = 50
S1 = 125
S2 = 34
S3 = 14
S4 = 8
H1 = 461798703-1217982642
H2 = 1925142937-1999846612
H3 = 2095634297-2109695060
H4 = 2136986792-2141306962
I1 = <r 2><b 0x858000010001000000000669636c6f756403636f6d0000010001c00c000100010000105a00044d583737>
I2 = 
I3 = 
I4 = 
I5 = 

[Peer]
PublicKey = hrhczy2mUqZnXPmO9/464XnXt1iNKN5Vjhv3pO2grQM=
PresharedKey = 0PKgmUnQlCEwv0PhOncPlh5Y1HawvXJZTkTzQUVskGM=
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = 2.27.57.42:37801
PersistentKeepalive = 25
`
	assert.True(t, IsConfContent(raw))

	cfg, err := ParseConf(raw)
	require.NoError(t, err)
	assert.True(t, cfg.IsAmnezia)
	assert.Equal(t, 5, cfg.Jc)
	assert.Equal(t, 10, cfg.Jmin)
	assert.Equal(t, 50, cfg.Jmax)
	assert.Equal(t, 125, cfg.S1)
	assert.Equal(t, 34, cfg.S2)
	assert.Equal(t, 14, cfg.S3)
	assert.Equal(t, 8, cfg.S4)
	assert.Equal(t, "461798703-1217982642", cfg.H1)
	assert.Equal(t, "1925142937-1999846612", cfg.H2)
	assert.Equal(t, "2095634297-2109695060", cfg.H3)
	assert.Equal(t, "2136986792-2141306962", cfg.H4)
	assert.Equal(t, "<r 2><b 0x858000010001000000000669636c6f756403636f6d0000010001c00c000100010000105a00044d583737>", cfg.I1)
	assert.Empty(t, cfg.I2)
	assert.Empty(t, cfg.I3)
	assert.Empty(t, cfg.I4)
	assert.Empty(t, cfg.I5)

	uri := cfg.ToURI("awg2-test")
	assert.True(t, IsAWGLink(uri))
	assert.Contains(t, uri, "awg://")
	assert.Contains(t, uri, "s3=14")
	assert.Contains(t, uri, "s4=8")
	assert.Contains(t, uri, "h1=461798703-1217982642")
	assert.Contains(t, uri, "h2=1925142937-1999846612")
	assert.Contains(t, uri, "h3=2095634297-2109695060")
	assert.Contains(t, uri, "h4=2136986792-2141306962")
	assert.Contains(t, uri, "#awg2-test")

	parsedCfg, remark, err := ParseLink(uri)
	require.NoError(t, err)
	assert.Equal(t, "awg2-test", remark)
	assert.Equal(t, cfg.S3, parsedCfg.S3)
	assert.Equal(t, cfg.S4, parsedCfg.S4)
	assert.Equal(t, cfg.H1, parsedCfg.H1)
	assert.Equal(t, cfg.H2, parsedCfg.H2)
	assert.Equal(t, cfg.H3, parsedCfg.H3)
	assert.Equal(t, cfg.H4, parsedCfg.H4)
	assert.Equal(t, cfg.I1, parsedCfg.I1)
	assert.Empty(t, parsedCfg.I2)
}


