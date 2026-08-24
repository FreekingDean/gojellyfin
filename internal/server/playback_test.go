package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/mediainfo"
	"github.com/FreekingDean/gojellyfin/internal/server/stream"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	librarymodal "github.com/FreekingDean/gojellyfin/internal/store/library"
	sourcemodal "github.com/FreekingDean/gojellyfin/internal/store/mediasource"
	streammodal "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
	"github.com/FreekingDean/gojellyfin/internal/transcode"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

const chromeProfile = `{
	"DirectPlayProfiles": [
		{"Container": "webm", "Type": "Video", "VideoCodec": "vp8,vp9,av1", "AudioCodec": "vorbis,opus"},
		{"Container": "mp4,m4v", "Type": "Video", "VideoCodec": "h264,vp8,vp9,av1", "AudioCodec": "aac,mp3,opus,flac,vorbis"},
		{"Container": "mkv", "Type": "Video", "VideoCodec": "h264,vp8,vp9,av1", "AudioCodec": "aac,mp3,opus,flac,vorbis"}
	]
}`

type playbackFixture struct {
	info    *mediainfo.Server
	streams *stream.Handler
	items   *items.Service
	library uuid.UUID
	token   string
	paths   map[string]string
}

func newPlaybackFixture(t *testing.T) *playbackFixture {
	t.Helper()

	if !transcode.Available() {
		t.Fatal("ffmpeg is not on PATH")
	}

	config, err := env.Load()
	if err != nil {
		t.Fatalf("failed to read the environment: %v", err)
	}

	connection, err := store.NewStore(config)
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}

	ctx := context.Background()
	client := connection.Client()
	unique := t.Name() + "-" + uuid.NewString()

	itemService := items.New(client)
	libraryService := libraries.New(client)
	sessionService := sessions.New(client)
	userService := users.New(client)

	library, err := libraryService.CreateLibrary(ctx, unique, librarymodal.CollectionTypeMovies, nil)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	user, err := userService.CreateUser(ctx, unique, "hash", true)
	if err != nil {
		t.Fatalf("failed to create the user: %v", err)
	}

	token := uuid.NewString()
	device := sessions.DeviceInfo{ID: unique, Name: "Test", AppName: "Test", AppVersion: "1"}
	if _, err := sessionService.Create(ctx, user.ID, token, device); err != nil {
		t.Fatalf("failed to create the session: %v", err)
	}

	t.Cleanup(func() {
		inLibrary := sourcemodal.HasItemWith(itemmodal.LibraryID(library.ID))
		if _, err := client.MediaStream.Delete().Where(streammodal.HasSourceWith(inLibrary)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the media streams: %v", err)
		}
		if _, err := client.MediaSource.Delete().Where(inLibrary).Exec(ctx); err != nil {
			t.Errorf("failed to delete the media sources: %v", err)
		}
		if err := libraryService.DeleteLibrary(ctx, library.ID); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := sessionService.DeleteByToken(ctx, token); err != nil {
			t.Errorf("failed to delete the session: %v", err)
		}
		if err := sessionService.RemoveDevice(ctx, unique); err != nil {
			t.Errorf("failed to delete the device: %v", err)
		}
		if err := userService.DeleteUser(ctx, user.ID); err != nil {
			t.Errorf("failed to delete the user: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	return &playbackFixture{
		info:    mediainfo.New(itemService),
		streams: stream.New(sessionService, itemService, transcode.NewEncoder(2, 0)),
		items:   itemService,
		library: library.ID,
		token:   token,
		paths:   map[string]string{},
	}
}

func (f *playbackFixture) rip(t *testing.T, name, audio string) uuid.UUID {
	t.Helper()

	return f.ripped(t, name, "libx264", "h264", audio)
}

func (f *playbackFixture) ripped(t *testing.T, name, encoder, video, audio string) uuid.UUID {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	generate := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=15:duration=4",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=4",
		"-c:v", encoder, "-pix_fmt", "yuv420p", "-g", "15",
		"-c:a", audio,
		"-shortest",
		path,
	)
	if output, err := generate.CombinedOutput(); err != nil {
		t.Skipf("this ffmpeg cannot build a %s/%s source: %v: %s", video, audio, err, output)
	}
	f.paths[name] = path

	ctx := context.Background()
	item, err := f.items.SaveScanned(ctx, items.Scanned{
		LibraryID:    f.library,
		Kind:         itemmodal.KindMovie,
		Key:          "movie:" + name + ":" + audio,
		Name:         name,
		SortName:     name,
		DateModified: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to save the item: %v", err)
	}

	source, err := f.items.SaveSource(ctx, items.ScannedSource{
		LibraryID:    f.library,
		ItemID:       item.ID,
		Path:         path,
		Name:         name,
		DateModified: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to save the source: %v", err)
	}

	err = f.items.SaveProbe(ctx, item, source, items.Probe{
		Container: strings.TrimPrefix(filepath.Ext(name), "."),
		Streams: []items.Stream{
			{Index: 0, Kind: streammodal.KindVideo, Codec: video},
			{Index: 1, Kind: streammodal.KindAudio, Codec: audio},
		},
	})
	if err != nil {
		t.Fatalf("failed to probe the source: %v", err)
	}

	return item.ID
}

func (f *playbackFixture) offer(t *testing.T, id uuid.UUID, profile string, startTicks int64) api.MediaSourceInfo {
	t.Helper()

	sources := *f.answer(t, id, profile, startTicks).MediaSources
	if len(sources) != 1 {
		t.Fatalf("got %d media sources, want 1", len(sources))
	}

	return sources[0]
}

func (f *playbackFixture) answer(t *testing.T, id uuid.UUID, profile string, startTicks int64) api.PlaybackInfoResponse {
	t.Helper()

	body := &api.PlaybackInfoDto{StartTimeTicks: apiutil.Ptr(startTicks)}
	if profile != "" {
		var declared api.DeviceProfile
		if err := json.Unmarshal([]byte(profile), &declared); err != nil {
			t.Fatalf("failed to read the profile: %v", err)
		}
		body.DeviceProfile = &declared
	}

	ctx := auth.ContextWithAuthorization(context.Background(), auth.Authorization{Token: f.token})
	response, err := f.info.GetPostedPlaybackInfo(ctx, api.GetPostedPlaybackInfoRequestObject{ItemId: id, JSONBody: body})
	if err != nil {
		t.Fatalf("failed to answer playback info: %v", err)
	}

	return api.PlaybackInfoResponse(response.(api.GetPostedPlaybackInfo200JSONResponse))
}

func (f *playbackFixture) follow(t *testing.T, rawURL string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, rawURL, nil)
	segments := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(segments) != 3 {
		t.Fatalf("path = %q, want /Videos/{itemId}/stream.{container}", request.URL.Path)
	}
	request.SetPathValue("itemId", segments[1])
	_, container, _ := strings.Cut(segments[2], ".")
	request.SetPathValue("container", container)

	recorder := httptest.NewRecorder()
	middleware.HttpCanonicalQuery(http.HandlerFunc(f.streams.Serve)).ServeHTTP(recorder, request)

	return recorder
}

func decoded(t *testing.T, body []byte) *ffmpeg.Probe {
	t.Helper()

	if len(body) == 0 {
		t.Fatal("the response has no body")
	}

	path := filepath.Join(t.TempDir(), "response")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("failed to write the response: %v", err)
	}

	probed, err := ffmpeg.ProbeFile(context.Background(), path)
	if err != nil {
		t.Fatalf("the response is not decodable: %v", err)
	}

	return probed
}

func fingerprint(t *testing.T, path, stream string) string {
	t.Helper()

	output, err := exec.Command("ffmpeg", "-v", "error", "-i", path, "-map", "0:"+stream, "-c", "copy", "-f", "md5", "-").Output()
	if err != nil {
		t.Fatalf("failed to fingerprint the %s of %s: %v", stream, path, err)
	}

	return strings.TrimSpace(string(output))
}

func received(t *testing.T, body []byte) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "received")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("failed to write the response: %v", err)
	}

	return path
}

