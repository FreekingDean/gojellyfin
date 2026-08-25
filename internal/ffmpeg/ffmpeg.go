package ffmpeg

import "os/exec"

type FFMpeg struct {
	probe string
}

func New() (*FFMpeg, error) {
	path, err := exec.LookPath("ffprobe")
	if err != nil {
		return nil, err
	}

	return &FFMpeg{
		probe: path,
	}, nil
}
