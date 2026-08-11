package devices

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
)

func TestDeviceInfoDto(t *testing.T) {
	lastActivity := time.Date(2026, 8, 11, 9, 30, 0, 0, time.UTC)
	device := &store.Device{
		ClientID:       "client-1",
		Name:           "Living Room TV",
		CustomName:     "Big Screen",
		AppName:        "Jellyfin Web",
		AppVersion:     "10.10.0",
		IconURL:        "https://example.test/tv.png",
		LastActivityAt: lastActivity,
	}

	info := deviceInfoDto(device)

	if *info.Id != "client-1" {
		t.Errorf("Id = %q, want %q", *info.Id, "client-1")
	}
	if *info.Name != "Living Room TV" {
		t.Errorf("Name = %q, want %q", *info.Name, "Living Room TV")
	}
	if *info.CustomName != "Big Screen" {
		t.Errorf("CustomName = %q, want %q", *info.CustomName, "Big Screen")
	}
	if *info.AppName != "Jellyfin Web" {
		t.Errorf("AppName = %q, want %q", *info.AppName, "Jellyfin Web")
	}
	if *info.AppVersion != "10.10.0" {
		t.Errorf("AppVersion = %q, want %q", *info.AppVersion, "10.10.0")
	}
	if *info.IconUrl != "https://example.test/tv.png" {
		t.Errorf("IconUrl = %q, want %q", *info.IconUrl, "https://example.test/tv.png")
	}
	if !info.DateLastActivity.Equal(lastActivity) {
		t.Errorf("DateLastActivity = %v, want %v", *info.DateLastActivity, lastActivity)
	}
	if info.LastUserId != nil {
		t.Errorf("LastUserId = %v, want nil", *info.LastUserId)
	}
	if info.LastUserName != nil {
		t.Errorf("LastUserName = %v, want nil", *info.LastUserName)
	}
}

func TestDeviceInfoDtoLastUser(t *testing.T) {
	dean := &store.User{ID: uuid.New(), Name: "Dean"}
	device := &store.Device{ClientID: "client-1"}
	device.Edges.Sessions = []*store.Session{
		{},
		newSession(dean),
		newSession(&store.User{ID: uuid.New(), Name: "Older"}),
	}

	info := deviceInfoDto(device)

	if info.LastUserId == nil || *info.LastUserId != dean.ID {
		t.Errorf("LastUserId = %v, want %v", info.LastUserId, dean.ID)
	}
	if info.LastUserName == nil || *info.LastUserName != "Dean" {
		t.Errorf("LastUserName = %v, want %q", info.LastUserName, "Dean")
	}
}

func TestDeviceOptionsDto(t *testing.T) {
	options := deviceOptionsDto(&store.Device{ClientID: "client-1", CustomName: "Big Screen"})

	if *options.DeviceId != "client-1" {
		t.Errorf("DeviceId = %q, want %q", *options.DeviceId, "client-1")
	}
	if *options.CustomName != "Big Screen" {
		t.Errorf("CustomName = %q, want %q", *options.CustomName, "Big Screen")
	}
	if options.Id != nil {
		t.Errorf("Id = %v, want nil", *options.Id)
	}
}

func newSession(user *store.User) *store.Session {
	session := &store.Session{}
	session.Edges.User = user

	return session
}
