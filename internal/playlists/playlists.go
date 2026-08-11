package playlists

import (
	"context"
	"fmt"
	"strings"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	playlistmodal "github.com/FreekingDean/gojellyfin/internal/store/playlist"
	entrymodal "github.com/FreekingDean/gojellyfin/internal/store/playlistentry"
	sharemodal "github.com/FreekingDean/gojellyfin/internal/store/playlistshare"
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

// What a caller may do with a playlist. The owner is not a share row, so
// ownership is answered from the playlist itself.
type Access struct {
	Owner   bool
	CanEdit bool
	CanView bool
}

type CreateParams struct {
	Name       string
	MediaType  MediaType
	OwnerID    uuid.UUID
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
			SetItemID(item.ID).
			SetOwnerID(params.OwnerID).
			SetOpenAccess(params.OpenAccess).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create playlist: %w", err)
		}

		itemIDs, err := expand(ctx, tx.Item, params.ItemIDs)
		if err != nil {
			return err
		}
		if err := appendEntries(ctx, tx.PlaylistEntry, playlist.ID, 0, itemIDs); err != nil {
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

func (s *Service) Access(ctx context.Context, itemID, userID uuid.UUID) (Access, error) {
	playlist, err := playlistByItem(ctx, s.store.Playlist, itemID)
	if err != nil {
		return Access{}, err
	}

	if playlist.OwnerID == userID {
		return Access{Owner: true, CanEdit: true, CanView: true}, nil
	}

	share, err := s.store.PlaylistShare.Query().
		Where(sharemodal.PlaylistID(playlist.ID), sharemodal.UserID(userID)).
		Only(ctx)
	if err != nil && !store.IsNotFound(err) {
		return Access{}, fmt.Errorf("failed to query playlist share: %w", err)
	}

	access := Access{CanView: playlist.OpenAccess}
	if share != nil {
		access.CanEdit = share.CanEdit
		access.CanView = true
	}

	return access, nil
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
				Where(entrymodal.PlaylistID(playlist.ID)).
				Exec(ctx); err != nil {
				return fmt.Errorf("failed to clear playlist entries: %w", err)
			}
			if err := appendEntries(ctx, tx.PlaylistEntry, playlist.ID, 0, *params.ItemIDs); err != nil {
				return err
			}
		}

		if params.Shares != nil {
			return replaceShares(ctx, tx, playlist.ID, *params.Shares)
		}

		return nil
	})
}

func (s *Service) Entries(ctx context.Context, itemID uuid.UUID, startIndex, limit int) ([]*Entry, int, error) {
	playlist, err := playlistByItem(ctx, s.store.Playlist, itemID)
	if err != nil {
		return nil, 0, err
	}

	entries := s.store.PlaylistEntry.Query().Where(entrymodal.PlaylistID(playlist.ID))

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
			Where(entrymodal.PlaylistID(playlist.ID)).
			Count(ctx)
		if err != nil {
			return fmt.Errorf("failed to count playlist entries: %w", err)
		}

		expanded, err := expand(ctx, tx.Item, itemIDs)
		if err != nil {
			return err
		}

		return appendEntries(ctx, tx.PlaylistEntry, playlist.ID, count, expanded)
	})
}

func (s *Service) RemoveEntries(ctx context.Context, itemID uuid.UUID, entryIDs []uuid.UUID) error {
	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		playlist, err := playlistByItem(ctx, tx.Playlist, itemID)
		if err != nil {
			return err
		}

		if _, err := tx.PlaylistEntry.Delete().
			Where(entrymodal.PlaylistID(playlist.ID), entrymodal.IDIn(entryIDs...)).
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

		entries := tx.PlaylistEntry.Query().Where(entrymodal.PlaylistID(playlist.ID))

		count, err := entries.Clone().Count(ctx)
		if err != nil {
			return fmt.Errorf("failed to count playlist entries: %w", err)
		}
		if newIndex < 0 || newIndex >= count {
			return fmt.Errorf("index %d is outside playlist %s", newIndex, itemID)
		}

		entry, err := entries.Where(entrymodal.ID(entryID)).Only(ctx)
		if err != nil {
			return fmt.Errorf("playlist %s has no entry %s: %w", itemID, entryID, err)
		}

		from := entry.SortOrder
		to := int32(newIndex)
		if from == to {
			return nil
		}

		shifted := tx.PlaylistEntry.Update().Where(entrymodal.PlaylistID(playlist.ID))
		if to < from {
			shifted = shifted.
				Where(entrymodal.SortOrderGTE(to), entrymodal.SortOrderLT(from)).
				AddSortOrder(1)
		} else {
			shifted = shifted.
				Where(entrymodal.SortOrderGT(from), entrymodal.SortOrderLTE(to)).
				AddSortOrder(-1)
		}
		if _, err := shifted.Save(ctx); err != nil {
			return fmt.Errorf("failed to shift playlist entries: %w", err)
		}

		if err := tx.PlaylistEntry.UpdateOne(entry).SetSortOrder(to).Exec(ctx); err != nil {
			return fmt.Errorf("failed to move playlist entry: %w", err)
		}

		return nil
	})
}

