package ffmpeg

import (
	"errors"
	"os/exec"
)

var ErrNotAvailable = errors.New("ffprobe is not installed")

type FFMpeg struct {
	probe string
}

func New() *FFMpeg {
	path, _ := exec.LookPath("ffprobe")

	return &FFMpeg{probe: path}
}
