package core

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stupside/castor/internal/cast/ffmpeg"
	"github.com/stupside/castor/internal/media"
	"github.com/stupside/castor/internal/source/resolve"
)

// TestSelectVideoEncoder pins the re-encode codec ladder: prefer the most
// efficient codec the renderer advertises AND this host can hardware-encode,
// never live-software-HEVC, and always fall to H.264. It hoisted from the old
// DLNA strategy test alongside selectVideoEncoder itself; the fake selectEncoder
// keeps it host-independent.
func TestSelectVideoEncoder(t *testing.T) {
	renderer := func(codecs ...media.Codec) media.Renderer {
		var r media.Renderer
		for _, c := range codecs {
			r.Video = append(r.Video, media.VideoSupport{Codec: c})
		}
		return r
	}
	fixed := func(m map[media.Codec]ffmpeg.Encoder) func(media.Codec) (ffmpeg.Encoder, bool) {
		return func(c media.Codec) (ffmpeg.Encoder, bool) { e, ok := m[c]; return e, ok }
	}

	hevcHW := ffmpeg.Encoder{Name: "hevc_videotoolbox", Codec: media.CodecHEVC, Hardware: true}
	hevcSW := ffmpeg.Encoder{Name: "libx265", Codec: media.CodecHEVC}
	h264HW := ffmpeg.Encoder{Name: "h264_videotoolbox", Codec: media.CodecH264, Hardware: true}
	h264SW := ffmpeg.Encoder{Name: "libx264", Codec: media.CodecH264}

	tests := []struct {
		name  string
		caps  media.Renderer
		avail map[media.Codec]ffmpeg.Encoder
		want  string
	}{
		{
			name:  "HEVC renderer with hardware HEVC picks HEVC",
			caps:  renderer(media.CodecHEVC, media.CodecH264),
			avail: map[media.Codec]ffmpeg.Encoder{media.CodecHEVC: hevcHW, media.CodecH264: h264HW},
			want:  "hevc_videotoolbox",
		},
		{
			name:  "HEVC renderer with only software HEVC falls to H.264 (never software HEVC live)",
			caps:  renderer(media.CodecHEVC, media.CodecH264),
			avail: map[media.Codec]ffmpeg.Encoder{media.CodecHEVC: hevcSW, media.CodecH264: h264HW},
			want:  "h264_videotoolbox",
		},
		{
			name:  "H.264-only renderer uses H.264 even when hardware HEVC exists",
			caps:  renderer(media.CodecH264),
			avail: map[media.Codec]ffmpeg.Encoder{media.CodecHEVC: hevcHW, media.CodecH264: h264SW},
			want:  "libx264",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := selectVideoEncoder(tt.caps, fixed(tt.avail)); got.Name != tt.want {
				t.Errorf("selectVideoEncoder = %q, want %q", got.Name, tt.want)
			}
		})
	}
}

// TestPreferredVideoCodecsHaveTargets guards the coupling between the codec
// ladder and the bitrate map: a preferred codec with no target would transcode
// unbounded (no -maxrate), silently reintroducing the rebuffering these fix.
func TestPreferredVideoCodecsHaveTargets(t *testing.T) {
	for _, c := range codecPreference {
		if _, ok := videoTargets[c]; !ok {
			t.Errorf("codec %q is in codecPreference but has no videoTargets entry", c)
		}
	}
}

