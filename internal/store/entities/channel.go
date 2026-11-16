package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type Channel struct {
	ent.Schema
}

func (Channel) Fields() []ent.Field {
	return withDefaultFields(
		field.Int32("number").Unique().Positive().Max(65535),
		field.String("name").Unique().NotEmpty(),
		field.Enum("order").Values("Shuffle", "Sequential").Default("Shuffle"),
		field.Int64("seed"),
		field.Time("epoch"),
	)
}

func (Channel) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("entries", ChannelEntry.Type).Annotations(cascadeOnDelete),
	}
}
