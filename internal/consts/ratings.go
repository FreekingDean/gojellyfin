package consts

import "strings"

type Rating string

const (
	RatingApproved Rating = "Approved"
	RatingG        Rating = "G"
	RatingPG       Rating = "PG"
	RatingPG13     Rating = "PG-13"
	RatingR        Rating = "R"
	RatingNC17     Rating = "NC-17"
	RatingTVY      Rating = "TV-Y"
	RatingTVY7     Rating = "TV-Y7"
	RatingTVG      Rating = "TV-G"
	RatingTVPG     Rating = "TV-PG"
	RatingTV14     Rating = "TV-14"
	RatingTVMA     Rating = "TV-MA"
	RatingUnrated  Rating = "NR"
)

var ratings = map[string]Rating{
	"APPROVED": RatingApproved,
	"G":        RatingG,
	"PG":       RatingPG,
	"PG-13":    RatingPG13,
	"R":        RatingR,
	"NC-17":    RatingNC17,
	"TV-Y":     RatingTVY,
	"TV-Y7":    RatingTVY7,
	"TV-G":     RatingTVG,
	"TV-PG":    RatingTVPG,
	"TV-14":    RatingTV14,
	"TV-MA":    RatingTVMA,
	"NR":       RatingUnrated,
	"UNRATED":  RatingUnrated,
}

// Ratings outside this list belong to a country table rather than to us, so an
// unknown name is carried through rather than rejected.
func ParseRating(value string) (Rating, bool) {
	rating, ok := ratings[strings.ToUpper(strings.TrimSpace(value))]
	if !ok {
		return Rating(value), false
	}

	return rating, true
}
