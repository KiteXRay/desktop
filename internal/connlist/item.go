package connlist

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"sync"
	"unicode"

	"github.com/google/uuid"
	vpn "github.com/goxray/core/client"
	xrayproto "github.com/lilendian0x00/xray-knife/v3/pkg/protocol"
	xray3 "github.com/lilendian0x00/xray-knife/v3/pkg/xray"

	"github.com/KiteXRay/desktop/internal/netchart"
)

type Client interface {
	Connect(string) error
	ConnectWithMode(string, vpn.TunnelMode) error
	Disconnect(context.Context) error
	BytesRead() int
	BytesWritten() int
}

// Item is a combine that is passed (via interface segregation) throughout the system to apply
// centralized changes to connections with the smallest overhead as possible.
type Item struct {
	id         string
	label      string
	link       string
	xconfigMap map[string]string
	active     bool

	parent   *Collection
	client   Client
	recorder NetworkRecorder

	mu                sync.Mutex
	totalRead         int64
	totalWritten      int64
	lastClientRead    int64
	lastClientWritten int64
}

func newItem(label, link string, parent *Collection) (*Item, error) {
	return newItemWithID("", label, link, parent)
}

func newItemWithID(id, label, link string, parent *Collection) (*Item, error) {
	if id == "" {
		id = uuid.New().String()
	}
	itm := &Item{
		id:    id,
		label: label,
		link:  link,
	}
	if err := itm.init(); err != nil {
		return nil, err
	}
	itm.parent = parent

	return itm, nil
}

func (c *Item) ID() string {
	return c.id
}

func (c *Item) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.recorder != nil {
		c.recorder.Stop()
	}
}

func (c *Item) init() error {
	proto, err := (&xray3.Core{}).CreateProtocol(c.Link())
	if err != nil {
		return fmt.Errorf("invalid xray link: %s", err)
	}
	if err := proto.Parse(); err != nil {
		return fmt.Errorf("invalid xray link: %s", err)
	}

	c.xconfigMap, err = c.xrayBaseConfigToMap(proto)
	if err != nil {
		return fmt.Errorf("parse xray config to map: %s", err)
	}

	cl, err := vpn.NewClientWithOpts(vpn.Config{
		Logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	if err != nil {
		return fmt.Errorf("create vpn client: %v", err)
	}
	c.client = cl

	c.recorder = netchart.NewRecorder(c.client)
	c.recorder.Start()

	return nil
}

func (c *Item) Update(link, label string) error {
	c.Close()
	c.link = link
	c.label = label
	if err := c.init(); err != nil {
		return err
	}
	if c.parent != nil {
		c.parent.onChange()
	}
	return nil
}

func (c *Item) Active() bool {
	return c.active
}

func (c *Item) SetActive(active bool) {
	c.active = active
	if c.parent != nil {
		c.parent.onChange()
	}
}

func (c *Item) Connect() error {
	return c.client.Connect(c.Link())
}

func (c *Item) ConnectWithMode(mode vpn.TunnelMode) error {
	return c.client.ConnectWithMode(c.Link(), mode)
}

func (c *Item) Disconnect() error {
	return c.client.Disconnect(context.Background())
}

func (c *Item) Label() string {
	return c.label
}

func (c *Item) Link() string {
	return c.link
}

func (c *Item) XRayConfig() map[string]string {
	return c.xconfigMap
}

func (c *Item) xrayBaseConfigToMap(proto xrayproto.Protocol) (map[string]string, error) {
	x := proto.ConvertToGeneralConfig()
	fmt.Printf("xrayBaseConfigToMap: %+v\n", x)
	xmap := map[string]string{
		"Protocol": x.Protocol, "Address": x.Address,
		"Security": x.Security, "Aid": x.Aid, "Host": x.Host,
		"ID": x.ID, "Network": x.Network, "Path": x.Path,
		"Port": x.Port, "Remark": x.Remark, "TLS": x.TLS,
		"SNI": x.SNI, "ALPN": x.ALPN, "TlsFingerprint": x.TlsFingerprint,
		"Authority": x.Authority, "ServiceName": x.ServiceName,
		"Mode": x.Mode, "Type": x.Type,
		"OrigLink": x.OrigLink,
	}

	// Marshalling will marshall the actual protocol, like proto.(*xray.Vmess)
	b, err := json.Marshal(proto)
	if err != nil {
		return nil, fmt.Errorf("marshal xray protocol: %w", err)
	}

	// Unmarshalling it will add protocol-specific values to the map.
	if err := json.Unmarshal(b, &xmap); err != nil {
		return nil, fmt.Errorf("unmarshal xray protocol: %w", err)
	}

	// Keys that duplicate base protocol values.
	removeDupKeys := []string{"add", "ps", "sni", "fp", "id", "OrigLink"}

	// Make all keys in map start with uppercase letter and remove duplicate keys.
	for k, _ := range xmap {
		if slices.Contains(removeDupKeys, k) {
			delete(xmap, k)
			continue
		}

		capitalized := titleCase(k)
		if capitalized != k {
			xmap[capitalized] = xmap[k]
			delete(xmap, k)
		}
	}

	return xmap, nil
}

func titleCase(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func (c *Item) SetPersistedTraffic(read, written int64) {
	c.mu.Lock()
	c.totalRead = read
	c.totalWritten = written
	c.mu.Unlock()
}

func (c *Item) sampleTraffic() {
	c.mu.Lock()
	defer c.mu.Unlock()

	var curRead, curWritten int64
	if c.client != nil {
		curRead = int64(c.client.BytesRead())
		curWritten = int64(c.client.BytesWritten())
	}
	if curRead == 0 && curWritten == 0 && c.recorder != nil {
		curRead = int64(c.recorder.BytesRead())
		curWritten = int64(c.recorder.BytesWritten())
	}

	if curRead >= c.lastClientRead {
		c.totalRead += (curRead - c.lastClientRead)
	} else {
		c.totalRead += curRead
	}
	c.lastClientRead = curRead

	if curWritten >= c.lastClientWritten {
		c.totalWritten += (curWritten - c.lastClientWritten)
	} else {
		c.totalWritten += curWritten
	}
	c.lastClientWritten = curWritten
}

func (c *Item) ResetTraffic() {
	c.mu.Lock()
	c.totalRead = 0
	c.totalWritten = 0
	var curRead, curWritten int64
	if c.client != nil {
		curRead = int64(c.client.BytesRead())
		curWritten = int64(c.client.BytesWritten())
	}
	if curRead == 0 && curWritten == 0 && c.recorder != nil {
		curRead = int64(c.recorder.BytesRead())
		curWritten = int64(c.recorder.BytesWritten())
	}
	c.lastClientRead = curRead
	c.lastClientWritten = curWritten
	c.mu.Unlock()

	if c.parent != nil {
		c.parent.onChange()
	}
}

func (c *Item) TotalBytes() int64 {
	return c.BytesRead() + c.BytesWritten()
}
