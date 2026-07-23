// Package cast turns a resolved media stream into playback on a renderer.
//
// It is device-agnostic: Play resolves the source, then hands off to a
// per-device Strategy chosen by device type. Each strategy is a self-contained
// module (internal/cast/renderer/dlna, internal/cast/renderer/chromecast) built
// on the shared internal/cast/core and the neutral building blocks under
// deliver/ and subtitle/; the strategies never import each other or this
// package, so a device concern cannot leak across the boundary or into the core.
package cast

import (
	"context"
	"fmt"

	"github.com/stupside/castor/internal/cast/core"
	"github.com/stupside/castor/internal/cast/renderer/chromecast"
	"github.com/stupside/castor/internal/cast/renderer/dlna"
	"github.com/stupside/castor/internal/device"
	"github.com/stupside/castor/internal/media"
)

// Play resolves a stream and casts it to the configured device. The source URL
// and local IP are resolved up front (renderer-independent); the chosen strategy
// owns everything after, including when to discover and connect the renderer.
// This switch is the one place mapping a device type to its strategy; adding a
// family is a case here plus its package, and the strategies stay isolated.
func Play(ctx context.Context, cfg Config, stream *media.Stream) error {
	resolved, localIP, err := core.ResolveSource(ctx, cfg.Config, stream)
	if err != nil {
		return err
	}

	var strategy core.Strategy
	switch cfg.Device.Type {
	case device.TypeDLNA:
		strategy = dlna.New(cfg.Config, cfg.Whisper)
	case device.TypeChromecast:
		strategy = chromecast.New(cfg.Config)
	default:
		return fmt.Errorf("unsupported device type: %q", cfg.Device.Type)
	}

	return strategy.Cast(ctx, resolved, localIP)
}
