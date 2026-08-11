package playlists

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	playlistmodal "github.com/FreekingDean/gojellyfin/internal/store/playlist"
	entrymodal "github.com/FreekingDean/gojellyfin/internal/store/playlistentry"
	sharemodal "github.com/FreekingDean/gojellyfin/internal/store/playlistshare"
	usermodal "github.com/FreekingDean/gojellyfin/internal/store/user"
)

type (
	Playlist  = store.Playlist
	Entry     = store.PlaylistEntry
	Share     = store.PlaylistShare
	Item      = store.Item
	MediaType = itemmodal.MediaType
)

const MediaTypeUnknown = itemmodal.MediaTypeUnknown

var ValidMediaType = itemmodal.MediaTypeValidator

type Permission struct {
	UserID  uuid.UUID
	CanEdit bool
}

type CreateParams struct {
	Name       string
	MediaType  MediaType
	OpenAccess bool
	ItemIDs    []uuid.UUID
	Shares     []Permission
}

type UpdateParams struct {
	Name       *string
	OpenAccess *bool
	ItemIDs    *[]uuid.UUID
	Shares     *[]Permission
}

// A playlist is an item, and clients address it by its item id, so every method
// here takes the item id rather than the playlist row id.
type Service struct {
	store *store.Client
}

func New(client *store.Client) *Service {
	return &Service{store: client}
}

func (s *Service) Create(ctx context.Context, params CreateParams) (*Item, error) {
	var created *Item

	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		item, err := tx.Item.Create().
			SetKind(itemmodal.KindPlaylist).
			SetMediaType(params.MediaType).
			SetName(params.Name).
			SetSortName(strings.ToLower(params.Name)).
			SetIsFolder(true).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create playlist item: %w", err)
		}

		playlist, err := tx.Playlist.Create().
			SetItem(item).
			SetOpenAccess(params.OpenAccess).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create playlist: %w", err)
		}

		if err := appendEntries(ctx, tx.PlaylistEntry, playlist.ID, 0, params.ItemIDs); err != nil {
			return err
		}
		if err := replaceShares(ctx, tx, playlist.ID, params.Shares); err != nil {
			return err
		}

		created = item

		return nil
	})
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (s *Service) PlaylistByItemID(ctx context.Context, itemID uuid.UUID) (*Playlist, error) {
	return playlistByItem(ctx, s.store.Playlist, itemID)
}

func (s *Service) Update(ctx context.Context, itemID uuid.UUID, params UpdateParams) error {
	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		playlist, err := playlistByItem(ctx, tx.Playlist, itemID)
		if err != nil {
			return err
		}

		if params.Name != nil {
			if err := tx.Item.UpdateOneID(itemID).
				SetName(*params.Name).
				SetSortName(strings.ToLower(*params.Name)).
				Exec(ctx); err != nil {
				return fmt.Errorf("failed to rename playlist: %w", err)
			}
		}

		if params.OpenAccess != nil {
			if err := tx.Playlist.UpdateOneID(playlist.ID).
				SetOpenAccess(*params.OpenAccess).
				Exec(ctx); err != nil {
				return fmt.Errorf("failed to update playlist access: %w", err)
			}
		}

		if params.ItemIDs != nil {
			if _, err := tx.PlaylistEntry.Delete().
				Where(entrymodal.HasPlaylistWith(playlistmodal.ID(playlist.ID))).
				Exec(ctx); err != nil {
				return fmt.Errorf("failed to clear playlist entries: %w", err)
			}
			if err := appendEntries(ctx, tx.PlaylistEntry, playlist.ID, 0, *params.ItemIDs); err != nil {
				return err
			}
		}

		if params.Shares != nil {
			if err := replaceShares(ctx, tx, playlist.ID, *params.Shares); err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *Service) Entries(ctx context.Context, itemID uuid.UUID, startIndex, limit int) ([]*Entry, int, error) {
	playlist, err := playlistByItem(ctx, s.store.Playlist, itemID)
	if err != nil {
		return nil, 0, err
	}

	entries := s.store.PlaylistEntry.Query().
		Where(entrymodal.HasPlaylistWith(playlistmodal.ID(playlist.ID)))

	total, err := entries.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count playlist entries: %w", err)
	}

	entries = entries.Order(entrymodal.BySortOrder(), entrymodal.ByID())
	if startIndex > 0 {
		entries = entries.Offset(startIndex)
	}
	if limit > 0 {
		entries = entries.Limit(limit)
	}

	records, err := entries.WithItem().All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query playlist entries: %w", err)
	}

	return records, total, nil
}

func (s *Service) AddItems(ctx context.Context, itemID uuid.UUID, itemIDs []uuid.UUID) error {
	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		playlist, err := playlistByItem(ctx, tx.Playlist, itemID)
		if err != nil {
			return err
		}

		count, err := tx.PlaylistEntry.Query().
			Where(entrymodal.HasPlaylistWith(playlistmodal.ID(playlist.ID))).
			Count(ctx)
		if err != nil {
			return fmt.Errorf("failed to count playlist entries: %w", err)
		}

		return appendEntries(ctx, tx.PlaylistEntry, playlist.ID, count, itemIDs)
	})
}

func (s *Service) RemoveEntries(ctx context.Context, itemID uuid.UUID, entryIDs []uuid.UUID) error {
	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		playlist, err := playlistByItem(ctx, tx.Playlist, itemID)
		if err != nil {
			return err
		}

		if _, err := tx.PlaylistEntry.Delete().
			Where(
				entrymodal.HasPlaylistWith(playlistmodal.ID(playlist.ID)),
				entrymodal.IDIn(entryIDs...),
			).
			Exec(ctx); err != nil {
			return fmt.Errorf("failed to remove playlist entries: %w", err)
		}

		entries, err := orderedEntries(ctx, tx.PlaylistEntry, playlist.ID)
		if err != nil {
			return err
		}

		return renumber(ctx, tx.PlaylistEntry, entries)
	})
}

