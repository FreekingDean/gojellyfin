package channels

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/store"
	channelmodel "github.com/FreekingDean/gojellyfin/internal/store/channel"
	entrymodel "github.com/FreekingDean/gojellyfin/internal/store/channelentry"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	"github.com/FreekingDean/gojellyfin/internal/store/predicate"
)

type (
	Channel = store.ChannelModel
	Order   = channelmodel.Order
)

var (
	ErrNotFound = errors.New("channel not found")
	ErrInvalid  = errors.New("invalid channel")
)

var airable = []items.Kind{itemmodel.KindMovie, itemmodel.KindSeries, itemmodel.KindSeason, itemmodel.KindEpisode}

type Definition struct {
	Number  int32
	Name    string
	Order   Order
	ItemIDs []uuid.UUID
}

type Service struct {
	store *store.Client
	items *items.Service
	now   func() time.Time
}

func New(client *store.Client, catalogue *items.Service) *Service {
	return &Service{store: client, items: catalogue, now: time.Now}
}

func (s *Service) List(ctx context.Context, viewer items.Viewer) ([]*Channel, error) {
	listed, err := s.store.Channel.Query().
		WithEntries(func(query *store.ChannelEntryQuery) { query.Order(entrymodel.ByPosition()) }).
		Order(channelmodel.ByNumber()).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list channels: %w", err)
	}

	visible := make([]*Channel, 0, len(listed))
	for _, channel := range listed {
		shown, err := s.visible(ctx, viewer, channel)
		if err != nil {
			return nil, err
		}
		if shown {
			visible = append(visible, channel)
		}
	}

	return visible, nil
}

func (s *Service) Channel(ctx context.Context, viewer items.Viewer, id uuid.UUID) (*Channel, error) {
	return s.one(ctx, viewer, channelmodel.ID(id))
}

func (s *Service) ByNumber(ctx context.Context, viewer items.Viewer, number int32) (*Channel, error) {
	return s.one(ctx, viewer, channelmodel.Number(number))
}

func (s *Service) one(ctx context.Context, viewer items.Viewer, which ...predicate.Channel) (*Channel, error) {
	channel, err := s.store.Channel.Query().
		Where(which...).
		WithEntries(func(query *store.ChannelEntryQuery) { query.Order(entrymodel.ByPosition()) }).
		OnlyModel(ctx)
	if store.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query a channel: %w", err)
	}

	shown, err := s.visible(ctx, viewer, channel)
	if err != nil {
		return nil, err
	}
	if !shown {
		return nil, ErrNotFound
	}

	return channel, nil
}

func (s *Service) visible(ctx context.Context, viewer items.Viewer, channel *Channel) (bool, error) {
	ids := EntryIDs(channel)
	found, err := s.items.ItemsByIDs(ctx, viewer, ids)
	if err != nil {
		return false, err
	}

	return len(found) == len(ids), nil
}

func EntryIDs(channel *Channel) []uuid.UUID {
	ids := make([]uuid.UUID, len(channel.Entries))
	for i, entry := range channel.Entries {
		ids[i] = entry.ItemID
	}

	return ids
}

func (s *Service) Create(ctx context.Context, definition Definition) (*Channel, error) {
	if err := s.validate(ctx, definition); err != nil {
		return nil, err
	}

	var created uuid.UUID
	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		channel, err := tx.Channel.Create().
			SetNumber(definition.Number).
			SetName(definition.Name).
			SetOrder(definition.Order).
			SetSeed(rand.Int64()).
			SetEpoch(s.now().UTC().Truncate(time.Second)).
			Save(ctx)
		if err != nil {
			return refused(err)
		}
		created = channel.ID

		return writeEntries(ctx, tx, channel.ID, definition.ItemIDs)
	})
	if err != nil {
		return nil, err
	}

	return s.Channel(ctx, items.Everyone, created)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, definition Definition) (*Channel, error) {
	if err := s.validate(ctx, definition); err != nil {
		return nil, err
	}

	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		err := tx.Channel.UpdateOneID(id).
			SetNumber(definition.Number).
			SetName(definition.Name).
			SetOrder(definition.Order).
			Exec(ctx)
		if store.IsNotFound(err) {
			return ErrNotFound
		}
		if err != nil {
			return refused(err)
		}

		if _, err := tx.ChannelEntry.Delete().Where(entrymodel.ChannelID(id)).Exec(ctx); err != nil {
			return fmt.Errorf("failed to clear the entries of channel %s: %w", id, err)
		}

		return writeEntries(ctx, tx, id, definition.ItemIDs)
	})
	if err != nil {
		return nil, err
	}

	return s.Channel(ctx, items.Everyone, id)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	err := s.store.Channel.DeleteOneID(id).Exec(ctx)
	if store.IsNotFound(err) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to delete channel %s: %w", id, err)
	}

	return nil
}

