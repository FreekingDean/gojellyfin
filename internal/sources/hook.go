package sources

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/sources/arr"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/source"
)

const (
	webhookName           = "gojellyfin"
	webhookImplementation = "Webhook"
	webhookMethodPost     = 1
)

var (
	errUnpublished = errors.New("PUBLISHED_SERVER_URL is not set, so there is no address to give the instance")

	hookedOn = []string{
		"onDownload",
		"onUpgrade",
		"onRename",
		"onMovieDelete",
		"onMovieFileDelete",
		"onMovieFileDeleteForUpgrade",
		"onSeriesDelete",
		"onEpisodeFileDelete",
		"onEpisodeFileDeleteForUpgrade",
		"onReleaseImport",
		"onArtistDelete",
		"onAlbumDelete",
	}
)

func (s *Service) WebhookURL(source Source) string {
	if s.config.PublishedServerURL == "" {
		return ""
	}

	return strings.TrimSuffix(s.config.PublishedServerURL, "/") + WebhookPath + "/" + source.ID.String()
}

func (s *Service) Hook(ctx context.Context, id uuid.UUID) error {
	source, err := s.store.Source.Query().Where(sourcemodel.ID(id)).OnlyModel(ctx)
	if err != nil {
		return fmt.Errorf("failed to query the source: %w", err)
	}
	hookErr := s.hook(ctx, *source)

	update := s.store.Source.UpdateOneID(id)
	if hookErr != nil {
		update.ClearWebhookRegisteredAt().SetWebhookError(hookErr.Error())
	} else {
		update.SetWebhookRegisteredAt(time.Now()).SetWebhookError("")
	}

	if err := update.Exec(ctx); err != nil {
		return errors.Join(hookErr, err)
	}

	return hookErr
}

func (s *Service) hook(ctx context.Context, source Source) error {
	target := s.WebhookURL(source)
	if target == "" {
		return errUnpublished
	}

	found, err := s.hooked(ctx, source)
	if err != nil {
		return err
	}

	method, path := http.MethodPost, arr.NotifyPath
	if found != nil {
		method, path = http.MethodPut, arr.NotifyPath+"/"+strconv.FormatInt(recordID(found), 10)
	} else if found, err = s.webhookSchema(ctx, source); err != nil {
		return err
	}

	configure(found, target, source.WebhookSecret)

	return s.send(ctx, source, method, path, found)
}

func (s *Service) unhook(ctx context.Context, source Source) error {
	found, err := s.hooked(ctx, source)
	if err != nil || found == nil {
		return err
	}

	return s.send(ctx, source, http.MethodDelete, arr.NotifyPath+"/"+strconv.FormatInt(recordID(found), 10), nil)
}

func (s *Service) hooked(ctx context.Context, source Source) (record, error) {
	notifications, err := fetch[[]record](ctx, s, s.client, source, arr.NotifyPath, nil)
	if err != nil {
		return nil, err
	}

	for _, notification := range notifications {
		if notification["implementation"] == webhookImplementation && notification["name"] == webhookName {
			return notification, nil
		}
	}

	return nil, nil
}

func (s *Service) webhookSchema(ctx context.Context, source Source) (record, error) {
	schemas, err := fetch[[]record](ctx, s, s.client, source, arr.NotifySchema, nil)
	if err != nil {
		return nil, err
	}

	for _, schema := range schemas {
		if schema["implementation"] == webhookImplementation {
			return schema, nil
		}
	}

	return nil, fmt.Errorf("%s offers no webhook connection", source.Name)
}

func configure(notification record, target, secret string) {
	notification["name"] = webhookName
	notification["tags"] = []int{}

	for _, event := range hookedOn {
		if _, offered := notification[event]; !offered {
			continue
		}
		if supported, stated := notification["supports"+strings.ToUpper(event[:1])+event[1:]].(bool); stated && !supported {
			continue
		}
		notification[event] = true
	}

	values := map[string]any{
		"url":      target,
		"method":   webhookMethodPost,
		"username": WebhookUsername,
		"password": secret,
	}

	fields, _ := notification["fields"].([]any)
	for _, field := range fields {
		entry, ok := field.(map[string]any)
		if !ok {
			continue
		}
		name, _ := entry["name"].(string)
		if value, wanted := values[name]; wanted {
			entry["value"] = value
		}
	}
}

func recordID(found record) int64 {
	id, _ := found["id"].(float64)

	return int64(id)
}

func (s *Service) hookQuietly(ctx context.Context, id uuid.UUID) {
	if err := s.Hook(ctx, id); err != nil {
		log.Printf("the webhook on source %s is not registered; its webhook error says why", id)
	}
}
