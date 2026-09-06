package scanner

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var articlePattern = regexp.MustCompile(`(?i)^(the|a|an)\s+`)

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
