package chromecast

import (
	"net/url"
	"testing"
	"time"

	"github.com/stupside/castor/internal/media"
)

// TestRemuxOptions locks the container-change contract: the remux targets mp4
// and leaves the video encoder nil so the bitstream is stream-copied (only the
// container changes). Audio is resolved separately, so it is unset here.
func TestRemuxOptions(t *testing.T) {
	u, _ := url.Parse("https://example.com/movie.mkv")
	resolved := &media.Stream{URL: u, ContentType: media.MKV}

	opts := remuxOptions(resolved, 30*time.Second)

	if opts.OutputFormat != "mp4" {
		t.Errorf("output format = %q, want mp4", opts.OutputFormat)
	}
	if opts.VideoEncoder != nil {
		t.Errorf("remux must stream-copy video (nil encoder), got %v", opts.VideoEncoder)
	}
	if opts.SourceContentType != media.MKV {
		t.Errorf("source content type = %q, want %q", opts.SourceContentType, media.MKV)
	}
	if opts.AudioCodec != "" {
		t.Errorf("audio must be left for ResolveAudio, got %q", opts.AudioCodec)
	}
	if opts.RWTimeoutMicros != (30 * time.Second).Microseconds() {
		t.Errorf("rw timeout = %d, want %d", opts.RWTimeoutMicros, (30 * time.Second).Microseconds())
	}
}
