package items

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"

	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
	"github.com/FreekingDean/gojellyfin/internal/transcode"
)

type Profile struct {
	Container  string
	VideoCodec string
	AudioCodec string
}

type Condition struct {
	Codec    string
	Property string
	Verb     string
	Value    string
}

type Capabilities struct {
	Profiles   []Profile
	Conditions []Condition
}

type Change int

const (
	ChangeNone Change = iota
	ChangeContainer
	ChangeAudio
	ChangeVideo
	ChangeVideoAudio
)

func (c Change) Available() bool {
	return c < ChangeVideo
}

type Plan struct {
	Source     *MediaSource
	Change     Change
	Container  string
	VideoCodec string
	AudioCodec string
}

var (
	ErrNoSource   = errors.New("the item has no file to play")
	ErrNoPlayable = errors.New("no file can be played as this client asked")
)

func (s *Service) SourceFor(ctx context.Context, itemID uuid.UUID, can Capabilities) (Plan, error) {
	sources, err := s.MediaSources(ctx, itemID)
	if err != nil {
		return Plan{}, err
	}
	if len(sources) == 0 {
		return Plan{}, ErrNoSource
	}

	ordered := slices.Clone(sources)
	slices.SortStableFunc(ordered, func(source, than *MediaSource) int {
		if taller := int(height(than) - height(source)); taller != 0 {
			return taller
		}
		if richer(source, than) {
			return -1
		}
		if richer(than, source) {
			return 1
		}

		return 0
	})

	for _, source := range ordered {
		if plan := can.plan(source); plan.Change.Available() {
			return plan, nil
		}
	}

	return Plan{}, ErrNoPlayable
}

func (c Capabilities) plan(source *MediaSource) Plan {
	picture, sound := stream(source, streammodal.KindVideo), stream(source, streammodal.KindAudio)
	video, audio := codec(picture), codec(sound)
	plan := Plan{Source: source, Container: container(source), VideoCodec: video, AudioCodec: audio}
	if len(c.Profiles) == 0 {
		return plan
	}

	decodes := true
	for _, condition := range c.Conditions {
		if picture != nil && lists(condition.Codec, video) && !condition.holds(picture) {
			decodes = false
		}
	}

	playsVideo, playsAudio := false, false
	copyInto, encodeInto := "", ""
	for _, profile := range c.Profiles {
		carries, keeps := decodes && lists(profile.VideoCodec, video), lists(profile.AudioCodec, audio)
		playsVideo, playsAudio = playsVideo || carries, playsAudio || keeps
		if !carries {
			continue
		}

		for _, name := range strings.Split(profile.Container, ",") {
			name = strings.ToLower(strings.TrimSpace(name))
			if name == plan.Container && keeps {
				return plan
			}
			if !transcode.CarriesVideo(name) {
				name = transcode.VideoContainer
			}
			if keeps && copyInto == "" {
				copyInto = name
			}
			if encodeInto == "" && lists(profile.AudioCodec, transcode.AudioCodec(name)) {
				encodeInto = name
			}
		}
	}

	switch {
	case !playsVideo:
		plan.Change = ChangeVideo
		if !playsAudio {
			plan.Change = ChangeVideoAudio
		}
	case copyInto != "":
		plan.Change, plan.Container = ChangeContainer, copyInto
	default:
		if encodeInto == "" {
			encodeInto = transcode.VideoContainer
		}
		plan.Change, plan.Container, plan.AudioCodec = ChangeAudio, encodeInto, transcode.AudioCodec(encodeInto)
	}

	return plan
}

func (c Condition) holds(picture *MediaStream) bool {
	named, ceiling := "", float64(0)
	switch c.Property {
	case "VideoRangeType":
		if picture.VideoRangeType != streammodal.VideoRangeTypeUnknown {
			named = string(picture.VideoRangeType)
		}
	case "VideoProfile":
		named = picture.Profile
	case "IsInterlaced":
		named = strconv.FormatBool(picture.IsInterlaced)
	case "IsAnamorphic":
		named = strconv.FormatBool(picture.IsAnamorphic)
	case "VideoLevel":
		ceiling = picture.Level
	case "Width":
		ceiling = float64(picture.Width)
	case "Height":
		ceiling = float64(picture.Height)
	case "VideoBitrate":
		ceiling = float64(picture.BitRate)
	default:
		return true
	}

	switch c.Verb {
	case "EqualsAny":
		return lists(c.Value, named)
	case "NotEquals":
		return named == "" || !strings.EqualFold(strings.TrimSpace(c.Value), named)
	case "LessThanEqual":
		limit, err := strconv.ParseFloat(strings.TrimSpace(c.Value), 64)

		return ceiling == 0 || err != nil || ceiling <= limit
	default:
		return true
	}
}

func lists(declared, value string) bool {
	if declared == "" || value == "" {
		return true
	}

	for _, entry := range strings.FieldsFunc(declared, func(r rune) bool { return r == ',' || r == '|' }) {
		if strings.EqualFold(strings.TrimSpace(entry), value) {
			return true
		}
	}

	return false
}

func stream(source *MediaSource, kind StreamKind) *MediaStream {
	for _, candidate := range source.Edges.Streams {
		if candidate.Kind == kind {
			return candidate
		}
	}

	return nil
}

func codec(stream *MediaStream) string {
	if stream == nil {
		return ""
	}

	return strings.ToLower(stream.Codec)
}

func height(source *MediaSource) int32 {
	if picture := stream(source, streammodal.KindVideo); picture != nil {
		return picture.Height
	}

	return 0
}

func container(source *MediaSource) string {
	if source.Container != "" {
		return strings.ToLower(source.Container)
	}

	return strings.TrimPrefix(strings.ToLower(filepath.Ext(source.Path)), ".")
}
