package lyrics

import (
	"cmp"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const ticksPerMillisecond = 10_000

type Line struct {
	Start *int64
	Text  string
}

type Metadata struct {
	Artist string
	Album  string
	Title  string
	Author string
	By     string
	Offset *int64
	Length *int64
}

type Lyrics struct {
	Metadata Metadata
	Synced   bool
	Lines    []Line
}

var (
	stamp = regexp.MustCompile(`^\[(\d+):(\d{1,2})(?:[.:](\d{1,3}))?\]`)
	tag   = regexp.MustCompile(`^\[([a-zA-Z#]+):(.*)\]$`)
)

func Parse(text string) Lyrics {
	var parsed Lyrics
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		if starts, rest := stamps(line); len(starts) > 0 {
			parsed.Synced = true
			for _, start := range starts {
				parsed.Lines = append(parsed.Lines, Line{Start: &start, Text: rest})
			}

			continue
		}

		if found := tag.FindStringSubmatch(line); found != nil {
			parsed.Metadata.set(strings.ToLower(found[1]), strings.TrimSpace(found[2]))

			continue
		}

		parsed.Lines = append(parsed.Lines, Line{Text: line})
	}

	if parsed.Synced {
		kept := parsed.Lines[:0]
		for _, line := range parsed.Lines {
			if line.Start != nil {
				kept = append(kept, line)
			}
		}
		parsed.Lines = kept
		slices.SortStableFunc(parsed.Lines, func(a, b Line) int { return cmp.Compare(*a.Start, *b.Start) })
	}

	return parsed
}

func stamps(line string) ([]int64, string) {
	var starts []int64
	for {
		found := stamp.FindStringSubmatch(line)
		if found == nil {
			return starts, strings.TrimSpace(line)
		}

		minutes, _ := strconv.ParseInt(found[1], 10, 64)
		seconds, _ := strconv.ParseInt(found[2], 10, 64)
		fraction := found[3]
		milliseconds := int64(0)
		if fraction != "" {
			milliseconds, _ = strconv.ParseInt((fraction + "00")[:3], 10, 64)
		}

		starts = append(starts, ((minutes*60+seconds)*1000+milliseconds)*ticksPerMillisecond)
		line = line[len(found[0]):]
	}
}

func (m *Metadata) set(name, value string) {
	switch name {
	case "ar":
		m.Artist = value
	case "al":
		m.Album = value
	case "ti":
		m.Title = value
	case "au":
		m.Author = value
	case "by":
		m.By = value
	case "offset":
		if milliseconds, err := strconv.ParseInt(value, 10, 64); err == nil {
			ticks := milliseconds * ticksPerMillisecond
			m.Offset = &ticks
		}
	case "length":
		if ticks, ok := length(value); ok {
			m.Length = &ticks
		}
	}
}

func length(value string) (int64, bool) {
	minutes, seconds, found := strings.Cut(value, ":")
	if !found {
		return 0, false
	}

	whole, err := strconv.ParseInt(minutes, 10, 64)
	if err != nil {
		return 0, false
	}

	part, err := strconv.ParseFloat(seconds, 64)
	if err != nil {
		return 0, false
	}

	return int64((float64(whole)*60+part)*1000) * ticksPerMillisecond, true
}
