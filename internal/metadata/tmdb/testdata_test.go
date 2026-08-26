package tmdb

const (
	matrixSearch = `{"page":1,"results":[{"id":603,"title":"The Matrix","original_title":"The Matrix","release_date":"1999-03-30"}],"total_results":1}`

	matrixDetail = `{
		"id": 603,
		"imdb_id": "tt0133093",
		"title": "The Matrix",
		"original_title": "The Matrix",
		"overview": "Set in the 22nd century, The Matrix tells the story of a computer hacker.",
		"tagline": "Welcome to the Real World.",
		"status": "Released",
		"release_date": "1999-03-30",
		"vote_average": 8.2,
		"production_countries": [{"iso_3166_1": "US", "name": "United States of America"}],
		"release_dates": {"results": [{"iso_3166_1": "US", "release_dates": [{"certification": "R", "type": 3}]}]}
	}`

	breakingBadSearch = `{"page":1,"results":[{"id":1396,"name":"Breaking Bad","first_air_date":"2008-01-20"}],"total_results":1}`

	breakingBadDetail = `{
		"id": 1396,
		"name": "Breaking Bad",
		"original_name": "Breaking Bad",
		"overview": "A high school chemistry teacher turns to a life of crime.",
		"tagline": "Change the equation.",
		"status": "Ended",
		"first_air_date": "2008-01-20",
		"last_air_date": "2013-09-29",
		"in_production": false,
		"vote_average": 8.9,
		"production_countries": [{"iso_3166_1": "US", "name": "United States of America"}],
		"external_ids": {"imdb_id": "tt0903747"},
		"content_ratings": {"results": [{"iso_3166_1": "US", "rating": "TV-MA"}]}
	}`

	breakingBadSeasonOne = `{
		"id": 3572,
		"name": "Season 1",
		"overview": "High school chemistry teacher Walter White's life is suddenly transformed.",
		"air_date": "2008-01-20",
		"season_number": 1,
		"poster_path": "/1BP4xYv9ZG4ZVHkL7ocOziBbSYH.jpg",
		"vote_average": 8.3
	}`

	breakingBadSpecials = `{
		"id": 3577,
		"name": "Specials",
		"overview": "",
		"air_date": "2009-02-17",
		"season_number": 0,
		"poster_path": "/40dT79mDEZwXkQiZNBgSaydQFDP.jpg"
	}`

	breakingBadPilot = `{
		"id": 62085,
		"name": "Pilot",
		"overview": "A chemistry teacher is diagnosed with terminal lung cancer.",
		"air_date": "2008-01-20",
		"season_number": 1,
		"episode_number": 1,
		"vote_average": 8.4,
		"external_ids": {"imdb_id": "tt0959621"}
	}`
)

var searches = map[string]string{
	"The Matrix":   matrixSearch,
	"Breaking Bad": breakingBadSearch,
}

var details = map[string]string{
	"/3/movie/603":                  matrixDetail,
	"/3/tv/1396":                    breakingBadDetail,
	"/3/tv/1396/season/0":           breakingBadSpecials,
	"/3/tv/1396/season/1":           breakingBadSeasonOne,
	"/3/tv/1396/season/1/episode/1": breakingBadPilot,
}
