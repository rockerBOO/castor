// Package chromecast is the cast strategy for Google Cast receivers: pass the
// source through when the device accepts its container, otherwise remux to mp4
// with the video stream-copied and the audio resolved from a probe. It
// implements core.Strategy and depends only on core and the shared building
// blocks; it never references the dlna package or the top-level cast package.
package chromecast

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/stupside/castor/internal/cast/core"
	"github.com/stupside/castor/internal/cast/ffmpeg"
	"github.com/stupside/castor/internal/device"
	"github.com/stupside/castor/internal/media"
)

// Strategy casts to a Chromecast. It connects up front because the pass-through
// decision needs the renderer's advertised containers.
type Strategy struct {
	cfg core.Config
}

// New builds the Chromecast strategy. Play wires it.
func New(cfg core.Config) *Strategy {
	return &Strategy{cfg: cfg}
}

var _ core.Strategy = (*Strategy)(nil)

// Cast connects the renderer, then either hands the source URL straight to it
// (the device accepts the container; Cast's own buffering handles pacing) or
// remuxes to mp4. Subtitles are not delivered: the go-chromecast library exposes
// no way to attach a caption track to the Load message, so the planner ships
// without them.
func (s *Strategy) Cast(ctx context.Context, resolved *media.Stream, localIP string) error {
	dev, err := core.Connect(ctx, s.cfg)
	if err != nil {
		return err
	}
	defer dev.Close()

	if dev.Capabilities().AcceptsContainer(resolved.ContentType) {
		slog.InfoContext(ctx, "execution plan", "strategy", "chromecast", "mode", "passthrough", "content_type", resolved.ContentType)
		slog.InfoContext(ctx, "starting playback", "url", resolved.URL.String(), "content_type", resolved.ContentType)
		if err := dev.Play(ctx, resolved.URL, resolved.ContentType); err != nil {
			return fmt.Errorf("starting playback: %w", err)
		}
		slog.InfoContext(ctx, "playback handed off to device")
		return nil
	}

	slog.InfoContext(ctx, "execution plan", "strategy", "chromecast", "mode", "remux", "output_content_type", media.MP4)
	return s.runRemux(ctx, resolved, dev, localIP)
}

// runRemux is the single-ffmpeg path for a container Chromecast won't take:
// network input, video stream-copied, container changed to mp4. No whisper, no
// input spool — though the output still spools so the device can replay from 0.
func (s *Strategy) runRemux(ctx context.Context, resolved *media.Stream, dev device.Device, localIP string) error {
	workDir, err := os.MkdirTemp("", "castor-")
	if err != nil {
		return fmt.Errorf("creating work directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(workDir) }()

	opts := remuxOptions(resolved, s.cfg.Transcode.RWTimeout)

	// Resolve audio from a source probe, the same decision the DLNA path makes
	// off its spool. This path has no input spool, so it reads the upstream
	// directly: one extra fetch before the remux ffmpeg opens the same URL again.
	// For the usual remux input (a static file in a container Cast won't take,
	// e.g. AVI) that is fine. It is NOT free for a short-lived or single-use
	// signed link: the probe can burn the token or a rate-limit slot and leave
	// the remux ffmpeg with a 403/429. HLS (the common signed-link shape)
	// normally passes through and never reaches here, but only while the receiver
	// advertises HLS, so this is a known, accepted risk, not a guarantee. A
	// failed probe leaves srcInfo zero, which core.ResolveAudio degrades to
	// stereo AAC.
	srcInfo, err := ffmpeg.Probe(ctx, s.cfg.Resolver.FFprobePath, resolved.URL.String(), resolved.Headers)
	if err != nil {
		slog.WarnContext(ctx, "source probe failed; will re-encode audio to stereo AAC", "error", err)
	}
	core.ResolveAudio(&opts, dev.Capabilities(), srcInfo)
	slog.InfoContext(ctx, "chromecast remux audio decision",
		"audio_codec", opts.AudioCodec,
		"source_audio_codec", string(srcInfo.AudioCodec),
		"source_audio_channels", srcInfo.AudioChannels,
	)

	proc, err := ffmpeg.Start(ctx, s.cfg.Transcode.FFmpegPath, ffmpeg.EncodeArgs(opts))
	if err != nil {
		return fmt.Errorf("starting transcode: %w", err)
	}
	defer core.FinishEncoder(ctx, proc)

	return core.ServeToDevice(ctx, dev, localIP, media.MP4, proc.Stdout, workDir)
}

// remuxOptions builds the encode options for the container-change remux: mp4
// output, the source wired up for a network read, and no video encoder so the
// video bitstream is stream-copied (Chromecast decodes the source codec). Audio
// is filled in afterward by core.ResolveAudio.
func remuxOptions(resolved *media.Stream, rwTimeout time.Duration) ffmpeg.EncodeOptions {
	return ffmpeg.EncodeOptions{
		OutputFormat:      "mp4",
		SourceURL:         resolved.URL,
		SourceHeaders:     resolved.Headers,
		SourceContentType: resolved.ContentType,
		RWTimeoutMicros:   rwTimeout.Microseconds(),
	}
}
