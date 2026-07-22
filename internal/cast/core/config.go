// Package core holds the device-agnostic machinery every cast strategy shares:
// config, source resolution, device discovery/connect, the audio copy-vs-encode
// decision, and the replay-server delivery. It knows nothing about DLNA or
// Chromecast; the strategy packages import core, never the other way round, and
// never each other, so a device concern physically cannot leak across the
// boundary or into this core.
package core

import (
	"time"

	"github.com/stupside/castor/internal/device"
	"github.com/stupside/castor/internal/source/resolve"
)

// Config is the device-neutral configuration every strategy shares: the target
// device, network, transcode binary, and source resolver. Device-specific knobs
// (e.g. DLNA's whisper subtitles) live in that strategy's own package, so core
// imports no device subsystem. The application config composes this with those;
// core never reads app-level state.
type Config struct {
	Device    DeviceConfig
	Network   NetworkConfig
	Transcode TranscodeConfig
	Resolver  resolve.Config
}

type DeviceConfig struct {
	Name string      `yaml:"name" validate:"required"`
	Type device.Type `yaml:"type" validate:"required"`
}

type NetworkConfig struct {
	Timeout   time.Duration `yaml:"timeout" validate:"required"`
	Interface string        `yaml:"interface"`
}

// TranscodeConfig holds the small set of ffmpeg settings that aren't decided
// by a strategy. Codec/bitrate/format choices live in the per-device strategy;
// only the binary path and the upstream I/O timeout come from config.
type TranscodeConfig struct {
	FFmpegPath string        `yaml:"ffmpeg_path" validate:"required"`
	RWTimeout  time.Duration `yaml:"rw_timeout" validate:"required"`
}
