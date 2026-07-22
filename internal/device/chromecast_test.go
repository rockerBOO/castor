package device

import (
	"net"
	"testing"

	castdns "github.com/vishen/go-chromecast/dns"

	"github.com/stupside/castor/internal/media"
)

// TestChromecastCapabilitiesAudio guards the receiver profile the remux path
// reads to keep surround: Cast decodes AAC/AC-3/E-AC-3, so a 5.1 track in a
// non-accepted container (e.g. an MKV remuxed to mp4) is copy-eligible rather
// than downmixed to stereo.
func TestChromecastCapabilitiesAudio(t *testing.T) {
	caps := chromecastCapabilities
	audio := func(c media.Codec, ch int) media.ProbeInfo {
		return media.ProbeInfo{AudioCodec: c, AudioChannels: ch}
	}
	for _, c := range []media.Codec{media.CodecAAC, media.CodecAC3, media.CodecEAC3} {
		if !caps.SupportsAudioCodec(c) {
			t.Errorf("Chromecast should advertise %q audio", c)
		}
	}
	if !caps.CanCopyAudio(audio(media.CodecAC3, 6)) {
		t.Error("5.1 AC-3 should be copy-eligible on Chromecast")
	}
	if !caps.CanCopyAudio(audio(media.CodecAAC, 6)) {
		t.Error("Chromecast decodes multichannel AAC up to 5.1, so 5.1 AAC should copy")
	}
	if caps.CanCopyAudio(audio(media.CodecAAC, 8)) {
		t.Error("Cast tops out at 5.1 AAC, so 7.1 AAC must not be copy-eligible")
	}
	// A codec Cast can't decode in mp4 (e.g. DTS) is not copy-eligible; the
	// remux re-encodes it instead of shipping a track that would black-screen.
	if caps.CanCopyAudio(audio("dts", 6)) {
		t.Error("unadvertised DTS must not be copy-eligible")
	}
}

func TestChromecastInfo(t *testing.T) {
	tests := []struct {
		name  string
		entry castdns.CastEntry
		want  Info
		ok    bool
	}{
		{
			name:  "ipv4 with friendly name on default port",
			entry: castdns.CastEntry{DeviceName: "Office Display", AddrV4: net.ParseIP("192.0.2.10"), Port: chromecastPort},
			want:  Info{Name: "Office Display", Type: TypeChromecast, Address: "192.0.2.10"},
			ok:    true,
		},
		{
			name:  "cast group on non-default port keeps host:port",
			entry: castdns.CastEntry{DeviceName: "Speakers", AddrV4: net.ParseIP("192.0.2.20"), Port: 32541},
			want:  Info{Name: "Speakers", Type: TypeChromecast, Address: "192.0.2.20:32541"},
			ok:    true,
		},
		{
			name:  "ipv6 used when no ipv4 is advertised",
			entry: castdns.CastEntry{DeviceName: "Living Room", AddrV6: net.ParseIP("2001:db8::1"), Port: chromecastPort},
			want:  Info{Name: "Living Room", Type: TypeChromecast, Address: "2001:db8::1"},
			ok:    true,
		},
		{
			name:  "name falls back to the mDNS instance name",
			entry: castdns.CastEntry{Name: "Kitchen", AddrV4: net.ParseIP("192.0.2.30"), Port: chromecastPort},
			want:  Info{Name: "Kitchen", Type: TypeChromecast, Address: "192.0.2.30"},
			ok:    true,
		},
		{
			name:  "entry without an address is rejected",
			entry: castdns.CastEntry{DeviceName: "Office Display"},
			ok:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := chromecastInfo(tt.entry)
			if ok != tt.ok {
				t.Fatalf("chromecastInfo() ok = %v, want %v", ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Errorf("chromecastInfo() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
