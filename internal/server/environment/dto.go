package environment

import (
	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func entryInfo(file filesystem.File, fullPath string) api.FileSystemEntryInfo {
	kind := api.FileSystemEntryTypeFile
	if file.Dir {
		kind = api.FileSystemEntryTypeDirectory
	}

	return api.FileSystemEntryInfo{
		Name: apiutil.Ptr(file.Name),
		Path: apiutil.Ptr(fullPath),
		Type: apiutil.Ptr(kind),
	}
}
