package scanner

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	yearPattern     = regexp.MustCompile(`^(.*?)[\s._\-]*[\(\[]?((?:19|20)\d{2})[\)\]]?[\s._\-]*$`)
	episodePattern  = regexp.MustCompile(`(?i)^(.*?)[\s._\-]*(?:s(\d{1,3})[\s._\-]*e(\d{1,4})|(\d{1,2})x(\d{1,4}))[\s._\-]*(.*)$`)
	seasonPattern   = regexp.MustCompile(`(?i)^(?:season|series|s)[\s._\-]*(\d{1,3})$`)
	specialsPattern = regexp.MustCompile(`(?i)^(?:specials?|season\s*0)$`)
	articlePattern  = regexp.MustCompile(`(?i)^(the|a|an)\s+`)

	videoExtensions = map[string]bool{
		".mkv": true, ".mp4": true, ".m4v": true, ".avi": true, ".mov": true,
		".wmv": true, ".flv": true, ".webm": true, ".mpg": true, ".mpeg": true,
		".ts": true, ".m2ts": true, ".mts": true, ".ogv": true, ".3gp": true,
	}
)

func isVideo(name string) bool {
	return videoExtensions[strings.ToLower(filepath.Ext(name))]
}

func stripExtension(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// parseTitle splits a trailing release year off a movie or series name.
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

// parseEpisode pulls season and episode numbers out of names like
// "Show S01E02 Title" or "Show 1x02".
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

// parseSeason reads a season number from a directory name.
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
