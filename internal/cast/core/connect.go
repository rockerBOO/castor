package core

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/stupside/castor/internal/device"
	"github.com/stupside/castor/internal/media"
	"github.com/stupside/castor/internal/source/resolve"
)

// ResolveSource runs the do-or-die prelude that doesn't depend on the renderer:
// resolve the source URL (HLS variant selection) and find our local IPv4. The
// device is discovered separately so its latency can overlap the puller.
func ResolveSource(ctx context.Context, cfg Config, stream *media.Stream) (*media.Stream, string, error) {
	slog.InfoContext(ctx, "resolving stream", "url", stream.URL.String())
	resolved, err := resolve.Resolve(ctx, cfg.Resolver, stream)
	if err != nil {
		return nil, "", fmt.Errorf("resolving URL: %w", err)
	}
	slog.InfoContext(ctx, "stream resolved", "url", resolved.URL.String(), "content_type", resolved.ContentType)

	localIP, err := localIPv4(cfg.Network.Interface)
	if err != nil {
		return nil, "", fmt.Errorf("resolving local IP: %w", err)
	}
	return resolved, localIP, nil
}

// Connect locates and connects the renderer named in cfg. cast.Play calls it
// once, up front: the plan needs the renderer's advertised capabilities
// (SelfFetch, accepted containers, decodable codecs) to fix its axes before any
// stage runs, so discovery can no longer overlap the pull the way the old
// per-device strategies arranged. device.Connect dispatches on device type
// internally, which is why nothing above here carries a device-type switch.
func Connect(ctx context.Context, cfg Config) (device.Device, error) {
	info, err := resolveInfo(ctx, cfg)
	if err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "device found", "name", info.Name, "type", string(info.Type), "address", info.Address)

	dev, err := device.Connect(ctx, info, cfg.Device)
	if err != nil {
		return nil, fmt.Errorf("connecting to device: %w", err)
	}
	slog.InfoContext(ctx, "connected to device", "name", info.Name)
	return dev, nil
}

// resolveInfo turns the configured target into a connectable Info. A pinned
// address (cfg.Device.Address) is honoured directly and skips discovery: the
// renderer is reached by unicast, so a cast works where SSDP/mDNS multicast does
// not (routed segments, or Android/Termux where interface enumeration is denied)
// and pays no discovery window. Absent an address it falls back to discovery,
// matching the device by name and type within the network timeout.
func resolveInfo(ctx context.Context, cfg Config) (device.Info, error) {
	if cfg.Device.Address != "" {
		slog.InfoContext(ctx, "pinning device by address", "name", cfg.Device.Name, "type", string(cfg.Device.Type), "address", cfg.Device.Address)
		ctx, cancel := context.WithTimeout(ctx, cfg.Network.Timeout)
		defer cancel()
		info, err := device.Locate(ctx, cfg.Device.Type, cfg.Device.Name, cfg.Device.Address)
		if err != nil {
			return device.Info{}, fmt.Errorf("pinning device: %w", err)
		}
		return info, nil
	}

	slog.InfoContext(ctx, "discovering device", "name", cfg.Device.Name, "type", string(cfg.Device.Type))
	info, err := device.FindInfo(ctx, cfg.Network.Timeout, cfg.Device.Type, cfg.Device.Name)
	if err != nil {
		return device.Info{}, fmt.Errorf("finding device: %w", err)
	}
	return info, nil
}

// localIPv4 returns the IPv4 address the local stream server should bind:
// the named interface's address, or, when name is empty, the source address
// of the default route. The UDP "connect" performs route selection only —
// no packet is sent.
func localIPv4(ifaceName string) (string, error) {
	if ifaceName == "" {
		conn, err := net.Dial("udp4", "8.8.8.8:53")
		if err != nil {
			return "", fmt.Errorf("detecting default-route address (set network.interface to pin one): %w", err)
		}
		defer conn.Close()
		return conn.LocalAddr().(*net.UDPAddr).IP.String(), nil
	}

	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return "", fmt.Errorf("looking up interface %q: %w", ifaceName, err)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "", fmt.Errorf("listing addresses on %s: %w", iface.Name, err)
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		if ip := ipNet.IP.To4(); ip != nil && !ip.IsLoopback() {
			return ip.String(), nil
		}
	}
	return "", fmt.Errorf("no IPv4 address on %s", iface.Name)
}
