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
	articlePattern  = regexp.MustCompile(`(?i)^(the|a|an)\s+`)
	languagePattern = regexp.MustCompile(`^[a-z]{2,3}$`)

	subtitleExtensions = map[string]bool{
		".srt": true, ".vtt": true, ".ass": true, ".ssa": true, ".sub": true,
	}
)

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

func episodeName(name string, number *int32) string {
	if name != "" {
		return name
	}
	if number == nil {
		return "Episode"
	}

	return "Episode " + strconv.Itoa(int(*number))
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