func (s *Service) MoveEntry(ctx context.Context, itemID, entryID uuid.UUID, newIndex int) error {
	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		playlist, err := playlistByItem(ctx, tx.Playlist, itemID)
		if err != nil {
			return err
		}

		entries, err := orderedEntries(ctx, tx.PlaylistEntry, playlist.ID)
		if err != nil {
			return err
		}

		current := slices.IndexFunc(entries, func(entry *Entry) bool { return entry.ID == entryID })
		if current < 0 {
			return fmt.Errorf("playlist %s has no entry %s", itemID, entryID)
		}

		moved := entries[current]
		entries = slices.Delete(entries, current, current+1)
		entries = slices.Insert(entries, min(max(newIndex, 0), len(entries)), moved)

		return renumber(ctx, tx.PlaylistEntry, entries)
	})
}

func (s *Service) Shares(ctx context.Context, itemID uuid.UUID) ([]*Share, error) {
	playlist, err := playlistByItem(ctx, s.store.Playlist, itemID)
	if err != nil {
		return nil, err
	}

	shares, err := s.store.PlaylistShare.Query().
		Where(sharemodal.HasPlaylistWith(playlistmodal.ID(playlist.ID))).
		Order(sharemodal.ByCreatedAt(), sharemodal.ByID()).
		WithUser().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query playlist shares: %w", err)
	}

	return shares, nil
}

func (s *Service) ShareFor(ctx context.Context, itemID, userID uuid.UUID) (*Share, error) {
	playlist, err := playlistByItem(ctx, s.store.Playlist, itemID)
	if err != nil {
		return nil, err
	}

	share, err := s.store.PlaylistShare.Query().
		Where(
			sharemodal.HasPlaylistWith(playlistmodal.ID(playlist.ID)),
			sharemodal.HasUserWith(usermodal.ID(userID)),
		).
		WithUser().
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query playlist share: %w", err)
	}

	return share, nil
}

func (s *Service) SetShare(ctx context.Context, itemID uuid.UUID, permission Permission) error {
	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		playlist, err := playlistByItem(ctx, tx.Playlist, itemID)
		if err != nil {
			return err
		}

		updated, err := tx.PlaylistShare.Update().
			Where(
				sharemodal.HasPlaylistWith(playlistmodal.ID(playlist.ID)),
				sharemodal.HasUserWith(usermodal.ID(permission.UserID)),
			).
			SetCanEdit(permission.CanEdit).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to update playlist share: %w", err)
		}
		if updated > 0 {
			return nil
		}

		return createShare(ctx, tx, playlist.ID, permission)
	})
}

func (s *Service) RemoveShare(ctx context.Context, itemID, userID uuid.UUID) error {
	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		playlist, err := playlistByItem(ctx, tx.Playlist, itemID)
		if err != nil {
			return err
		}

		if _, err := tx.PlaylistShare.Delete().
			Where(
				sharemodal.HasPlaylistWith(playlistmodal.ID(playlist.ID)),
				sharemodal.HasUserWith(usermodal.ID(userID)),
			).
			Exec(ctx); err != nil {
			return fmt.Errorf("failed to remove playlist share: %w", err)
		}

		return nil
	})
}

func playlistByItem(ctx context.Context, client *store.PlaylistClient, itemID uuid.UUID) (*Playlist, error) {
	playlist, err := client.Query().
		Where(playlistmodal.HasItemWith(itemmodal.ID(itemID))).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query playlist %s: %w", itemID, err)
	}

	return playlist, nil
}

func orderedEntries(ctx context.Context, client *store.PlaylistEntryClient, playlistID uuid.UUID) ([]*Entry, error) {
	entries, err := client.Query().
		Where(entrymodal.HasPlaylistWith(playlistmodal.ID(playlistID))).
		Order(entrymodal.BySortOrder(), entrymodal.ByID()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query playlist entries: %w", err)
	}

	return entries, nil
}

func appendEntries(ctx context.Context, client *store.PlaylistEntryClient, playlistID uuid.UUID, from int, itemIDs []uuid.UUID) error {
	for offset, itemID := range itemIDs {
		if err := client.Create().
			SetPlaylistID(playlistID).
			SetItemID(itemID).
			SetSortOrder(int32(from + offset)).
			Exec(ctx); err != nil {
			return fmt.Errorf("failed to add playlist entry: %w", err)
		}
	}

	return nil
}

func renumber(ctx context.Context, client *store.PlaylistEntryClient, entries []*Entry) error {
	for index, entry := range entries {
		if entry.SortOrder == int32(index) {
			continue
		}
		if err := client.UpdateOne(entry).SetSortOrder(int32(index)).Exec(ctx); err != nil {
			return fmt.Errorf("failed to reorder playlist entry: %w", err)
		}
	}

	return nil
}

func replaceShares(ctx context.Context, tx *store.Tx, playlistID uuid.UUID, permissions []Permission) error {
	if _, err := tx.PlaylistShare.Delete().
		Where(sharemodal.HasPlaylistWith(playlistmodal.ID(playlistID))).
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to clear playlist shares: %w", err)
	}

	for _, permission := range permissions {
		if err := createShare(ctx, tx, playlistID, permission); err != nil {
			return err
		}
	}

	return nil
}

func createShare(ctx context.Context, tx *store.Tx, playlistID uuid.UUID, permission Permission) error {
	if err := tx.PlaylistShare.Create().
		SetPlaylistID(playlistID).
		SetUserID(permission.UserID).
		SetCanEdit(permission.CanEdit).
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to create playlist share: %w", err)
	}

	return nil
}
