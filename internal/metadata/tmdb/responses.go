package tmdb

type searchResults struct {
	Results []struct {
		ID int `json:"id"`
	} `json:"results"`
}

type country struct {
	Name string `json:"name"`
}

type externalIDs struct {
	IMDbID string `json:"imdb_id"`
}

type Movie struct {
	ID                  int       `json:"id"`
	IMDbID              string    `json:"imdb_id"`
	Title               string    `json:"title"`
	OriginalTitle       string    `json:"original_title"`
	Overview            string    `json:"overview"`
	Tagline             string    `json:"tagline"`
	Status              string    `json:"status"`
	ReleaseDate         string    `json:"release_date"`
	VoteAverage         float64   `json:"vote_average"`
	ProductionCountries []country `json:"production_countries"`
	ReleaseDates        struct {
		Results []struct {
			Country      string `json:"iso_3166_1"`
			ReleaseDates []struct {
				Certification string `json:"certification"`
			} `json:"release_dates"`
		} `json:"results"`
	} `json:"release_dates"`
}

type Series struct {
	ID                  int         `json:"id"`
	Name                string      `json:"name"`
	OriginalName        string      `json:"original_name"`
	Overview            string      `json:"overview"`
	Tagline             string      `json:"tagline"`
	Status              string      `json:"status"`
	FirstAirDate        string      `json:"first_air_date"`
	LastAirDate         string      `json:"last_air_date"`
	InProduction        bool        `json:"in_production"`
	VoteAverage         float64     `json:"vote_average"`
	ProductionCountries []country   `json:"production_countries"`
	ExternalIDs         externalIDs `json:"external_ids"`
	ContentRatings      struct {
		Results []struct {
			Country string `json:"iso_3166_1"`
			Rating  string `json:"rating"`
		} `json:"results"`
	} `json:"content_ratings"`
}

type Episode struct {
	ID          int         `json:"id"`
	Name        string      `json:"name"`
	Overview    string      `json:"overview"`
	AirDate     string      `json:"air_date"`
	VoteAverage float64     `json:"vote_average"`
	ExternalIDs externalIDs `json:"external_ids"`
}
