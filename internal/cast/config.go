package cast

import (
	"github.com/stupside/castor/internal/cast/core"
	"github.com/stupside/castor/internal/cast/subtitle/whisper"
)

// Config is the application-facing cast configuration: the device-neutral
// core.Config (embedded, so cfg.Device etc. read through and cfg.Config is the
// exact value the shared machinery wants) plus Whisper, the subtitle-transcription
// knob. Whisper is kept off core.Config so the neutral core config carries only
// what the shared machinery reads; it is threaded to whichever renderer does
// subtitle work.
type Config struct {
	core.Config
	Whisper whisper.Config
}

// Every config section's type is re-exported here so the application composes
// the cast surface through this one package (cast.DeviceConfig, cast.WhisperConfig,
// ...) instead of reaching into core or the subtitle building block. The
// definitions live in the packages that consume them.
type (
	DeviceConfig    = core.DeviceConfig
	NetworkConfig   = core.NetworkConfig
	TranscodeConfig = core.TranscodeConfig
	WhisperConfig   = whisper.Config
)
