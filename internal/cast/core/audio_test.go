package core

import (
	"testing"

	"github.com/stupside/castor/internal/cast/ffmpeg"
	"github.com/stupside/castor/internal/media"
)

// TestPreferredAudioCodecsHaveTargets guards the coupling between the surround
// codec ladder and the bitrate map: a preferred codec with no target would
// silently downmix a source it should have kept in surround.
func TestPreferredAudioCodecsHaveTargets(t *testing.T) {
	for _, c := range audioCodecPreference {
		if _, ok := audioTargets[c]; !ok {
			t.Errorf("codec %q is in audioCodecPreference but has no audioTargets entry", c)
		}
	}
}

func TestResolveAudio(t *testing.T) {
	aacStereo := media.AudioSupport{Codec: media.CodecAAC, MaxChannels: 2}
	ac3 := media.AudioSupport{Codec: media.CodecAC3}
	eac3 := media.AudioSupport{Codec: media.CodecEAC3}
	caps := func(a ...media.AudioSupport) media.Renderer { return media.Renderer{Audio: a} }
	src := func(codec media.Codec, ch int) media.ProbeInfo {
		return media.ProbeInfo{AudioCodec: codec, AudioChannels: ch}
	}

	tests := []struct {
		name         string
		caps         media.Renderer
		src          media.ProbeInfo
		wantCodec    string
		wantChannels int
	}{
		{"stereo aac is copied", caps(aacStereo, ac3), src(media.CodecAAC, 2), "copy", 0},
		{"5.1 ac3 is copied intact", caps(aacStereo, ac3), src(media.CodecAC3, 6), "copy", 0},
		{"5.1 aac re-encodes to ac3, layout kept", caps(aacStereo, ac3), src(media.CodecAAC, 6), "ac3", 6},
		{"5.1 dts prefers eac3 when advertised", caps(eac3, ac3), src("dts", 6), "eac3", 6},
		{"7.1 folds to the ac3 channel ceiling", caps(ac3), src("dts", 8), "ac3", 6},
		{"7.1 folds to the eac3 5.1 ceiling (ffmpeg eac3 has no 7.1)", caps(eac3), src("dts", 8), "eac3", 6},
		{"multichannel with no surround support downmixes to stereo aac", caps(aacStereo), src("dts", 6), "aac", 2},
		{"stereo source the renderer can't copy downmixes to aac", caps(), src("mp3", 2), "aac", 2},
		{"conservative caps fall to the stereo aac floor", media.Renderer{}, media.ProbeInfo{}, "aac", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts ffmpeg.EncodeOptions
			ResolveAudio(&opts, tt.caps, tt.src)
			if opts.AudioCodec != tt.wantCodec {
				t.Errorf("audio codec = %q, want %q", opts.AudioCodec, tt.wantCodec)
			}
			if opts.AudioChannels != tt.wantChannels {
				t.Errorf("audio channels = %d, want %d", opts.AudioChannels, tt.wantChannels)
			}
			// A copy must not carry re-encode parameters; a re-encode must.
			if tt.wantCodec == "copy" && opts.AudioBitrate != "" {
				t.Errorf("copy must not set an audio bitrate, got %q", opts.AudioBitrate)
			}
			if tt.wantCodec != "copy" && opts.AudioBitrate == "" {
				t.Error("a re-encode must set an audio bitrate")
			}
		})
	}
}
