package scanner

import (
	"strings"
	"unicode"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/items"
	creditmodal "github.com/FreekingDean/gojellyfin/internal/store/credit"
)

const (
	nameDelimiters  = ";/|"
	genreDelimiters = ";/|,"
)

var studioTags = []string{"studio", "publisher"}

var creditTags = []struct {
	tag  string
	kind items.CreditKind
}{
	{"artist", creditmodal.KindArtist},
	{"albumartist", creditmodal.KindAlbumArtist},
	{"composer", creditmodal.KindComposer},
	{"conductor", creditmodal.KindConductor},
	{"lyricist", creditmodal.KindLyricist},
	{"arranger", creditmodal.KindArranger},
	{"engineer", creditmodal.KindEngineer},
	{"mixer", creditmodal.KindMixer},
	{"remixer", creditmodal.KindRemixer},
	{"director", creditmodal.KindDirector},
	{"producer", creditmodal.KindProducer},
	{"writer", creditmodal.KindWriter},
	{"writtenby", creditmodal.KindWriter},
	{"actor", creditmodal.KindActor},
}

func metadata(probe *ffmpeg.Probe) items.ContainerMetadata {
	tags := containerTags(probe)

	extracted := items.ContainerMetadata{
		Genres:  splitOn(tags["genre"], genreDelimiters),
		Tags:    splitOn(tags["keywords"], nameDelimiters),
		Studios: make([]string, 0),
		People:  make([]items.Person, 0),
	}

	for _, tag := range studioTags {
		extracted.Studios = append(extracted.Studios, splitOn(tags[tag], nameDelimiters)...)
	}
	extracted.Studios = unique(extracted.Studios)

	seen := make(map[items.Person]bool)
	for _, credit := range creditTags {
		for _, name := range splitOn(tags[credit.tag], nameDelimiters) {
			person := items.Person{Name: name, Kind: credit.kind}
			if seen[person] {
				continue
			}
			seen[person] = true
			extracted.People = append(extracted.People, person)
		}
	}

	return extracted
}

// Some containers hang the tags off the media stream rather than the format,
// and the format wins where both carry the same key.
func containerTags(probe *ffmpeg.Probe) map[string]string {
	tags := make(map[string]string)

	for _, stream := range probe.Streams {
		if stream.CodecType != "audio" && stream.CodecType != "video" {
			continue
		}
		for key, value := range stream.Tags {
			tags[tagKey(key)] = value
		}
	}
	for key, value := range probe.Format.Tags {
		tags[tagKey(key)] = value
	}

	return tags
}

func tagKey(key string) string {
	return strings.Map(func(r rune) rune {
		if r == '_' || r == '-' || r == ' ' {
			return -1
		}

		return unicode.ToLower(r)
	}, key)
}

func splitOn(value, delimiters string) []string {
	return unique(strings.FieldsFunc(value, func(r rune) bool {
		return strings.ContainsRune(delimiters, r)
	}))
}

func unique(values []string) []string {
	seen := make(map[string]bool, len(values))
	kept := make([]string, 0, len(values))

	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		kept = append(kept, trimmed)
	}

	return kept
}
