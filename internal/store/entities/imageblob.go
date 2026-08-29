package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ImageBlob struct {
	ent.Schema
}

func (ImageBlob) Fields() []ent.Field {
	return withDefaultFields(
		field.String("key"),
		field.Bytes("data"),
	)
}

func (ImageBlob) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("key").Unique(),
	}
}
