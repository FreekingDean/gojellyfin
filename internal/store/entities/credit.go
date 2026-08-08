package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type Credit struct {
	ent.Schema
}

func (Credit) Fields() []ent.Field {
	return withDefaultFields(
		field.Enum("kind").Values(
			"Unknown", "Actor", "Director", "Composer", "Writer", "GuestStar",
			"Producer", "Conductor", "Lyricist", "Arranger", "Engineer", "Mixer",
			"Remixer", "Creator", "Artist", "AlbumArtist", "Author", "Illustrator",
			"Penciller", "Inker", "Colorist", "Letterer", "CoverArtist", "Editor",
			"Translator",
		),
		field.String("role").Optional(),
		field.Int32("sort_order").Optional(),
	)
}

func (Credit) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("credits").Unique().Required(),
		edge.From("person", Person.Type).Ref("credits").Unique().Required(),
	}
}
