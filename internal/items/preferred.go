package items

func BestSource(sources []*MediaSource) *MediaSource {
	var best *MediaSource
	for _, source := range sources {
		if best == nil || richer(source, best) {
			best = source
		}
	}

	return best
}

func richer(source, than *MediaSource) bool {
	if source.Bitrate != than.Bitrate {
		return source.Bitrate > than.Bitrate
	}

	return source.Size > than.Size
}