func (s *Service) Shares(ctx context.Context, itemID uuid.UUID) ([]*Share, error) {
	playlist, err := playlistByItem(ctx, s.store.Playlist, itemID)
	if err != nil {
		return nil, err
	}

	shares, err := s.store.PlaylistShare.Query().
		Where(sharemodal.PlaylistID(playlist.ID)).
		Order(sharemodal.ByCreatedAt(), sharemodal.ByID()).
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
		Where(sharemodal.PlaylistID(playlist.ID), sharemodal.UserID(userID)).
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

		return upsertShare(ctx, tx.PlaylistShare, playlist.ID, permission)
	})
}

func (s *Service) RemoveShare(ctx context.Context, itemID, userID uuid.UUID) error {
	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		playlist, err := playlistByItem(ctx, tx.Playlist, itemID)
		if err != nil {
			return err
		}

		if _, err := tx.PlaylistShare.Delete().
			Where(sharemodal.PlaylistID(playlist.ID), sharemodal.UserID(userID)).
			Exec(ctx); err != nil {
			return fmt.Errorf("failed to remove playlist share: %w", err)
		}

		return nil
	})
}

func playlistByItem(ctx context.Context, client *store.PlaylistClient, itemID uuid.UUID) (*Playlist, error) {
	playlist, err := client.Query().Where(playlistmodal.ItemID(itemID)).Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query playlist %s: %w", itemID, err)
	}

	return playlist, nil
}

func orderedEntries(ctx context.Context, client *store.PlaylistEntryClient, playlistID uuid.UUID) ([]*Entry, error) {
	entries, err := client.Query().
		Where(entrymodal.PlaylistID(playlistID)).
		Order(entrymodal.BySortOrder(), entrymodal.ByID()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query playlist entries: %w", err)
	}

	return entries, nil
}

// Jellyfin turns a folder id into the playable items beneath it, however deep,
// so adding an album or a series adds its tracks or episodes.
func expand(ctx context.Context, client *store.ItemClient, itemIDs []uuid.UUID) ([]uuid.UUID, error) {
	if len(itemIDs) == 0 {
		return nil, nil
	}

	records, err := client.Query().Where(itemmodal.IDIn(itemIDs...)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query playlist items: %w", err)
	}

	folders := make(map[uuid.UUID]bool, len(records))
	for _, record := range records {
		folders[record.ID] = record.IsFolder
	}

	expanded := make([]uuid.UUID, 0, len(itemIDs))
	for _, itemID := range itemIDs {
		if !folders[itemID] {
			expanded = append(expanded, itemID)
			continue
		}

		children, err := descendants(ctx, client, itemID)
		if err != nil {
			return nil, err
		}
		expanded = append(expanded, children...)
	}

	return expanded, nil
}

func descendants(ctx context.Context, client *store.ItemClient, folderID uuid.UUID) ([]uuid.UUID, error) {
	children, err := client.Query().
		Where(itemmodal.ParentID(folderID)).
		Order(itemmodal.BySortName(sql.OrderAsc())).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query folder children: %w", err)
	}

	expanded := make([]uuid.UUID, 0, len(children))
	for _, child := range children {
		if !child.IsFolder {
			expanded = append(expanded, child.ID)
			continue
		}

		nested, err := descendants(ctx, client, child.ID)
		if err != nil {
			return nil, err
		}
		expanded = append(expanded, nested...)
	}

	return expanded, nil
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
		Where(sharemodal.PlaylistID(playlistID)).
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to clear playlist shares: %w", err)
	}

	for _, permission := range permissions {
		if err := upsertShare(ctx, tx.PlaylistShare, playlistID, permission); err != nil {
			return err
		}
	}

	return nil
}

func upsertShare(ctx context.Context, client *store.PlaylistShareClient, playlistID uuid.UUID, permission Permission) error {
	if err := client.Create().
		SetPlaylistID(playlistID).
		SetUserID(permission.UserID).
		SetCanEdit(permission.CanEdit).
		OnConflictColumns(sharemodal.FieldPlaylistID, sharemodal.FieldUserID).
		UpdateCanEdit().
		UpdateUpdatedAt().
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to save playlist share: %w", err)
	}

	return nil
}
