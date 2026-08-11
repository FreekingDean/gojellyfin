package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type MediaAttachment struct {
	ent.Schema
}

func (MediaAttachment) Fields() []ent.Field {
	return withDefaultFields(
		field.Int32("index"),
		field.String("codec").Optional(),
		field.String("codec_tag").Optional(),
		field.String("comment").Optional(),
		field.String("file_name").Optional(),
		field.String("mime_type").Optional(),
	)
}

func (MediaAttachment) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("source", MediaSource.Type).Ref("attachments").Unique().Required(),
	}
}

func (MediaAttachment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("index").Edges("source").Unique(),
	}
}
