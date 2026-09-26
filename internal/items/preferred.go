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
	bitrate, other := deref(source.Bitrate), deref(than.Bitrate)
	if bitrate != other {
		return bitrate > other
	}

	return deref(source.Size) > deref(than.Size)
}
