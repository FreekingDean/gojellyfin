package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type Image struct {
	ent.Schema
}

func (Image) Fields() []ent.Field {
	return withDefaultFields(
		field.UUID("item_id", uuid.UUID{}),

		field.Enum("kind").Values(
			"Primary", "Art", "Backdrop", "Banner", "Logo", "Thumb", "Disc",
			"Box", "Screenshot", "Menu", "Chapter", "BoxRear", "Profile",
		),
		field.Int32("index").Default(0),
		field.Enum("source").Values("Local", "Remote").Default("Local"),
		field.String("path"),
		field.String("tag"),
		field.String("blur_hash").Optional(),
		field.Int32("width").Optional(),
		field.Int32("height").Optional(),
		field.Int64("size").Optional(),
	)
}

func (Image) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("images").Unique().Required().Field("item_id"),
	}
}

func (Image) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("item_id", "kind", "index").Unique(),
	}
}
