package media

// Codec is an ffmpeg canonical codec name: the value ffprobe reports for a
// stream and the vocabulary the planner, renderer capabilities, and encoders
// all share, so codecs travel as a type instead of bare strings. It names the
// abstract codec (H.264, AC-3), not a concrete encoder; one codec can have
// several encoders (libx264, h264_vaapi, h264_videotoolbox). The type spans
// both video and audio; VideoSupport / AudioSupport disambiguate which side a
// value belongs to.
type Codec string

// Video codecs.
const (
	CodecH264 Codec = "h264"
	CodecHEVC Codec = "hevc"
)

// Audio codecs. AC-3 and E-AC-3 are the Dolby surround codecs: unlike AAC (which
// a renderer commonly decodes stereo-only), advertised support for either means
// the renderer decodes multichannel, so a 5.1 source can reach it intact.
const (
	CodecAAC  Codec = "aac"
	CodecAC3  Codec = "ac3"
	CodecEAC3 Codec = "eac3"
)
