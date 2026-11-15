package items

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/store/entities"
	streammodel "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
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
	Profiles     []Profile
	Conditions   []Condition
	Audio        []Profile
	AudioEncodes []string
}

type Change int

const (
	ChangeNone Change = iota
	ChangeContainer
	ChangeAudio
	ChangeVideo
	ChangeVideoAudio
	ChangeRefused
)

func (c Change) Available() bool {
	return c < ChangeRefused
}

func (c Change) EncodesVideo() bool {
	return c == ChangeVideo || c == ChangeVideoAudio
}

func (c Change) EncodesAudio() bool {
	return c != ChangeNone
}

type Reason string

const (
	ReasonContainer  Reason = "ContainerNotSupported"
	ReasonVideoCodec Reason = "VideoCodecNotSupported"
	ReasonAudioCodec Reason = "AudioCodecNotSupported"
	ReasonRange      Reason = "VideoRangeTypeNotSupported"
	ReasonProfile    Reason = "VideoProfileNotSupported"
	ReasonLevel      Reason = "VideoLevelNotSupported"
	ReasonInterlaced Reason = "InterlacedVideoNotSupported"
	ReasonAnamorphic Reason = "AnamorphicVideoNotSupported"
	ReasonResolution Reason = "VideoResolutionNotSupported"
	ReasonBitrate    Reason = "VideoBitrateNotSupported"
)

var refusals = map[string]Reason{
	"VideoRangeType": ReasonRange,
	"VideoProfile":   ReasonProfile,
	"VideoLevel":     ReasonLevel,
	"IsInterlaced":   ReasonInterlaced,
	"IsAnamorphic":   ReasonAnamorphic,
	"Width":          ReasonResolution,
	"Height":         ReasonResolution,
	"VideoBitrate":   ReasonBitrate,
}

type Plan struct {
	Source     *MediaSource
	Change     Change
	Container  string
	AudioCodec string
	VideoCodec string
	Reasons    []Reason
}

type Playback = entities.PlaybackPlan

func (p Plan) Playback() Playback {
	playback := Playback{
		Direct:      p.Change == ChangeNone,
		VideoDirect: !p.Change.EncodesVideo(),
		AudioDirect: p.Change == ChangeNone || p.Change == ChangeContainer || p.Change == ChangeVideo,
		Container:   p.Container,
		VideoCodec:  p.VideoCodec,
		AudioCodec:  p.AudioCodec,
	}
	if playback.VideoDirect {
		playback.VideoCodec = codec(stream(p.Source, streammodel.KindVideo))
	}
	for _, reason := range p.Reasons {
		playback.Reasons = append(playback.Reasons, string(reason))
	}

	return playback
}

func (p *Plan) because(reason Reason) {
	if !slices.Contains(p.Reasons, reason) {
		p.Reasons = append(p.Reasons, reason)
	}
}

var (
	ErrNotFound   = errors.New("no such item")
	ErrNoSource   = errors.New("the item has no file to play")
	ErrNoPlayable = errors.New("no file can be played as this client asked")
)

