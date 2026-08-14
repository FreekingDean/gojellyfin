package items

import (
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
)

// Playable is what a client said it can take. A nil set is silence rather than
// a refusal: the client named no restriction, so none is imposed.
type Playable struct {
	Containers  map[string]bool
	VideoCodecs map[string]bool
	AudioCodecs map[string]bool
}

// PreferredSource picks the file to play. Compatibility wins over quality,
// because the best encode a client cannot decode is worse than the worse one it
// can — a 4K HEVC beside a 1080p H.264 is exactly that choice, and answering it
// is the whole reason an item holds more than one file.
//
// Nothing playable still returns the best source, so the caller can remux it or
// refuse it with a status rather than a nil.
func PreferredSource(sources []*MediaSource, can Playable) *MediaSource {
	if len(sources) == 0 {
		return nil
	}

	var best *MediaSource
	for _, source := range sources {
		if !can.plays(source) {
			continue
		}
		if best == nil || richer(source, best) {
			best = source
		}
	}
	if best != nil {
		return best
	}

	return BestSource(sources)
}

// BestSource is the highest quality file regardless of what a client can play,
// which is what a download wants: the bytes are being saved rather than decoded
// here, so compatibility is the far end's problem.
func BestSource(sources []*MediaSource) *MediaSource {
	var best *MediaSource
	for _, source := range sources {
		if best == nil || richer(source, best) {
			best = source
		}
	}

	return best
}

// Bitrate first, because it survives a source whose size was never probed.
func richer(source, than *MediaSource) bool {
	if source.Bitrate != than.Bitrate {
		return source.Bitrate > than.Bitrate
	}

	return source.Size > than.Size
}

func (p Playable) plays(source *MediaSource) bool {
	if p.Containers != nil && !p.Containers[source.Container] {
		return false
	}

	for _, stream := range source.Edges.Streams {
		switch stream.Kind {
		case streammodal.KindVideo:
			if p.VideoCodecs != nil && !p.VideoCodecs[stream.Codec] {
				return false
			}
		case streammodal.KindAudio:
			if p.AudioCodecs != nil && !p.AudioCodecs[stream.Codec] {
				return false
			}
		}
	}

	return true
}
