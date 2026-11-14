package sources

import "testing"

func TestLocalise(t *testing.T) {
	source := Source{Name: "Radarr", RootPath: "/movies/", LocalPath: "/media/movies"}

	for path, want := range map[string]string{
		"/movies/Film (2001)/film.mkv": "/media/movies/Film (2001)/film.mkv",
		"/movies":                      "/media/movies",
	} {
		got, err := Localise(source, path)
		if err != nil || got != want {
			t.Errorf("Localise(%q) = %q, %v, want %q", path, got, err, want)
		}
	}

	for _, path := range []string{
		"/moviesecrets/film.mkv",
		"/movies/../etc/passwd",
		"/movies/Film/../../etc/passwd",
		"movies/film.mkv",
		"/tv/show.mkv",
	} {
		if got, err := Localise(source, path); err == nil {
			t.Errorf("Localise(%q) = %q, want it refused as outside the root", path, got)
		}
	}
}
