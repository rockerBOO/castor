package core

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"

	"github.com/stupside/castor/internal/cast/deliver/replay"
	"github.com/stupside/castor/internal/cast/ffmpeg"
	"github.com/stupside/castor/internal/device"
	"github.com/stupside/castor/internal/media"
)

// FinishEncoder tears down the encode process: close its output, wait for
// exit, and surface stderr forensics on a failure we didn't cause ourselves
// (a cancelled context means we killed ffmpeg, e.g. Ctrl+C — not worth
// dumping the tail for).
func FinishEncoder(ctx context.Context, proc *ffmpeg.Process) {
	_ = proc.Stdout.Close()
	if err := proc.Wait(); err != nil && ctx.Err() == nil {
		proc.LogStderrTail(ctx, "ffmpeg stderr")
		slog.WarnContext(ctx, "ffmpeg exited with error", "error", err)
	}
}

// ServeToDevice fronts stream with the replay-from-zero HTTP server, points
// the renderer at it, and blocks until the stream has been fully produced
// and delivered or ctx ends. outputContentType is what the device is told it
// is fetching (e.g. "video/mp2t" or "video/mp4").
func ServeToDevice(ctx context.Context, dev device.Device, localIP, outputContentType string, stream io.Reader, workDir string) error {
	fmtInfo, ok := media.FormatForContentType(outputContentType)
	if !ok {
		return fmt.Errorf("no format info for output content type %q", outputContentType)
	}

	srv, err := replay.New(replay.Config{
		LocalIP:     localIP,
		ContentType: fmtInfo.ContentType,
		Extension:   fmtInfo.Extension,
		Headers:     dev.StreamHeaders(fmtInfo.ContentType),
		SpoolPath:   filepath.Join(workDir, "out"+fmtInfo.Extension),
	}, stream)
	if err != nil {
		return fmt.Errorf("starting stream server: %w", err)
	}
	defer srv.Close()

	streamURL := srv.URL()
	slog.InfoContext(ctx, "starting playback", "url", streamURL.String(), "content_type", fmtInfo.ContentType)
	if err := dev.Play(ctx, streamURL, fmtInfo.ContentType); err != nil {
		return fmt.Errorf("starting playback: %w", err)
	}
	slog.InfoContext(ctx, "streaming to device, press Ctrl+C to stop")
	return srv.Wait(ctx)
}
