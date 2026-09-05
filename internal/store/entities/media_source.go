package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type MediaSource struct {
	ent.Schema
}

func (MediaSource) Fields() []ent.Field {
	return withDefaultFields(
		field.UUID("item_id", uuid.UUID{}),
		field.UUID("library_id", uuid.UUID{}),

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

func (MediaSource) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("media_sources").Unique().Required().Field("item_id"),
		edge.From("library", Library.Type).Ref("media_sources").Unique().Required().Field("library_id"),
		edge.To("streams", MediaStream.Type).Annotations(cascadeOnDelete),
	}
}

func (MediaSource) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("library_id", "path").Unique(),
	}
}