func (s *Service) SourceFor(ctx context.Context, item *Item, can Capabilities) (Plan, error) {
	sources, err := s.MediaSources(ctx, item.ID)
	if err != nil {
		return Plan{}, err
	}
	if len(sources) == 0 {
		return Plan{}, ErrNoSource
	}

	ordered := slices.Clone(sources)
	slices.SortStableFunc(ordered, func(source, than *MediaSource) int {
		if taller := int(Height(than) - Height(source)); taller != 0 {
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

	planned, costly := can.plan, Change.EncodesVideo
	if IsAudio(item) {
		planned, costly = can.soundPlan, Change.EncodesAudio
	}

	var encode *Plan
	for _, source := range ordered {
		plan := planned(source)
		if !plan.Change.Available() {
			continue
		}
		if !costly(plan.Change) {
			return plan, nil
		}
		if encode == nil && (IsAudio(item) || Height(source) <= transcode.MaxHeight) {
			encode = &plan
		}
	}
	if encode != nil {
		return *encode, nil
	}

	return Plan{}, ErrNoPlayable
}

func (c Capabilities) plan(source *MediaSource) Plan {
	picture, sound := stream(source, streammodel.KindVideo), stream(source, streammodel.KindAudio)
	video, audio := codec(picture), codec(sound)
	plan := Plan{Source: source, Container: Container(source), AudioCodec: audio}
	if len(c.Profiles) == 0 {
		return plan
	}

	var refused []Reason
	for _, condition := range c.Conditions {
		if picture != nil && lists(condition.Codec, video) && !condition.holds(picture) {
			refused = append(refused, refusals[condition.Property])
		}
	}
	decodes := len(refused) == 0

	named, playsVideo := false, false
	copyInto, encodeInto := "", ""
	for _, profile := range c.Profiles {
		carries, keeps := decodes && lists(profile.VideoCodec, video), lists(profile.AudioCodec, audio)
		named, playsVideo = named || lists(profile.VideoCodec, video), playsVideo || carries
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
		if !named {
			plan.because(ReasonVideoCodec)
		}
		for _, reason := range refused {
			plan.because(reason)
		}

		return c.encodePlan(plan, audio)
	case copyInto != "":
		plan.Change, plan.Container = ChangeContainer, copyInto
		plan.because(ReasonContainer)
	default:
		if encodeInto == "" {
			encodeInto = transcode.VideoContainer
		}
		plan.Change, plan.Container, plan.AudioCodec = ChangeAudio, encodeInto, transcode.AudioCodec(encodeInto)
		plan.because(ReasonAudioCodec)
	}

	return plan
}

func (c Capabilities) soundPlan(source *MediaSource) Plan {
	plan := Plan{Source: source, Container: Container(source), AudioCodec: AudioCodec(source)}
	if len(c.Audio) == 0 {
		return plan
	}

	if c.decodesAudio(plan.Container, plan.AudioCodec) {
		return plan
	}

	for _, name := range c.audioTargets() {
		if !transcode.Supported(name) {
			continue
		}
		encoded := transcode.AudioCodec(name)
		if slices.Contains(c.AudioEncodes, name) || c.decodesAudio(name, encoded) {
			plan.Change, plan.Container, plan.AudioCodec = ChangeAudio, name, encoded
			plan.because(c.soundRefusal(plan.Source))

			return plan
		}
	}

	plan.Change = ChangeRefused

	return plan
}

func (c Capabilities) soundRefusal(source *MediaSource) Reason {
	for _, profile := range c.Audio {
		if lists(profile.AudioCodec, AudioCodec(source)) {
			return ReasonContainer
		}
	}

	return ReasonAudioCodec
}

func (c Capabilities) audioTargets() []string {
	targets := slices.Clone(c.AudioEncodes)
	for _, profile := range c.Audio {
		for _, name := range strings.Split(profile.Container, ",") {
			targets = append(targets, strings.ToLower(strings.TrimSpace(name)))
		}
	}

	return targets
}

func (c Capabilities) decodesAudio(container, codec string) bool {
	for _, profile := range c.Audio {
		if lists(profile.Container, container) && lists(profile.AudioCodec, codec) {
			return true
		}
	}

	return false
}

func (c Capabilities) encodePlan(plan Plan, audio string) Plan {
	copyInto, encodeInto := "", ""
	for _, profile := range c.Profiles {
		if !lists(profile.VideoCodec, transcode.VideoCodec) {
			continue
		}

		for _, name := range strings.Split(profile.Container, ",") {
			name = strings.ToLower(strings.TrimSpace(name))
			if !transcode.CarriesVideo(name) {
				name = transcode.VideoContainer
			}
			if copyInto == "" && lists(profile.AudioCodec, audio) {
				copyInto = name
			}
			if encodeInto == "" && lists(profile.AudioCodec, transcode.AudioCodec(name)) {
				encodeInto = name
			}
		}
	}

	plan.VideoCodec = transcode.VideoCodec
	switch {
	case copyInto != "":
		plan.Change, plan.Container = ChangeVideo, copyInto
	case encodeInto != "":
		plan.Change, plan.Container, plan.AudioCodec = ChangeVideoAudio, encodeInto, transcode.AudioCodec(encodeInto)
		plan.because(ReasonAudioCodec)
	default:
		plan.Change, plan.VideoCodec = ChangeRefused, ""
	}

	return plan
}

func (c Condition) holds(picture *MediaStream) bool {
	named, ceiling := "", float64(0)
	switch c.Property {
	case "VideoRangeType":
		if rangeType := deref(picture.VideoRangeType); rangeType != streammodel.VideoRangeTypeUnknown {
			named = string(rangeType)
		}
	case "VideoProfile":
		named = picture.Profile
	case "IsInterlaced":
		named = strconv.FormatBool(picture.IsInterlaced)
	case "IsAnamorphic":
		named = strconv.FormatBool(picture.IsAnamorphic)
	case "VideoLevel":
		ceiling = deref(picture.Level)
	case "Width":
		ceiling = float64(deref(picture.Width))
	case "Height":
		ceiling = float64(deref(picture.Height))
	case "VideoBitrate":
		ceiling = float64(deref(picture.BitRate))
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
	for _, candidate := range source.Streams {
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

func Height(source *MediaSource) int32 {
	if picture := stream(source, streammodel.KindVideo); picture != nil {
		return deref(picture.Height)
	}

	return 0
}

func HDRTransfer(source *MediaSource) string {
	picture := stream(source, streammodel.KindVideo)
	if picture == nil {
		return ""
	}

	switch deref(picture.VideoRangeType) {
	case "", streammodel.VideoRangeTypeUnknown, streammodel.VideoRangeTypeSDR, streammodel.VideoRangeTypeDOVIWithSDR:
		return ""
	case streammodel.VideoRangeTypeHLG, streammodel.VideoRangeTypeDOVIWithHLG:
		return transcode.TransferHLG
	default:
		return transcode.TransferPQ
	}
}

func VideoCodec(source *MediaSource) string {
	return codec(stream(source, streammodel.KindVideo))
}

func AudioCodec(source *MediaSource) string {
	return codec(stream(source, streammodel.KindAudio))
}

func Container(source *MediaSource) string {
	if source.Container != "" {
		return strings.ToLower(source.Container)
	}

	return strings.TrimPrefix(strings.ToLower(filepath.Ext(source.Path)), ".")
}

func Picture(source *MediaSource) *MediaStream {
	return stream(source, streammodel.KindVideo)
}

func Sound(source *MediaSource) *MediaStream {
	return stream(source, streammodel.KindAudio)
}
