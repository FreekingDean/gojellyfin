package transcode

import (
	"fmt"
	"net/url"
	"strconv"
)

const (
	Path        = "/transcode"
	TokenHeader = "X-Transcoder-Token"
)

func (s Spec) Query() url.Values {
	query := url.Values{}
	query.Set("path", s.Path)
	query.Set("container", s.Container)
	if s.Bitrate > 0 {
		query.Set("bitrate", strconv.FormatInt(int64(s.Bitrate), 10))
	}
	if s.StartTicks > 0 {
		query.Set("startTicks", strconv.FormatInt(s.StartTicks, 10))
	}

	return query
}

func SpecFromQuery(query url.Values) (Spec, error) {
	spec := Spec{
		Path:      query.Get("path"),
		Container: query.Get("container"),
	}

	if raw := query.Get("bitrate"); raw != "" {
		bitrate, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			return Spec{}, fmt.Errorf("bad bitrate %q: %w", raw, err)
		}
		spec.Bitrate = int32(bitrate)
	}
	if raw := query.Get("startTicks"); raw != "" {
		ticks, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return Spec{}, fmt.Errorf("bad startTicks %q: %w", raw, err)
		}
		spec.StartTicks = ticks
	}

	return spec, spec.Valid()
}
