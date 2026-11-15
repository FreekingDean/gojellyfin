package ffmpeg

import (
	"errors"
	"os/exec"
)

var ErrNotAvailable = errors.New("ffprobe is not installed")

type FFMpeg struct {
	probe  string
	decode string
}

func New() *FFMpeg {
	probe, _ := exec.LookPath("ffprobe")
	decode, _ := exec.LookPath("ffmpeg")

	return &FFMpeg{probe: probe, decode: decode}
}
