package dto

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestItemDtosCarryTheAlbumAndArtistOfATrack(t *testing.T) {
	ctx := context.Background()

	connection, err := store.NewStore()
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}

	client := connection.Client()
	name := t.Name() + "-" + uuid.NewString()
	library, err := client.Library.Create().SetName(name).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	t.Cleanup(func() {
		if _, err := client.Item.Delete().Where(itemmodal.LibraryID(library.ID)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the items: %v", err)
		}
		if err := client.Library.DeleteOne(library).Exec(ctx); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	service := items.New(client)
	save := func(kind items.Kind, itemName string, parentID *uuid.UUID) *items.Item {
		record, err := service.SaveScanned(ctx, items.Scanned{
			LibraryID: library.ID,
			ParentID:  parentID,
			Kind:      kind,
			Name:      itemName,
			SortName:  itemName,
			Path:      "/" + itemName,
		})
		if err != nil {
			t.Fatalf("failed to save %q: %v", itemName, err)
		}

		return record
	}

	artist := save(itemmodal.KindMusicArtist, name+" Artist", nil)
	album := save(itemmodal.KindMusicAlbum, name+" Album", &artist.ID)
	track := save(itemmodal.KindAudio, name+" Track", &album.ID)

	converted, err := ItemDtos(ctx, service, []*items.Item{track, album})
	if err != nil {
		t.Fatalf("failed to convert the items: %v", err)
	}

	song := converted[0]
	if song.Album == nil || *song.Album != album.Name {
		t.Errorf("album = %v, want %s", song.Album, album.Name)
	}
	if song.AlbumId == nil || *song.AlbumId != album.ID {
		t.Errorf("album id = %v, want %s", song.AlbumId, album.ID)
	}
	if song.AlbumArtist == nil || *song.AlbumArtist != artist.Name {
		t.Errorf("album artist = %v, want %s", song.AlbumArtist, artist.Name)
	}
	if song.ArtistItems == nil || len(*song.ArtistItems) != 1 || *(*song.ArtistItems)[0].Id != artist.ID {
		t.Errorf("artist items = %v, want the artist", song.ArtistItems)
	}

	record := converted[1]
	if record.Album != nil {
		t.Errorf("an album has no album of its own, got %v", record.Album)
	}
	if record.AlbumArtist == nil || *record.AlbumArtist != artist.Name {
		t.Errorf("album artist = %v, want %s", record.AlbumArtist, artist.Name)
	}
}
