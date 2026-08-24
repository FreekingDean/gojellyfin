package items

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
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
// Resolution decides first and the cost of playing it breaks the tie, so a 4K
// that needs its audio converted beats a 1080p that plays untouched. A picture
// that has to be re-encoded is the exception: it ranks below every source that
// needs no encode, and only the smallest such source is ranked at all, because
// the encode discards the extra resolution anyway.
func (s *Service) SourceFor(ctx context.Context, itemID uuid.UUID, can Capabilities) (Plan, error) {
	sources, err := s.MediaSources(ctx, itemID)
	if err != nil {
		return Plan{}, err
	}
	if len(sources) == 0 {
		return Plan{}, ErrNoSource
	}

	plans := make([]Plan, 0, len(sources))
	for _, source := range sources {
		plans = append(plans, can.planFor(source))
	}

	best := ranked(plans)
	if !best.Change.Available() {
		return Plan{}, ErrNoPlayable
	}

	return best, nil
}

// The order the ladder is written in: a source needing no encode outranks one
// that does whatever its resolution, then the taller picture wins, then the
// cheaper change, then the richer encode of the two.
func ranked(plans []Plan) Plan {
	tallest := int32(0)
	for _, plan := range plans {
		tallest = max(tallest, plan.height())
	}

	shortest := tallest
	for _, plan := range plans {
		if !plan.Change.Available() {
			shortest = min(shortest, plan.tier(tallest))
		}
	}

	sort.SliceStable(plans, func(i, j int) bool {
		left, right := plans[i], plans[j]
		if left.Change.Available() != right.Change.Available() {
			return left.Change.Available()
		}
		if lheight, rheight := left.tier(tallest), right.tier(tallest); lheight != rheight {
			return lheight > rheight
		}
		if left.Change != right.Change {
			return left.Change < right.Change
		}

		return richer(left.Source, right.Source)
	})

	for _, plan := range plans {
		if plan.Change.Available() || plan.tier(tallest) == shortest {
			return plan
		}
	}

	return plans[0]
}

func (p Plan) height() int32 {
	for _, stream := range p.Source.Edges.Streams {
		if stream.Kind == streammodal.KindVideo {
			return stream.Height
		}
	}

	return 0
}

// A file nobody has probed has no dimensions, which is not the same as being a
// small one. It tiers with the tallest rather than below everything, so an item
// the probe has not reached is still answered.
func (p Plan) tier(tallest int32) int32 {
	if height := p.height(); height > 0 {
		return height
	}

	return tallest
}

func (c Capabilities) planFor(source *MediaSource) Plan {
	container, video, audio := describe(source)
	plan := Plan{Source: source, Container: container, VideoCodec: video, AudioCodec: audio}

	if len(c.Profiles) == 0 || c.takes(container, video, audio) {
		return plan
	}

	open := c.openTo(video)
	if len(open) == 0 {
		plan.Change = ChangeVideo
		if !c.decodesAudio(audio) {
			plan.Change = ChangeVideoAudio
		}

		return plan
	}

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
	if len(open) == 0 && c.carries(video) {
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