func (s *Service) validate(ctx context.Context, definition Definition) error {
	if definition.Number < 1 || definition.Number > maxNumber {
		return fmt.Errorf("%w: a channel number runs from 1 to %d", ErrInvalid, maxNumber)
	}
	if definition.Name == "" {
		return fmt.Errorf("%w: a channel needs a name", ErrInvalid)
	}
	if channelmodel.OrderValidator(definition.Order) != nil {
		return fmt.Errorf("%w: order is Shuffle or Sequential", ErrInvalid)
	}
	if len(definition.ItemIDs) == 0 {
		return fmt.Errorf("%w: a channel needs something to air", ErrInvalid)
	}

	unique := slices.Compact(slices.SortedFunc(slices.Values(definition.ItemIDs), func(a, b uuid.UUID) int {
		return slices.Compare(a[:], b[:])
	}))
	if len(unique) != len(definition.ItemIDs) {
		return fmt.Errorf("%w: a title is listed twice", ErrInvalid)
	}

	found, err := s.items.ItemsByIDs(ctx, items.Everyone, definition.ItemIDs)
	if err != nil {
		return err
	}
	for _, id := range definition.ItemIDs {
		item, held := found[id]
		if !held || !slices.Contains(airable, item.Kind) {
			return fmt.Errorf("%w: %s is not a movie, series, season or episode", ErrInvalid, id)
		}
	}

	return nil
}

func writeEntries(ctx context.Context, tx *store.Tx, channelID uuid.UUID, itemIDs []uuid.UUID) error {
	builders := make([]*store.ChannelEntryCreate, len(itemIDs))
	for i, itemID := range itemIDs {
		builders[i] = tx.ChannelEntry.Create().SetChannelID(channelID).SetItemID(itemID).SetPosition(int32(i))
	}

	if err := tx.ChannelEntry.CreateBulk(builders...).Exec(ctx); err != nil {
		return fmt.Errorf("failed to write the entries of channel %s: %w", channelID, err)
	}

	return nil
}

func refused(err error) error {
	if store.IsConstraintError(err) {
		return fmt.Errorf("%w: another channel has that number or name", ErrInvalid)
	}

	return fmt.Errorf("failed to save a channel: %w", err)
}

func (s *Service) Schedule(ctx context.Context, channel *Channel) (Schedule, error) {
	slots, err := s.slots(ctx, EntryIDs(channel))
	if err != nil {
		return Schedule{}, err
	}

	return NewSchedule(channel.Order, channel.Seed, channel.Epoch, slots), nil
}

func (s *Service) slots(ctx context.Context, roots []uuid.UUID) ([]Slot, error) {
	leaves, err := s.store.Item.Query().
		Where(
			itemmodel.DeletedAtIsNil(),
			itemmodel.KindIn(itemmodel.KindMovie, itemmodel.KindEpisode),
			itemmodel.Or(
				itemmodel.IDIn(roots...),
				itemmodel.ParentIDIn(roots...),
				itemmodel.HasParentWith(itemmodel.ParentIDIn(roots...)),
			),
		).
		WithParent().
		Order(
			itemmodel.ByParentIndexNumber(sql.OrderNullsLast()),
			itemmodel.ByIndexNumber(sql.OrderNullsLast()),
			itemmodel.BySortName(),
		).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query what a channel airs: %w", err)
	}

	ids := make([]uuid.UUID, len(leaves))
	for i, leaf := range leaves {
		ids[i] = leaf.ID
	}
	files, err := s.items.PlayedSources(ctx, ids)
	if err != nil {
		return nil, err
	}

	byRoot := map[uuid.UUID][]Slot{}
	for _, leaf := range leaves {
		file, held := files[leaf.ID]
		if !held || file.RunTimeTicks == nil || *file.RunTimeTicks <= 0 {
			continue
		}

		root := rootOf(leaf, roots)
		byRoot[root] = append(byRoot[root], Slot{
			ItemID:   leaf.ID,
			SourceID: file.ID,
			Length:   time.Duration(*file.RunTimeTicks) * tick,
		})
	}

	slots := []Slot{}
	for _, root := range roots {
		slots = append(slots, byRoot[root]...)
	}

	return slots, nil
}

func rootOf(leaf *items.Item, roots []uuid.UUID) uuid.UUID {
	if slices.Contains(roots, leaf.ID) {
		return leaf.ID
	}
	if leaf.ParentID != nil && slices.Contains(roots, *leaf.ParentID) {
		return *leaf.ParentID
	}
	if leaf.Parent != nil && leaf.Parent.ParentID != nil {
		return *leaf.Parent.ParentID
	}

	return uuid.Nil
}
