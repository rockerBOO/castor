// Package device discovers media renderers on the local network and speaks
// their control protocols (DLNA/UPnP AVTransport, Chromecast). The Device
// interface is deliberately small: the cast pipeline decides what to send
// (subtitles are burned in upstream), a Device only needs to fetch and play.
package device

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/stupside/castor/internal/media"
)

type Type string

const (
	TypeDLNA       Type = "dlna"
	TypeChromecast Type = "chromecast"
	TypeRoku       Type = "roku"
)

// SelfFetches reports whether a renderer of this type fetches media URLs itself
// (a smart client such as Chromecast, or Roku's channel Video node) versus only
// playing bytes castor serves to it (DLNA). It is a static property of the
// protocol, known before the network round-trip of discovery and connect, so the
// cast pipeline can decide up front that a non-self-fetching renderer will always
// serve a local spool and begin buffering the single-use source while connect runs
// concurrently. It MUST agree with the connected renderer's Capabilities().SelfFetch,
// which the copy-vs-encode stage reads once the device is in hand.
func SelfFetches(t Type) bool {
	r, ok := rendererFor(t)
	return ok && r.selfFetches()
}

type Info struct {
	Name    string
	Type    Type
	Address string
}

// Config is the resolved device target the agnostic layers carry and forward:
// which renderer to reach (Name, Type) plus an opaque Family payload. It names no
// device family, so core/cast and every other device carry it without knowing any
// one family exists. The composition root (internal/config) fills Family with the
// selected family's typed connect settings; only that family's strategy reads it,
// via a type assertion in its own connect. Family is nil for a family that needs none.
type Config struct {
	Name string
	Type Type

	Family any
}

// Device is a connected renderer, ready to play. Obtain one via Connect.
type Device interface {
	// Play points the renderer at streamURL, advertised as contentType.
	Play(ctx context.Context, streamURL *url.URL, contentType string) error

	// Capabilities reports what this renderer can play: the containers it
	// accepts as-is and the video envelopes it decodes natively. Each device
	// resolves this from itself at connect time (DLNA negotiates it over
	// GetProtocolInfo; Chromecast reports its known receiver profile), so the
	// copy-vs-encode decision follows what the renderer advertises rather than
	// an assumption baked in per device type.
	Capabilities() media.Renderer

	// StreamHeaders returns protocol-specific HTTP headers the local stream
	// server must send when this renderer fetches contentType. Nil when the
	// protocol needs none.
	StreamHeaders(contentType string) map[string]string

	Close() error
}

// renderer is one device family's strategy: everything protocol-specific about
// reaching a renderer of that family, behind a single interface. Discovering a
// device, locating a pinned one, and connecting are all family-specific, so they
// live here rather than as switches scattered across the package. The registry
// below is the one place a family is wired in and the sole dispatch table, so no
// operation carries a per-family type switch; adding a family is implementing this
// interface and registering it.
type renderer interface {
	// selfFetches reports whether the family fetches media URLs itself (a smart
	// client) rather than only playing bytes castor serves it. It is a static
	// protocol property answered without a device in hand (see SelfFetches).
	selfFetches() bool

	// discover scans the local network for devices of this family until ctx expires,
	// contributing no devices (rather than erroring) when the scan fails.
	discover(ctx context.Context) []Info

	// connect opens a session to the device at info. cfg carries the family's opaque
	// connect settings (Config.Family), asserted and read only by this family.
	connect(ctx context.Context, info Info, cfg Config) (Device, error)
}

// renderers is the family registry: the single wiring point for renderer families
// and the sole dispatch table for every family-specific operation. Registry order
// is the discovery/scan listing order.
var renderers = []struct {
	Type Type
	renderer
}{
	{TypeDLNA, dlna{}},
	{TypeChromecast, chromecast{}},
	{TypeRoku, roku{}},
}

// rendererFor returns the strategy registered for a device type.
func rendererFor(t Type) (renderer, bool) {
	for _, r := range renderers {
		if r.Type == t {
			return r.renderer, true
		}
	}
	return nil, false
}

// Connect opens a session to the renderer described by info, dispatching to its
// family strategy. cfg is forwarded blindly from upstream; only the target
// family's connect reads its own settings out of cfg.Family.
func Connect(ctx context.Context, info Info, cfg Config) (Device, error) {
	r, ok := rendererFor(info.Type)
	if !ok {
		return nil, fmt.Errorf("unknown device type: %q", info.Type)
	}
	return r.connect(ctx, info, cfg)
}

func FindInfo(ctx context.Context, timeout time.Duration, dtype Type, name string) (Info, error) {
	devices, err := Discover(ctx, timeout)
	if err != nil {
		return Info{}, err
	}
	for _, d := range devices {
		if d.Type == dtype && strings.EqualFold(d.Name, name) {
			return d, nil
		}
	}
	return Info{}, fmt.Errorf("device %q (type %s) not found", name, dtype)
}

// Discover scans the local network for renderers of every registered family in
// parallel (DLNA via SSDP MediaRenderer, Chromecast via mDNS _googlecast._tcp,
// Roku via SSDP roku:ecp), sharing one timeout window; a family that fails
// contributes no devices rather than failing the whole scan. Results follow the
// registry order.
func Discover(ctx context.Context, timeout time.Duration) ([]Info, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	found := make([][]Info, len(renderers))
	var wg sync.WaitGroup
	for i, r := range renderers {
		wg.Go(func() { found[i] = r.discover(ctx) })
	}
	wg.Wait()

	return slices.Concat(found...), nil
}
