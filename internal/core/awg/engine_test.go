package awg

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xjasonlyu/tun2socks/v2/transport/socks5"
	"github.com/goxray/core/wireguard"
)

func TestBuildIPCConfig_MorozovLaptop(t *testing.T) {
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
	cfg, err := wireguard.ParseConf(raw)
	require.NoError(t, err)

	ipc, err := BuildIPCConfig(cfg)
	require.NoError(t, err)

	assert.Contains(t, ipc, "private_key=")
	assert.Contains(t, ipc, "public_key=")
	assert.Contains(t, ipc, "preshared_key=")
	assert.Contains(t, ipc, "endpoint=206.223.242.81:51820")
	assert.Contains(t, ipc, "jc=5\n")
	assert.Contains(t, ipc, "jmin=45\n")
	assert.Contains(t, ipc, "jmax=103\n")
	assert.Contains(t, ipc, "s1=97\n")
	assert.Contains(t, ipc, "s2=21\n")
	assert.Contains(t, ipc, "h1=1050280201\n")
	assert.Contains(t, ipc, "h2=2061389574\n")
	assert.Contains(t, ipc, "h3=201794519\n")
	assert.Contains(t, ipc, "h4=820786197\n")
	assert.Contains(t, ipc, "persistent_keepalive_interval=25\n")
}

func TestBuildIPCConfig_AWG2(t *testing.T) {
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
	cfg, err := wireguard.ParseConf(raw)
	require.NoError(t, err)

	ipc, err := BuildIPCConfig(cfg)
	require.NoError(t, err)

	assert.Contains(t, ipc, "s1=125\n")
	assert.Contains(t, ipc, "s2=34\n")
	assert.Contains(t, ipc, "s3=14\n")
	assert.Contains(t, ipc, "s4=8\n")
	assert.Contains(t, ipc, "h1=461798703-1217982642\n")
	assert.Contains(t, ipc, "h2=1925142937-1999846612\n")
	assert.Contains(t, ipc, "h3=2095634297-2109695060\n")
	assert.Contains(t, ipc, "h4=2136986792-2141306962\n")
	assert.Contains(t, ipc, "i1=<r 2><b 0x858000010001000000000669636c6f756403636f6d0000010001c00c000100010000105a00044d583737>\n")
	assert.NotContains(t, ipc, "i2=")
}

func TestEngine_StartAndCloseIdempotent(t *testing.T) {
	raw := `[Interface]
PrivateKey = yAnz5TF+KmRqDCBgMW10geXdDaFnBP9TeQoHGnRzhlk=
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
PublicKey = xIxIPEwq9uvWdyNG6lwA6n86d2k4fAenJm0+vV9/wCc=
Endpoint = 198.51.100.25:51820
AllowedIPs = 0.0.0.0/0
`
	cfg, err := wireguard.ParseConf(raw)
	require.NoError(t, err)

	engine := NewEngine()
	// Pick an ephemeral port for testing
	err = engine.Start(cfg, 0)
	require.NoError(t, err)

	// Closing once should not panic
	require.NotPanics(t, func() {
		_ = engine.Close()
	})

	// Closing second time should not panic (verifying fix for close of closed channel)
	require.NotPanics(t, func() {
		_ = engine.Close()
	})
}

func TestEngine_SOCKS5Handshake(t *testing.T) {
	raw := `[Interface]
PrivateKey = yAnz5TF+KmRqDCBgMW10geXdDaFnBP9TeQoHGnRzhlk=
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
PublicKey = xIxIPEwq9uvWdyNG6lwA6n86d2k4fAenJm0+vV9/wCc=
Endpoint = 198.51.100.25:51820
AllowedIPs = 0.0.0.0/0
`
	cfg, err := wireguard.ParseConf(raw)
	require.NoError(t, err)

	engine := NewEngine()
	err = engine.Start(cfg, 0)
	require.NoError(t, err)
	defer engine.Close()

	// 1. Test UDP Associate handshake (immediate local response)
	connUDP, err := net.Dial("tcp", engine.Addr())
	require.NoError(t, err)
	defer connUDP.Close()

	_, err = socks5.ClientHandshake(connUDP, socks5.ParseAddrString("0.0.0.0:0"), socks5.CmdUDPAssociate, nil)
	require.NoError(t, err)

	// 2. Test TCP Connect handshake protocol framing
	connTCP, err := net.Dial("tcp", engine.Addr())
	require.NoError(t, err)
	defer connTCP.Close()

	targetAddr := socks5.ParseAddrString("91.189.91.57:80")
	require.NotNil(t, targetAddr)

	_, err = socks5.ClientHandshake(connTCP, targetAddr, socks5.CmdConnect, nil)
	if err != nil {
		assert.NotContains(t, err.Error(), "connection reset")
		assert.NotContains(t, err.Error(), "short buffer")
		assert.NotContains(t, err.Error(), "EOF")
	}
}


