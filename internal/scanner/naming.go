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

	videoExtensions = map[string]bool{
		".mkv": true, ".mp4": true, ".m4v": true, ".avi": true, ".mov": true,
		".wmv": true, ".flv": true, ".webm": true, ".mpg": true, ".mpeg": true,
		".ts": true, ".m2ts": true, ".mts": true, ".ogv": true, ".3gp": true,
	}

	subtitleExtensions = map[string]bool{
		".srt": true, ".vtt": true, ".ass": true, ".ssa": true, ".sub": true,
	}
)

func isVideo(name string) bool {
	return videoExtensions[strings.ToLower(filepath.Ext(name))]
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
