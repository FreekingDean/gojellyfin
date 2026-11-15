package probe

import (
	"regexp"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/items"
	segmentmodel "github.com/FreekingDean/gojellyfin/internal/store/mediasegment"
)

var chapterKinds = []struct {
	kind  items.SegmentKind
	title *regexp.Regexp
}{
	{segmentmodel.KindRecap, regexp.MustCompile(`^(recap|previously( on)?)\b`)},
	{segmentmodel.KindIntro, regexp.MustCompile(`^(intro|introduction|opening( (credits|theme|titles))?|op|title sequence|theme song)\b`)},
	{segmentmodel.KindOutro, regexp.MustCompile(`^(outro|ending( (credits|theme))?|ed|(end |closing )?credits)\b`)},
	{segmentmodel.KindPreview, regexp.MustCompile(`^(preview|next (episode|time|on))\b`)},
	{segmentmodel.KindCommercial, regexp.MustCompile(`^(commercial|advert(isement)?|ad break)\b`)},
}

func chapterSegments(chapters []ffmpeg.Chapter) []*items.MediaSegment {
	var found []*items.MediaSegment
	seen := map[items.SegmentKind]bool{}

	for _, chapter := range chapters {
		kind, ok := chapterKind(chapter.Tags["title"])
		if !ok || seen[kind] || chapter.EndTime <= chapter.StartTime {
			continue
		}

		seen[kind] = true
		found = append(found, &items.MediaSegment{
			Kind:       kind,
			Origin:     segmentmodel.OriginChapter,
			StartTicks: int64(chapter.StartTime * ticksPerSecond),
			EndTicks:   int64(chapter.EndTime * ticksPerSecond),
		})
	}

	return found
}

func chapterKind(title string) (items.SegmentKind, bool) {
	title = strings.ToLower(strings.TrimSpace(title))
	for _, candidate := range chapterKinds {
		if candidate.title.MatchString(title) {
			return candidate.kind, true
		}
	}

	return "", false
}
