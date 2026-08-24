package items

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/uuid"

	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
	"github.com/FreekingDean/gojellyfin/internal/transcode"
)

// One line of a client's device profile: a container together with the codecs
// it may carry inside that container. The three are a triple, so which of them
// a source fails is what decides whether playing it costs nothing, a mux, or an
// encode.
type Profile struct {
	Container  string
	VideoCodec string
	AudioCodec string
}

// What a client said it can decode. No profiles is silence rather than a
// refusal: a client that declared nothing is handed its source untouched.
type Capabilities struct {
	Profiles []Profile
	Codecs   []CodecCondition
}

// What has to change about a file before this client can play it, cheapest
// first. The last two are ranked but cannot be served: re-encoding a picture is
// #481, so a source needing one is ordered below every source that does not and
// then refused rather than answered with bytes nothing here can make.
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

// The one file the client is given and what it will be served as. The client is
// never told the others exist.
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

// SourceFor answers the whole of the playback question: of the files behind
// this item, which one, and what has to change about it.
//
// The taller picture is tried first and the first one that can be served wins,
// so a 4K needing its audio converted is answered before a 1080p that plays
// untouched, and a 4K the client cannot decode at all falls through to it.
func (s *Service) SourceFor(ctx context.Context, itemID uuid.UUID, can Capabilities) (Plan, error) {
	sources, err := s.MediaSources(ctx, itemID)
	if err != nil {
		return Plan{}, err
	}
	if len(sources) == 0 {
		return Plan{}, ErrNoSource
	}

	return can.best(sources)
}

// The first file that works, tallest first. There is no ranking to build,
// because a source's plan is already the least that has to change about it: the
// first source whose plan can be served is the answer, and a source needing a
// picture encode is simply passed over, which is what puts it last without
// anything having to say so.
func (c Capabilities) best(sources []*MediaSource) (Plan, error) {
	tallest := int32(0)
	for _, source := range sources {
		tallest = max(tallest, height(source))
	}

	ordered := slices.Clone(sources)
	slices.SortStableFunc(ordered, func(source, than *MediaSource) int {
		if mine, theirs := tier(source, tallest), tier(than, tallest); mine != theirs {
			return int(theirs - mine)
		}
		switch {
		case richer(source, than):
			return -1
		case richer(than, source):
			return 1
		default:
			return 0
		}
	})

	for _, source := range ordered {
		if plan := c.planFor(source); plan.Change.Available() {
			return plan, nil
		}
	}

	return Plan{}, ErrNoPlayable
}

func height(source *MediaSource) int32 {
	if picture := videoStream(source); picture != nil {
		return picture.Height
	}

	return 0
}

// A file nobody has probed has no dimensions, which is not the same as being a
// small one. It tiers with the tallest rather than below everything, so an item
// the probe has not reached is still answered.
func tier(source *MediaSource, tallest int32) int32 {
	if height := height(source); height > 0 {
		return height
	}

	return tallest
}

func (c Capabilities) planFor(source *MediaSource) Plan {
	container, video, audio := describe(source)
	plan := Plan{Source: source, Container: container, VideoCodec: video, AudioCodec: audio}

	if len(c.Profiles) == 0 {
		return plan
	}

	// A picture the client will not decode is not rescued by a different
	// container or by converting the sound beside it, so the two rungs a mux
	// can reach are not open to it at all.
	if !c.carries(video) || !c.satisfies(source) {
		plan.Change = ChangeVideo
		if !c.decodesAudio(audio) {
			plan.Change = ChangeVideoAudio
		}

		return plan
	}

	if c.takes(container, video, audio) {
		return plan
	}

	open := c.openTo(video)

	for _, profile := range open {
		if lists(profile.AudioCodec, audio) {
			plan.Change, plan.Container = ChangeContainer, profile.Container

			return plan
		}
	}

	plan.Change = ChangeAudio
	for _, profile := range open {
		if converted := transcode.AudioCodec(profile.Container); lists(profile.AudioCodec, converted) {
			plan.Container, plan.AudioCodec = profile.Container, converted

			return plan
		}
	}

	plan.Container = transcode.VideoContainer
	plan.AudioCodec = transcode.AudioCodec(plan.Container)

	return plan
}

func (c Capabilities) takes(container, video, audio string) bool {
	for _, profile := range c.Profiles {
		if lists(profile.Container, container) && lists(profile.VideoCodec, video) && lists(profile.AudioCodec, audio) {
			return true
		}
	}

	return false
}

func (c Capabilities) decodesAudio(audio string) bool {
	for _, profile := range c.Profiles {
		if lists(profile.AudioCodec, audio) {
			return true
		}
	}

	return false
}

// The containers the client declared that carry this picture and that ffmpeg
// can write down a pipe that is never seeked, in the order the client declared
// them. Empty means nothing it named can carry the picture at all.
func (c Capabilities) openTo(video string) []Profile {
	open := make([]Profile, 0, len(c.Profiles))
	for _, profile := range c.Profiles {
		if !lists(profile.VideoCodec, video) {
			continue
		}
		for _, container := range strings.Split(profile.Container, ",") {
			if container = strings.TrimSpace(container); transcode.CarriesVideo(container) {
				open = append(open, Profile{
					Container:  strings.ToLower(container),
					VideoCodec: profile.VideoCodec,
					AudioCodec: profile.AudioCodec,
				})
			}
		}
	}
	if len(open) == 0 {
		return []Profile{{Container: transcode.VideoContainer, AudioCodec: c.audioWith(video)}}
	}

	return open
}

func (c Capabilities) carries(video string) bool {
	for _, profile := range c.Profiles {
		if lists(profile.VideoCodec, video) {
			return true
		}
	}

	return false
}

// What the client said it can decode alongside this picture. It is what the
// fallback container has to be held to: a client that named only opus beside
// vp9 is not one that takes ac3 because the container it named cannot be
// written. One profile naming no audio at all is silence, and silence covers
// everything the rest of them list.
func (c Capabilities) audioWith(video string) string {
	declared := make([]string, 0, len(c.Profiles))
	for _, profile := range c.Profiles {
		if !lists(profile.VideoCodec, video) {
			continue
		}
		if profile.AudioCodec == "" {
			return ""
		}
		declared = append(declared, profile.AudioCodec)
	}

	return strings.Join(declared, ",")
}

// Silence on either side passes: a client that named no codecs named none it
// refuses, and a stream nobody probed is not held to a name nobody read.
func lists(declared, value string) bool {
	if declared == "" || value == "" {
		return true
	}

	for _, entry := range strings.Split(declared, ",") {
		if strings.EqualFold(strings.TrimSpace(entry), value) {
			return true
		}
	}

	return false
}

// The three ffmpeg would map, which are the three the client has to be able to
// open and decode.
func describe(source *MediaSource) (string, string, string) {
	return SourceContainer(source), firstCodec(source, streammodal.KindVideo), firstCodec(source, streammodal.KindAudio)
}

func firstCodec(source *MediaSource, kind StreamKind) string {
	for _, stream := range source.Edges.Streams {
		if stream.Kind == kind {
			return strings.ToLower(stream.Codec)
		}
	}

	return ""
}

func SourceContainer(source *MediaSource) string {
	if source.Container != "" {
		return strings.ToLower(source.Container)
	}

	return strings.TrimPrefix(strings.ToLower(filepath.Ext(source.Path)), ".")
}
