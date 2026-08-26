package scanner

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/FreekingDean/gojellyfin/internal/items"
)

var (
	yearPattern     = regexp.MustCompile(`^(.*?)[\s._\-]*[\(\[]?((?:19|20)\d{2})[\)\]]?[\s._\-]*$`)
	episodePattern  = regexp.MustCompile(`(?i)^(.*?)[\s._\-]*(?:s(\d{1,3})[\s._\-]*e(\d{1,4})|(\d{1,2})x(\d{1,4}))[\s._\-]*(.*)$`)
	seasonPattern   = regexp.MustCompile(`(?i)^(?:season|series|s)[\s._\-]*(\d{1,3})$`)
	specialsPattern = regexp.MustCompile(`(?i)^(?:specials?|season\s*0)$`)
	articlePattern  = regexp.MustCompile(`(?i)^(the|a|an)\s+`)
	languagePattern = regexp.MustCompile(`^[a-z]{2,3}$`)
	trackPattern    = regexp.MustCompile(`^(?:(\d{1,2})[\s\-]+)?(\d{1,3})[\s\-]+(.+)$`)
	discPattern     = regexp.MustCompile(`(?i)^(?:disc|disk|cd|volume|vol)[\s\-]*(\d{1,2})$`)

	videoExtensions = map[string]bool{
		".mkv": true, ".mp4": true, ".m4v": true, ".avi": true, ".mov": true,
		".wmv": true, ".flv": true, ".webm": true, ".mpg": true, ".mpeg": true,
		".ts": true, ".m2ts": true, ".mts": true, ".ogv": true, ".3gp": true,
	}

	audioExtensions = map[string]bool{
		".mp3": true, ".flac": true, ".m4a": true, ".m4b": true, ".aac": true,
		".ogg": true, ".oga": true, ".opus": true, ".wav": true, ".wma": true,
		".alac": true, ".aiff": true, ".aif": true, ".ape": true, ".mka": true,
		".dsf": true, ".wv": true,
	}

	subtitleExtensions = map[string]bool{
		".srt": true, ".vtt": true, ".ass": true, ".ssa": true, ".sub": true,
	}
)

func isVideo(name string) bool {
	return videoExtensions[strings.ToLower(filepath.Ext(name))]
}

func isAudio(name string) bool {
	return audioExtensions[strings.ToLower(filepath.Ext(name))]
}

func isSubtitle(name string) bool {
	return subtitleExtensions[strings.ToLower(filepath.Ext(name))]
}

func parseSubtitle(base, name string) (items.ExternalSubtitle, bool) {
	if !isSubtitle(name) {
		return items.ExternalSubtitle{}, false
	}

	remainder := stripExtension(name)
	if !strings.EqualFold(remainder, base) && !strings.HasPrefix(strings.ToLower(remainder), strings.ToLower(base)+".") {
		return items.ExternalSubtitle{}, false
	}

	subtitle := items.ExternalSubtitle{
		Codec: strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), "."),
	}

	tokens := make([]string, 0)
	for _, token := range strings.Split(remainder[len(base):], ".") {
		if token != "" {
			tokens = append(tokens, strings.ToLower(token))
		}
	}

	for len(tokens) > 0 && takeFlag(&subtitle, tokens[len(tokens)-1]) {
		tokens = tokens[:len(tokens)-1]
	}
	if len(tokens) > 0 && languagePattern.MatchString(tokens[len(tokens)-1]) {
		subtitle.Language = tokens[len(tokens)-1]
		tokens = tokens[:len(tokens)-1]
	}
	subtitle.Title = clean(strings.Join(tokens, " "))

	return subtitle, true
}

func takeFlag(subtitle *items.ExternalSubtitle, token string) bool {
	switch token {
	case "forced":
		subtitle.IsForced = true
	case "default":
		subtitle.IsDefault = true
	case "sdh", "hi", "cc":
		subtitle.IsHearingImpaired = true
	default:
		return false
	}

	return true
}

