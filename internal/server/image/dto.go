package image

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/items"
)

const immutable = "public, max-age=31536000, immutable"

var contentTypes = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
	".gif":  "image/gif",
	".bmp":  "image/bmp",
	".tbn":  "image/jpeg",
}

type imageFile struct {
	body        io.ReadCloser
	contentType string
	length      int64
	tag         string
	cacheable   bool
}

func openImage(record *items.Image, tag *string) (*imageFile, bool) {
	body, err := os.Open(record.Path)
	if err != nil {
		return nil, false
	}

	info, err := body.Stat()
	if err != nil {
		_ = body.Close()
		return nil, false
	}

	return &imageFile{
		body:        body,
		contentType: contentType(record.Path),
		length:      info.Size(),
		tag:         record.Tag,
		cacheable:   tag != nil && *tag == record.Tag,
	}, true
}

func (f *imageFile) withoutBody() *imageFile {
	_ = f.body.Close()
	f.body = nil

	return f
}

func (f *imageFile) write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", f.contentType)
	w.Header().Set("Content-Length", fmt.Sprint(f.length))
	w.Header().Set("ETag", strconv.Quote(f.tag))
	if f.cacheable {
		w.Header().Set("Cache-Control", immutable)
	}
	w.WriteHeader(http.StatusOK)

	if f.body == nil {
		return nil
	}
	defer func() { _ = f.body.Close() }()

	_, err := io.Copy(w, f.body)

	return err
}

func (f *imageFile) VisitGetItemImageResponse(w http.ResponseWriter) error {
	return f.write(w)
}

func (f *imageFile) VisitHeadItemImageResponse(w http.ResponseWriter) error {
	return f.write(w)
}

func (f *imageFile) VisitGetItemImageByIndexResponse(w http.ResponseWriter) error {
	return f.write(w)
}

func (f *imageFile) VisitHeadItemImageByIndexResponse(w http.ResponseWriter) error {
	return f.write(w)
}

func contentType(path string) string {
	if found, ok := contentTypes[strings.ToLower(filepath.Ext(path))]; ok {
		return found
	}

	return "application/octet-stream"
}
