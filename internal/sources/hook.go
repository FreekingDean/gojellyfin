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
	"github.com/FreekingDean/gojellyfin/internal/store"
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
	var hookErr error
	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		source, err := tx.Source.Query().Where(sourcemodel.ID(id)).ForUpdate().OnlyModel(ctx)
		if err != nil {
			return fmt.Errorf("failed to query the source: %w", err)
		}
		hookErr = s.hook(ctx, *source)

		update := tx.Source.UpdateOneID(id)
		if hookErr != nil {
			update.ClearWebhookRegisteredAt().SetWebhookError(hookErr.Error())
		} else {
			update.SetWebhookRegisteredAt(time.Now()).SetWebhookError("")
		}

		return update.Exec(ctx)
	})

	return errors.Join(hookErr, err)
}

func (s *Service) hook(ctx context.Context, source Source) error {
	target := s.WebhookURL(source)
	if target == "" {
		return errUnpublished
	}

	held, err := s.hooked(ctx, source)
	if err != nil {
		return err
	}

	if len(held) > 1 {
		if err := s.drop(ctx, source, held[1:]); err != nil {
			return err
		}
	}

	var found record
	method, path := http.MethodPost, arr.NotifyPath
	if len(held) > 0 {
		found = held[0]
		method, path = http.MethodPut, notificationPath(found)
	} else if found, err = s.webhookSchema(ctx, source); err != nil {
		return err
	}

	configure(found, target, source.WebhookSecret)

	return s.send(ctx, source, method, path, found)
}

func (s *Service) unhook(ctx context.Context, source Source) error {
	held, err := s.hooked(ctx, source)
	if err != nil {
		return err
	}

	return s.drop(ctx, source, held)
}

func (s *Service) drop(ctx context.Context, source Source, held []record) error {
	for _, found := range held {
		if err := s.send(ctx, source, http.MethodDelete, notificationPath(found), nil); err != nil {
			return err
		}
	}

	return nil
}

func notificationPath(found record) string {
	return arr.NotifyPath + "/" + strconv.FormatInt(recordID(found), 10)
}

func (s *Service) hooked(ctx context.Context, source Source) ([]record, error) {
	notifications, err := fetch[[]record](ctx, s, s.client, source, arr.NotifyPath, nil)
	if err != nil {
		return nil, err
	}

	var held []record
	for _, notification := range notifications {
		if notification["implementation"] == webhookImplementation && notification["name"] == webhookName {
			held = append(held, notification)
		}
	}

	return held, nil
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
