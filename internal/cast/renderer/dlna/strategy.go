// Package dlna is the cast strategy for UPnP/DLNA renderers: a read-once spool
// pipeline (pull → spool → probe → encode with optional whisper hardsubs →
// replay server), remuxing to MPEG-TS. It implements core.Strategy and depends
// only on core and the shared building blocks; it never references the
// chromecast package or the top-level cast package.
package dlna

import (
	"context"
	"log/slog"

	"github.com/stupside/castor/internal/cast/core"
	"github.com/stupside/castor/internal/cast/subtitle/whisper"
	"github.com/stupside/castor/internal/media"
)

// Strategy casts to a DLNA renderer. whisper is the subtitle config: a DLNA-only
// concern core knows nothing about, threaded in here rather than carried on
// core.Config. The renderer is discovered inside runSpooled, concurrently with
// the pull (SSDP is slow and the signed source URL can't wait for it).
type Strategy struct {
	cfg     core.Config
	whisper whisper.Config
}

// New builds the DLNA strategy. Play wires it.
func New(cfg core.Config, whisperCfg whisper.Config) *Strategy {
	return &Strategy{cfg: cfg, whisper: whisperCfg}
}

var _ core.Strategy = (*Strategy)(nil)

// outputContentType is the MPEG-TS container every DLNA renderer receives.
const outputContentType = "video/mp2t"

// keyframeSeconds caps the encoded GOP so a renderer joining mid-stream resyncs
// within a couple seconds.
const keyframeSeconds = 2

// videoTarget is a VBV-capped bitrate: the average, the peak cap, and the buffer
// window the cap applies over.
type videoTarget struct{ bitrate, maxrate, bufsize string }

// videoTargets is the re-encode target per codec, bounding the transcoder's
// output so it stays within the renderer's decode budget. HEVC needs about half
// of H.264 for the same quality. maxrate == bitrate makes the VBV cap a true
// ceiling rather than an average the encoder overshoots; bufsize is ~2s. These
// live in the DLNA strategy because Chromecast stream-copies video and never
// re-encodes. Adding a codec the pipeline encodes to is one entry here.
var videoTargets = map[media.Codec]videoTarget{
	media.CodecH264: {bitrate: "4M", maxrate: "4M", bufsize: "8M"},
	media.CodecHEVC: {bitrate: "2M", maxrate: "2M", bufsize: "4M"},
}

// Cast runs the read-once spool pipeline: always spool the source and remux to
// MPEG-TS. The video is stream-copied when the renderer decodes the source
// envelope natively, else re-encoded; audio is copied or re-encoded by
// core.ResolveAudio; whisper hardsubs are burned in when enabled. Subtitles are
// burned in rather than sent as a track because Samsung renderers can't be
// trusted to display DLNA-delivered caption tracks. See runSpooled.
func (s *Strategy) Cast(ctx context.Context, resolved *media.Stream, localIP string) error {
	slog.InfoContext(ctx, "execution plan",
		"strategy", "dlna",
		"output_content_type", outputContentType,
		"subtitles", s.whisper.Enable,
	)
	return s.runSpooled(ctx, resolved, localIP)
}
