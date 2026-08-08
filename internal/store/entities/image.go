package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type Image struct {
	ent.Schema
}

func (Image) Fields() []ent.Field {
	return withDefaultFields(
		field.Enum("kind").Values(
			"Primary", "Art", "Backdrop", "Banner", "Logo", "Thumb", "Disc",
			"Box", "Screenshot", "Menu", "Chapter", "BoxRear", "Profile",
		),
		field.Int32("index"),
		field.String("path"),
		field.String("blur_hash").Optional(),
		field.Int32("width").Optional(),
		field.Int32("height").Optional(),
		field.Int64("size").Optional(),
	)
}

func (Image) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("images").Unique().Required(),
	}
}
