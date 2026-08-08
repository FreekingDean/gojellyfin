package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type ChannelMapping struct {
	Name                string `json:"name"`
	ChannelID           string `json:"channel_id"`
	ProviderChannelName string `json:"provider_channel_name"`
	ProviderChannelID   string `json:"provider_channel_id"`
}

type ListingsProvider struct {
	ent.Schema
}

func (ListingsProvider) Fields() []ent.Field {
	return withDefaultFields(
		field.String("kind"),
		field.String("username").Optional(),
		field.String("password").Optional().Sensitive(),
		field.String("listings_id").Optional(),
		field.String("zip_code").Optional(),
		field.String("country").Optional(),
		field.String("path").Optional(),
		field.String("movie_prefix").Optional(),
		field.String("preferred_language").Optional(),
		field.String("user_agent").Optional(),

		field.Bool("enable_all_tuners"),
		field.JSON("enabled_tuners", []string{}).Optional(),
		field.JSON("news_categories", []string{}).Optional(),
		field.JSON("sports_categories", []string{}).Optional(),
		field.JSON("kids_categories", []string{}).Optional(),
		field.JSON("movie_categories", []string{}).Optional(),
		field.JSON("channel_mappings", []ChannelMapping{}).Optional(),
	)
}
