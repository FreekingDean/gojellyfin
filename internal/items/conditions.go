package items

import (
	"strconv"
	"strings"

	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
)

// A line of a client's CodecProfiles: what it needs to be true of a picture
// before it will decode it, over and above naming the codec. Chrome names h264
// in its DirectPlayProfiles and then says here that it wants SDR, a profile it
// knows, and a level it can reach — so a codec check on its own answers an HDR
// remux with a washed out picture rather than with a different file.
type CodecCondition struct {
	Codec      string
	Conditions []Condition
}

// Only the three verbs jellyfin-web writes for video. One it does not write is
// one nothing has been tested against, and an unread rule passes rather than
// refusing a file for a sentence nobody here understands.
const (
	EqualsAny     = "EqualsAny"
	NotEquals     = "NotEquals"
	LessThanEqual = "LessThanEqual"
)

type Condition struct {
	Property string
	Verb     string
	Value    string
}

// Every condition is treated as one the client must have. IsRequired rides on
// all of them and jellyfin-web never evaluates them itself, so what the flag
// means for direct play cannot be read off the bundle; upstream uses it to
// decide what a transcode has to enforce rather than what direct play may
// ignore. Holding them all is the reading that cannot play the wrong bytes.
func (c Capabilities) satisfies(source *MediaSource) bool {
	picture := videoStream(source)
	if picture == nil {
		return true
	}

	codec := strings.ToLower(picture.Codec)
	for _, entry := range c.Codecs {
		if entry.Codec != "" && !lists(entry.Codec, codec) {
			continue
		}
		for _, condition := range entry.Conditions {
			if !condition.holds(picture) {
				return false
			}
		}
	}

	return true
}

// A property nobody probed passes, the same way a codec nobody read does: a
// file is never refused for a fact the probe did not record.
func (c Condition) holds(picture *MediaStream) bool {
	switch c.Property {
	case "VideoRangeType":
		return c.matches(rangeTypeOf(picture))
	case "VideoProfile":
		return c.matches(picture.Profile)
	case "VideoLevel":
		return c.atMost(picture.Level, picture.Level > 0)
	case "Width":
		return c.atMost(float64(picture.Width), picture.Width > 0)
	case "Height":
		return c.atMost(float64(picture.Height), picture.Height > 0)
	case "VideoBitrate":
		return c.atMost(float64(picture.BitRate), picture.BitRate > 0)
	case "IsInterlaced":
		return c.matches(strconv.FormatBool(picture.IsInterlaced))
	case "IsAnamorphic":
		return c.matches(strconv.FormatBool(picture.IsAnamorphic))
	default:
		return true
	}
}

func (c Condition) matches(value string) bool {
	if value == "" {
		return true
	}

	switch c.Verb {
	case EqualsAny:
		return anyOf(c.Value, value)
	case NotEquals:
		return !strings.EqualFold(strings.TrimSpace(c.Value), value)
	default:
		return true
	}
}

func (c Condition) atMost(value float64, known bool) bool {
	if !known || c.Verb != LessThanEqual {
		return true
	}

	ceiling, err := strconv.ParseFloat(strings.TrimSpace(c.Value), 64)
	if err != nil {
		return true
	}

	return value <= ceiling
}

// A client writes the values it accepts for one property separated by pipes,
// which is a different separator from the commas its codec lists use.
func anyOf(declared, value string) bool {
	for _, entry := range strings.Split(declared, "|") {
		if strings.EqualFold(strings.TrimSpace(entry), value) {
			return true
		}
	}

	return false
}

// Unknown is what the enum calls a range the probe recorded but could not name,
// and it has to read as unknown here rather than as a range of its own.
func rangeTypeOf(picture *MediaStream) string {
	if picture.VideoRangeType == streammodal.VideoRangeTypeUnknown {
		return ""
	}

	return string(picture.VideoRangeType)
}

func videoStream(source *MediaSource) *MediaStream {
	for _, stream := range source.Edges.Streams {
		if stream.Kind == streammodal.KindVideo {
			return stream
		}
	}

	return nil
}