func stripExtension(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func parseTitle(name string) (string, *int32) {
	name = clean(name)

	match := yearPattern.FindStringSubmatch(name)
	if match == nil || clean(match[1]) == "" {
		return name, nil
	}

	year, err := strconv.Atoi(match[2])
	if err != nil {
		return name, nil
	}

	return clean(match[1]), ptr(int32(year))
}

func parseEpisode(name string) (season, episode *int32, title string, ok bool) {
	match := episodePattern.FindStringSubmatch(stripExtension(name))
	if match == nil {
		return nil, nil, "", false
	}

	seasonText, episodeText := match[2], match[3]
	if seasonText == "" {
		seasonText, episodeText = match[4], match[5]
	}

	seasonNumber, err := strconv.Atoi(seasonText)
	if err != nil {
		return nil, nil, "", false
	}
	episodeNumber, err := strconv.Atoi(episodeText)
	if err != nil {
		return nil, nil, "", false
	}

	return ptr(int32(seasonNumber)), ptr(int32(episodeNumber)), clean(match[6]), true
}

func parseSeason(name string) (*int32, bool) {
	name = clean(name)
	if specialsPattern.MatchString(name) {
		return ptr(int32(0)), true
	}

	match := seasonPattern.FindStringSubmatch(name)
	if match == nil {
		return nil, false
	}

	number, err := strconv.Atoi(match[1])
	if err != nil {
		return nil, false
	}

	return ptr(int32(number)), true
}

// parseTrack pulls a disc and track number off names like "01 - Title" or
// "1-05 Title".
func parseTrack(name string) (disc, track *int32, title string) {
	title = clean(stripExtension(name))

	match := trackPattern.FindStringSubmatch(title)
	if match == nil || clean(match[3]) == "" {
		return nil, nil, title
	}

	number, err := strconv.ParseInt(match[2], 10, 32)
	if err != nil {
		return nil, nil, title
	}
	if side, err := strconv.ParseInt(match[1], 10, 32); err == nil {
		disc = ptr(int32(side))
	}

	return disc, ptr(int32(number)), clean(match[3])
}

// parseDisc reads a disc number from a directory inside an album.
func parseDisc(name string) (*int32, bool) {
	match := discPattern.FindStringSubmatch(clean(name))
	if match == nil {
		return nil, false
	}

	number, err := strconv.ParseInt(match[1], 10, 32)
	if err != nil {
		return nil, false
	}

	return ptr(int32(number)), true
}

func seasonName(number *int32) string {
	if number == nil || *number == 0 {
		return "Specials"
	}

	return "Season " + strconv.Itoa(int(*number))
}

func seasonSortName(number *int32) string {
	if number == nil {
		return "0000"
	}

	return fmt.Sprintf("%04d", *number)
}

func episodeTitle(seriesName string, season, episode *int32) string {
	if season == nil || episode == nil {
		return seriesName
	}

	return fmt.Sprintf("%s S%02dE%02d", seriesName, *season, *episode)
}

func movieKey(name string, year *int32) string {
	return "movie:" + titleSlug(name, year)
}

func seriesKey(slug string) string {
	return "series:" + slug
}

func seasonKey(slug string, number *int32) string {
	if number == nil {
		return "season:" + slug
	}

	return fmt.Sprintf("season:%s:%d", slug, *number)
}

func episodeKey(slug string, season, episode *int32, title string) string {
	if season == nil || episode == nil {
		return "episode:" + slug + ":" + slugify(title)
	}

	return fmt.Sprintf("episode:%s:%d:%d", slug, *season, *episode)
}

func musicArtistKey(slug string) string {
	return "musicartist:" + slug
}

func musicAlbumKey(slug string) string {
	return "musicalbum:" + slug
}

// A track is keyed by its position under its album, so two recordings of one
// song stay apart while a flac and an mp3 of the same track become two files of
// one item. A track with no number falls back to its title the way an unnumbered
// episode does.
func audioKey(scope string, disc, track *int32, title string) string {
	if track == nil {
		return keyOf("audio", scope, slugify(title))
	}

	side := int32(1)
	if disc != nil {
		side = *disc
	}

	return keyOf("audio", scope, fmt.Sprintf("%d:%d", side, *track))
}

func albumSlug(artist string, name string, year *int32) string {
	return keyOf(artist, titleSlug(name, year))
}

func keyOf(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}

	return strings.Join(kept, ":")
}

func titleSlug(name string, year *int32) string {
	if year == nil {
		return slugify(name)
	}

	return fmt.Sprintf("%s:%d", slugify(name), *year)
}

func slugify(name string) string {
	var slug strings.Builder
	separated := false
	for _, letter := range strings.ToLower(name) {
		if !unicode.IsLetter(letter) && !unicode.IsDigit(letter) {
			separated = slug.Len() > 0
			continue
		}
		if separated {
			slug.WriteByte('-')
			separated = false
		}
		slug.WriteRune(letter)
	}

	return slug.String()
}

func sortName(name string) string {
	return articlePattern.ReplaceAllString(strings.ToLower(clean(name)), "")
}

func clean(name string) string {
	name = strings.NewReplacer(".", " ", "_", " ").Replace(name)

	return strings.TrimSpace(strings.Join(strings.Fields(name), " "))
}

func ptr[T any](v T) *T {
	return &v
}
