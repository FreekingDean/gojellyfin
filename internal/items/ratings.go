package items

import (
	"entgo.io/ent/dialect/sql"

	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	"github.com/FreekingDean/gojellyfin/internal/store/predicate"
)

var unratedKinds = map[string][]itemmodel.Kind{
	"Movie":  {itemmodel.KindMovie},
	"Series": {itemmodel.KindSeries, itemmodel.KindSeason, itemmodel.KindEpisode},
	"Music":  {itemmodel.KindMusicArtist, itemmodel.KindMusicAlbum, itemmodel.KindAudio},
}

func (v Viewer) rated() []predicate.Item {
	blocked := v.blockedUnrated()
	if v.MaxRating == nil && len(blocked) == 0 {
		return nil
	}

	return []predicate.Item{func(selector *sql.Selector) {
		rating := func(b *sql.Builder) {
			b.WriteString("COALESCE(").
				WriteString(selector.C(itemmodel.FieldParentalRating)).
				WriteString(", (SELECT rated_parent.parental_rating FROM items AS rated_parent WHERE rated_parent.id = ").
				WriteString(selector.C(itemmodel.FieldParentID)).
				WriteString("), (SELECT rated_grandparent.parental_rating FROM items AS rated_parent JOIN items AS rated_grandparent ON rated_grandparent.id = rated_parent.parent_id WHERE rated_parent.id = ").
				WriteString(selector.C(itemmodel.FieldParentID)).
				WriteString("))")
		}

		selector.Where(sql.P(func(b *sql.Builder) {
			b.WriteString("(").WriteString(selector.C(itemmodel.FieldKind)).WriteString(" = ").Arg(string(itemmodel.KindPlaylist))
			b.WriteString(" OR (")
			rating(b)
			b.WriteString(" IS NULL")
			if len(blocked) > 0 {
				b.WriteString(" AND ").WriteString(selector.C(itemmodel.FieldKind)).WriteString(" NOT IN (").Args(blocked...).WriteString(")")
			}
			b.WriteString(")")
			b.WriteString(" OR ")
			rating(b)
			if v.MaxRating != nil {
				b.WriteString(" <= ").Arg(*v.MaxRating)
			} else {
				b.WriteString(" IS NOT NULL")
			}
			b.WriteString(")")
		}))
	}}
}

func (v Viewer) blockedUnrated() []any {
	var kinds []any
	for _, name := range v.BlockUnrated {
		for _, kind := range unratedKinds[name] {
			kinds = append(kinds, string(kind))
		}
	}

	return kinds
}
