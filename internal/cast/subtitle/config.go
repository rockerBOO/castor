// Package subtitle holds configuration shared across the subtitle mechanisms,
// deliberately free of any mechanism implementation (and its cgo) so the pure
// planner in core can read it without importing the whisper transcriber.
package subtitle

// Whisper holds settings for the in-process whisper.cpp transcriber. It lives
// here, not in the whisper package, so core.Config can carry it (the planner
// reads Enable to choose the subtitle axis) without pulling whisper's cgo into
// the decision core. It is the sole mechanism for enabling subtitle generation;
// there is no CLI override. Set Enable: true in config.yaml (or
// CASTOR_WHISPER__ENABLE=true) to opt in. Everything else is self-managed: the
// transcription and VAD models auto-download to the user cache, and the streaming
// pipeline needs no tuning.
type Whisper struct {
	Enable    bool   `yaml:"enable"`
	ModelPath string `yaml:"model_path"` // override the auto-downloaded tiny.en model
	Language  string `yaml:"language"`   // BCP-47; pin it (per-buffer auto-detection is unstable)
}
