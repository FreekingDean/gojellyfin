package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type DisplayPreferences struct {
	ent.Schema
}

func (DisplayPreferences) Fields() []ent.Field {
	return withDefaultFields(
		field.String("client"),
		field.String("view_type").Optional(),
		field.String("sort_by").Optional(),
		field.String("index_by").Optional(),
		field.Enum("sort_order").Values("Ascending", "Descending"),
		field.Enum("scroll_direction").Values("Horizontal", "Vertical"),
		field.Bool("remember_indexing"),
		field.Bool("remember_sorting"),
		field.Bool("show_backdrop"),
		field.Bool("show_sidebar"),
		field.Int32("primary_image_height"),
		field.Int32("primary_image_width"),
		field.JSON("custom_prefs", map[string]string{}).Optional(),
	)
}

func (DisplayPreferences) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("display_preferences").Unique().Required(),
		edge.From("item", Item.Type).Ref("display_preferences").Unique(),
	}
}

func (DisplayPreferences) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("client").Edges("user", "item").Unique(),
		index.Fields("client").Edges("user").Unique().
			Annotations(entsql.IndexWhere("item_display_preferences IS NULL")),
	}
}
