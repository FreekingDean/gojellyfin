package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type ItemSource struct {
	ent.Schema
}

func (ItemSource) Fields() []ent.Field {
	return withDefaultFields(
		field.UUID("item_id", uuid.UUID{}),
		field.UUID("source_id", uuid.UUID{}),

		field.String("name"),
		field.String("path"),
		field.String("container").Optional(),
		field.Int64("size").Optional(),
		field.Int64("run_time_ticks").Optional(),
		field.Int32("bitrate").Optional(),
		field.Time("date_modified").Optional(),
		field.Time("probed_at").Optional(),
	)
}

func (ItemSource) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("item_sources").Unique().Required().Field("item_id"),
		edge.From("source", Source.Type).Ref("files").Unique().Required().Field("source_id"),
		edge.To("streams", MediaStream.Type).Annotations(cascadeOnDelete),
	}
}

func (ItemSource) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("item_id", "source_id").Unique(),
		index.Fields("path").Unique(),
	}
}
