package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type Request struct {
	ent.Schema
}

func (Request) Fields() []ent.Field {
	return withDefaultFields(
		field.UUID("user_id", uuid.UUID{}),
		field.String("key"),
		field.Enum("kind").Values("Movie", "Series"),
		field.Int("tmdb_id"),
		field.Int("tvdb_id").Default(0),
		field.String("name"),
		field.Enum("status").Values("Pending", "Approved", "Declined").Default("Approved"),
	)
}

func (Request) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("requests").Unique().Required().Field("user_id"),
	}
}

func (Request) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "key").Unique(),
		index.Fields("key"),
	}
}
