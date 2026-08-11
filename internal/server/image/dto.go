package image

import (
	"io"
	"path/filepath"
	"strings"
)

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
}

func contentType(path string) string {
	if found, ok := contentTypes[strings.ToLower(filepath.Ext(path))]; ok {
		return found
	}

	return "application/octet-stream"
}
