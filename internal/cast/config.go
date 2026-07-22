package cast

import (
	"github.com/stupside/castor/internal/cast/core"
	"github.com/stupside/castor/internal/cast/whisper"
)

// Config is the application-facing cast configuration: the device-neutral
// core.Config (embedded, so cfg.Device etc. read through and cfg.Config is the
// exact value the shared machinery wants) plus Whisper, a DLNA-only subtitle
// knob kept out of core so the shared core imports no device subsystem.
type Config struct {
	core.Config
	Whisper whisper.Config
}

// The neutral config parts are re-exported so the application still writes
// cast.DeviceConfig etc.; their definitions live in core.
type (
	DeviceConfig    = core.DeviceConfig
	NetworkConfig   = core.NetworkConfig
	TranscodeConfig = core.TranscodeConfig
)
