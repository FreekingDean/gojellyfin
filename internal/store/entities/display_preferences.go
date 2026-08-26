package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type DisplayPreferences struct {
	ent.Schema
}

func (DisplayPreferences) Fields() []ent.Field {
	return withDefaultFields(
		field.UUID("user_id", uuid.UUID{}),
		field.String("reference_id"),
		field.String("client"),
		field.String("view_type").Optional(),
		field.String("sort_by").Optional().Default("SortName"),
		field.String("index_by").Optional(),
		field.Enum("sort_order").Values("Ascending", "Descending").Default("Ascending"),
		field.Enum("scroll_direction").Values("Horizontal", "Vertical").Default("Horizontal"),
		field.Bool("remember_indexing").Default(false),
		field.Bool("remember_sorting").Default(false),
		field.Bool("show_backdrop").Default(true),
		field.Bool("show_sidebar").Default(false),
		field.Int32("primary_image_height").Default(0),
		field.Int32("primary_image_width").Default(0),
		field.JSON("custom_prefs", map[string]string{}).Optional(),
	)
}

func (DisplayPreferences) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("display_preferences").Unique().Required().Field("user_id"),
	}
}

func (DisplayPreferences) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "reference_id", "client").Unique(),
	}
}
