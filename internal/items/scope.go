package items

import (
	"fmt"

	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	"github.com/FreekingDean/gojellyfin/internal/store/predicate"
)

type Scope struct {
	key         string
	descendants []string
	tree        bool
}

func MovieScope(tmdbID int) Scope {
	return Scope{key: MovieKey(tmdbID)}
}

func SeriesScope(tmdbID int) Scope {
	return Scope{
		key: SeriesKey(tmdbID),
		descendants: []string{
			fmt.Sprintf("season:%s:%d:", provider, tmdbID),
			fmt.Sprintf("episode:%s:%d:", provider, tmdbID),
		},
	}
}

func ArtistScope(musicBrainzID string) Scope {
	return Scope{key: ArtistKey(musicBrainzID), tree: true}
}

func (s Scope) items() predicate.Item {
	keys := []predicate.Item{itemmodel.Key(s.key)}
	if s.tree {
		keys = append(keys,
			itemmodel.HasParentWith(itemmodel.Key(s.key)),
			itemmodel.HasParentWith(itemmodel.HasParentWith(itemmodel.Key(s.key))),
		)
	}
	for _, prefix := range s.descendants {
		keys = append(keys, itemmodel.KeyHasPrefix(prefix))
	}

	return itemmodel.Or(keys...)
}
