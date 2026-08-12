package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type document struct {
	Countries       map[string]country
	Cultures        []culture
	Options         []option
	FallbackRatings []parentalRating
}

type country struct {
	DisplayName        string
	ThreeLetterISOName string
	ParentalRatings    []parentalRating `json:",omitempty"`
}

type culture struct {
	Name                string
	TwoLetterISOName    string
	ThreeLetterISONames []string
}

type option struct {
	Name  string
	Value string
}

type parentalRating struct {
	Name  string
	Level int
}

const fallbackCode = "0-PREFER"

func main() {
	source := flag.String("source", "", "path to jellyfin's Emby.Server.Implementations/Localization")
	out := flag.String("out", "", "file to write the merged document to")
	flag.Parse()

	if *source == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}

	doc, err := build(*source)
	if err != nil {
		log.Fatal(err)
	}

	encoded, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, append(encoded, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
}

func build(source string) (*document, error) {
	countries, err := readCountries(filepath.Join(source, "countries.json"))
	if err != nil {
		return nil, err
	}
	tables, err := readRatings(filepath.Join(source, "Ratings"))
	if err != nil {
		return nil, err
	}
	cultures, err := readCultures(filepath.Join(source, "iso6392.txt"))
	if err != nil {
		return nil, err
	}
	options, err := readOptions(filepath.Join(source, "LocalizationManager.cs"))
	if err != nil {
		return nil, err
	}

	for code, table := range tables {
		if code == fallbackCode {
			continue
		}
		record, found := countries[code]
		if !found {
			continue
		}
		record.ParentalRatings = table
		countries[code] = record
	}

	return &document{
		Countries:       countries,
		Cultures:        cultures,
		Options:         options,
		FallbackRatings: tables[fallbackCode],
	}, nil
}

func readCountries(path string) (map[string]country, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var upstream []struct {
		DisplayName              string
		Name                     string
		ThreeLetterISORegionName string
	}
	if err := json.Unmarshal(contents, &upstream); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	countries := make(map[string]country, len(upstream))
	for _, record := range upstream {
		countries[record.Name] = country{
			DisplayName:        record.DisplayName,
			ThreeLetterISOName: record.ThreeLetterISORegionName,
		}
	}

	return countries, nil
}

func readRatings(dir string) (map[string][]parentalRating, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.csv"))
	if err != nil {
		return nil, err
	}

	tables := make(map[string][]parentalRating, len(paths))
	for _, path := range paths {
		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		rows, err := csv.NewReader(strings.NewReader(string(contents))).ReadAll()
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}

		table := make([]parentalRating, 0, len(rows))
		for _, row := range rows {
			if len(row) != 2 {
				return nil, fmt.Errorf("parsing %s: %q is not a name and a level", path, row)
			}
			level, err := strconv.Atoi(row[1])
			if err != nil {
				return nil, fmt.Errorf("parsing %s: %w", path, err)
			}
			table = append(table, parentalRating{Name: row[0], Level: level})
		}

		code := strings.ToUpper(strings.TrimSuffix(filepath.Base(path), ".csv"))
		tables[code] = table
	}

	return tables, nil
}

func readCultures(path string) ([]culture, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cultures []culture
	for _, line := range strings.Split(string(contents), "\n") {
		parts := strings.Split(strings.TrimSpace(line), "|")
		if len(parts) != 5 || parts[3] == "" || parts[2] == "" {
			continue
		}

		names := []string{parts[0]}
		if parts[1] != "" {
			names = append(names, parts[1])
		}
		cultures = append(cultures, culture{
			Name:                parts[3],
			TwoLetterISOName:    parts[2],
			ThreeLetterISONames: names,
		})
	}

	return cultures, nil
}

var optionPattern = regexp.MustCompile(`new LocalizationOption\("([^"]*)", "([^"]*)"\)`)

func readOptions(path string) ([]option, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	matches := optionPattern.FindAllStringSubmatch(string(contents), -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no localization options in %s", path)
	}

	options := make([]option, 0, len(matches))
	for _, match := range matches {
		options = append(options, option{Name: match[1], Value: match[2]})
	}

	return options, nil
}