// TestResolveVideo pins the copy-vs-encode gate the served path runs against a
// probe: stream-copy only when nothing forces a re-encode (no burn-in, under the
// height ceiling, renderer decodes the source envelope), else re-encode to a
// VBV-capped target. It is the whole-function counterpart to TestSelectVideoEncoder
// (which pins only the codec ladder), and heir to the copy-vs-encode assertions
// the deleted DLNA pipeline test carried.
//
// The re-encode rows resolve a concrete encoder through the real SelectEncoder
// (ResolveVideo wires it directly, not through the injectable seam), which proves
// each candidate with a one-frame test encode, so those rows need a working
// ffmpeg and skip without one. The copy row is pure and always runs.
func TestResolveVideo(t *testing.T) {
	// A renderer that natively decodes 8-bit H.264: nil Profiles means "any
	// profile" and nil BitDepths means "8-bit", so the copyable source below
	// matches and is stream-copy eligible.
	h264Renderer := media.Renderer{Video: []media.VideoSupport{{Codec: media.CodecH264}}}
	// A copyable probed source: 8-bit H.264, short enough for the height ceilings
	// used below. BitDepth must be set: an unknown depth (0) is not in {8} and so
	// would itself defeat the copy, masking the axis each row means to isolate.
	copyable := media.ProbeInfo{VideoCodec: media.CodecH264, VideoHeight: 720, VideoBitDepth: 8}

	t.Run("copyable source within the height cap is stream-copied", func(t *testing.T) {
		var opts ffmpeg.EncodeOptions
		cfg := Config{Resolver: resolve.Config{MaxHeight: 1080}}

		ResolveVideo(context.Background(), &opts, h264Renderer, copyable, cfg)

		if opts.VideoEncoder != nil {
			t.Fatalf("expected stream-copy (nil encoder), got %q", opts.VideoEncoder.Name)
		}
		if opts.VideoBitrate != "" {
			t.Errorf("a stream-copy must not set a video bitrate, got %q", opts.VideoBitrate)
		}
	})

	// Each row flips exactly one input that must force the re-encode branch,
	// isolating that trigger: the height gate, the burn-in signal, and an
	// undecodable source codec.
	reencode := []struct {
		name    string
		caps    media.Renderer
		src     media.ProbeInfo
		maxH    int
		hasSubs bool
	}{
		{"a source above the height cap re-encodes", h264Renderer, copyable, 480, false},
		{"a burn-in forces a re-encode even when the source is copyable", h264Renderer, copyable, 1080, true},
		{"a codec the renderer cannot decode re-encodes", h264Renderer, media.ProbeInfo{VideoCodec: media.CodecHEVC, VideoHeight: 720, VideoBitDepth: 8}, 1080, false},
	}
	for _, tt := range reencode {
		t.Run(tt.name, func(t *testing.T) {
			ffmpegPath := requireFFmpeg(t)

			var opts ffmpeg.EncodeOptions
			if tt.hasSubs {
				// SubtitleTextFile is the signal ResolveVideo reads to force the
				// re-encode; its value only needs to be non-empty here.
				opts.SubtitleTextFile = filepath.Join(t.TempDir(), "cue.txt")
			}
			cfg := Config{
				Resolver:  resolve.Config{MaxHeight: tt.maxH},
				Transcode: TranscodeConfig{FFmpegPath: ffmpegPath},
			}

			ResolveVideo(context.Background(), &opts, tt.caps, tt.src, cfg)

			if opts.VideoEncoder == nil {
				t.Fatal("expected a re-encode (non-nil encoder), got stream-copy")
			}
			// The chosen encoder's codec must carry its VBV-capped target, or the
			// transcode would run unbounded. Which concrete encoder wins is
			// host-dependent (hardware vs software), so assert against the target
			// for the codec it produced rather than a fixed bitrate.
			target, ok := videoTargets[opts.VideoEncoder.Codec]
			if !ok {
				t.Fatalf("encoder %q produces codec %q with no videoTargets entry", opts.VideoEncoder.Name, opts.VideoEncoder.Codec)
			}
			if opts.VideoBitrate != target.bitrate {
				t.Errorf("video bitrate = %q, want %q (the %q target)", opts.VideoBitrate, target.bitrate, opts.VideoEncoder.Codec)
			}
		})
	}
}

// requireFFmpeg returns the ffmpeg path or skips: the re-encode branch of
// ResolveVideo resolves a real encoder (SelectEncoder runs a test encode), so a
// host without ffmpeg cannot exercise it.
func requireFFmpeg(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH; skipping the re-encode branch (encoder selection runs a real test encode)")
	}
	return path
}
