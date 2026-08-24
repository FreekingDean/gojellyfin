package items

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