func codecs(probed *ffmpeg.Probe) (string, string) {
	video, audio := "", ""
	for _, track := range probed.Streams {
		switch track.CodecType {
		case "video":
			video = track.CodecName
		case "audio":
			audio = track.CodecName
		}
	}

	return video, audio
}

func TestPlayback(t *testing.T) {
	t.Run("everything the client declared is handed over untouched", func(t *testing.T) {
		fixture := newPlaybackFixture(t)
		id := fixture.rip(t, "playable.mkv", "aac")

		source := fixture.offer(t, id, chromeProfile, 0)
		recorder := fixture.follow(t, apiutil.Deref(source.TranscodingUrl))

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body)
		}
		if got := recorder.Header().Get("Content-Type"); got != "video/x-matroska" {
			t.Errorf("content type = %q, want video/x-matroska", got)
		}
		if got := recorder.Header().Get("Accept-Ranges"); got != "bytes" {
			t.Errorf("accept ranges = %q, want bytes, so the client seeks without asking again", got)
		}

		file, err := os.ReadFile(fixture.paths["playable.mkv"])
		if err != nil {
			t.Fatalf("failed to read the source: %v", err)
		}
		if !bytes.Equal(recorder.Body.Bytes(), file) {
			t.Errorf("the response is %d bytes and the file is %d, want the file itself", recorder.Body.Len(), len(file))
		}
	})

	t.Run("a container the client cannot open costs a mux and nothing else", func(t *testing.T) {
		fixture := newPlaybackFixture(t)
		id := fixture.rip(t, "boxed.mov", "aac")

		source := fixture.offer(t, id, chromeProfile, 0)
		recorder := fixture.follow(t, apiutil.Deref(source.TranscodingUrl))

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body)
		}
		if got := recorder.Header().Get("Content-Type"); got != "video/mp4" {
			t.Errorf("content type = %q, want video/mp4", got)
		}

		probed := decoded(t, recorder.Body.Bytes())
		if !strings.Contains(probed.Format.FormatName, "mp4") {
			t.Errorf("container = %q, want mp4", probed.Format.FormatName)
		}

		video, audio := codecs(probed)
		if video != "h264" || audio != "aac" {
			t.Errorf("streams are %q and %q, want them unchanged", video, audio)
		}

		answered := received(t, recorder.Body.Bytes())
		for _, stream := range []string{"v:0", "a:0"} {
			source := fingerprint(t, fixture.paths["boxed.mov"], stream)
			if got := fingerprint(t, answered, stream); got != source {
				t.Errorf("%s was re-encoded: %s, want the source %s copied through", stream, got, source)
			}
		}
	})

	t.Run("audio the client cannot decode is the one stream converted", func(t *testing.T) {
		fixture := newPlaybackFixture(t)
		id := fixture.rip(t, "rip.mkv", "ac3")

		source := fixture.offer(t, id, chromeProfile, 0)
		recorder := fixture.follow(t, apiutil.Deref(source.TranscodingUrl))

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body)
		}
		if got := recorder.Header().Get("Content-Type"); got != "video/mp4" {
			t.Errorf("content type = %q, want video/mp4", got)
		}

		video, audio := codecs(decoded(t, recorder.Body.Bytes()))
		if video != "h264" {
			t.Errorf("video is %q, want h264", video)
		}
		if audio != "aac" {
			t.Errorf("audio is %q, want aac", audio)
		}

		answered := received(t, recorder.Body.Bytes())
		if got, want := fingerprint(t, answered, "v:0"), fingerprint(t, fixture.paths["rip.mkv"], "v:0"); got != want {
			t.Errorf("the picture was re-encoded: %s, want the source %s copied through", got, want)
		}
		if got, want := fingerprint(t, answered, "a:0"), fingerprint(t, fixture.paths["rip.mkv"], "a:0"); got == want {
			t.Error("the ac3 was copied through rather than converted")
		}
	})

	t.Run("a picture nothing can convert is refused rather than sent", func(t *testing.T) {
		fixture := newPlaybackFixture(t)
		id := fixture.ripped(t, "oddball.mkv", "mpeg4", "mpeg4", "aac")

		answer := fixture.answer(t, id, chromeProfile, 0)
		if got := len(*answer.MediaSources); got != 0 {
			t.Errorf("answered with %d sources, want none for a picture nothing here can make", got)
		}
		if got := apiutil.Deref(answer.ErrorCode); got != api.NoCompatibleStream {
			t.Errorf("error code = %q, want %q", got, api.NoCompatibleStream)
		}

	})

	t.Run("a client that declared nothing gets the file untouched", func(t *testing.T) {
		fixture := newPlaybackFixture(t)
		id := fixture.rip(t, "rip.mkv", "ac3")

		source := fixture.offer(t, id, "", 0)
		recorder := fixture.follow(t, apiutil.Deref(source.TranscodingUrl))

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body)
		}

		if _, audio := codecs(decoded(t, recorder.Body.Bytes())); audio != "ac3" {
			t.Errorf("audio is %q, want the ac3 source passed through", audio)
		}
	})

	t.Run("what the url promises is what the bytes are", func(t *testing.T) {
		fixture := newPlaybackFixture(t)

		for _, tc := range []struct {
			name        string
			file        string
			audio       string
			profile     string
			contentType string
		}{
			{name: "untouched", file: "untouched.mkv", audio: "aac", profile: chromeProfile, contentType: "video/x-matroska"},
			{name: "muxed", file: "muxed.mov", audio: "aac", profile: chromeProfile, contentType: "video/mp4"},
			{name: "converted", file: "converted.mkv", audio: "ac3", profile: chromeProfile, contentType: "video/mp4"},
			{name: "undeclared", file: "undeclared.mkv", audio: "ac3", profile: "", contentType: "video/x-matroska"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				id := fixture.rip(t, tc.file, tc.audio)
				source := fixture.offer(t, id, tc.profile, 0)

				raw := apiutil.Deref(source.TranscodingUrl)
				path, query, _ := strings.Cut(raw, "?")
				_, suffix, _ := strings.Cut(filepath.Base(path), ".")

				if named := apiutil.Deref(source.TranscodingContainer); named != suffix {
					t.Errorf("the source names %q and the url asks for %q", named, suffix)
				}
				if !strings.Contains(query, "container="+suffix) {
					t.Errorf("query = %q, want container=%s", query, suffix)
				}

				recorder := fixture.follow(t, raw)
				if got := recorder.Header().Get("Content-Type"); got != tc.contentType {
					t.Errorf("content type = %q, want %q", got, tc.contentType)
				}
			})
		}
	})

	t.Run("a seek starts the stream where the client asked", func(t *testing.T) {
		fixture := newPlaybackFixture(t)
		id := fixture.rip(t, "rip.mkv", "ac3")

		source := fixture.offer(t, id, chromeProfile, 30_000_000)
		raw := apiutil.Deref(source.TranscodingUrl)
		if !strings.Contains(raw, "startTimeTicks=30000000") {
			t.Fatalf("url = %q, want the position the client seeked to", raw)
		}

		recorder := fixture.follow(t, raw)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body)
		}

		whole := decoded(t, fixture.follow(t, apiutil.Deref(fixture.offer(t, id, chromeProfile, 0).TranscodingUrl)).Body.Bytes()).Format.Seconds()
		remaining := decoded(t, recorder.Body.Bytes()).Format.Seconds()

		if remaining <= 0 || remaining > whole-1 {
			t.Errorf("the response runs %.2fs of the %.2fs source, want the seek to have skipped most of it", remaining, whole)
		}
	})
}
