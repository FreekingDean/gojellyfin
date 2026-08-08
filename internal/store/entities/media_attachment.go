package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
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
