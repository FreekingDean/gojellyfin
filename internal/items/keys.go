package items

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const provider = "tmdb"

var articlePattern = regexp.MustCompile(`(?i)^(the|a|an)\s+`)

func MovieKey(tmdbID int) string {
	return fmt.Sprintf("movie:%s:%d", provider, tmdbID)
}

func SeriesKey(tmdbID int) string {
	return fmt.Sprintf("series:%s:%d", provider, tmdbID)
}

func SeasonKey(tmdbID int, season int32) string {
	return fmt.Sprintf("season:%s:%d:%d", provider, tmdbID, season)
}

func EpisodeKey(tmdbID int, season, episode int32) string {
	return fmt.Sprintf("episode:%s:%d:%d:%d", provider, tmdbID, season, episode)
}

func SeasonName(number int32) string {
	if number == 0 {
		return "Specials"
	}

	return "Season " + strconv.Itoa(int(number))
}

func SeasonSortName(number int32) string {
	return fmt.Sprintf("%04d", number)
}

func EpisodeName(name string, number int32) string {
	if name != "" {
		return name
	}

	return "Episode " + strconv.Itoa(int(number))
}

func sorted(value string) string {
	return strings.ToLower(value)
}

func SortName(name string) string {
	cleaned := strings.NewReplacer(".", " ", "_", " ").Replace(name)
	cleaned = strings.TrimSpace(strings.Join(strings.Fields(cleaned), " "))

	return articlePattern.ReplaceAllString(strings.ToLower(cleaned), "")
}
