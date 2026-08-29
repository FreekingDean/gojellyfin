package collage

import (
	"bytes"
	"image"
	"image/jpeg"

	_ "image/gif"
	_ "image/png"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	width      = 960
	height     = 540
	columns    = 5
	rows       = 2
	cells      = columns * rows
	cellWidth  = width / columns
	cellHeight = height / rows
	quality    = 90
)

func compose(posters []image.Image) ([]byte, error) {
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))

	for cell := range cells {
		poster := posters[cell%len(posters)]
		left := (cell % columns) * cellWidth
		top := (cell / columns) * cellHeight
		into := image.Rect(left, top, left+cellWidth, top+cellHeight)
		draw.CatmullRom.Scale(canvas, into, poster, filled(poster.Bounds()), draw.Src, nil)
	}

	written := &bytes.Buffer{}
	if err := jpeg.Encode(written, canvas, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}

	return written.Bytes(), nil
}

func filled(bounds image.Rectangle) image.Rectangle {
	if bounds.Dx()*cellHeight > bounds.Dy()*cellWidth {
		kept := bounds.Dy() * cellWidth / cellHeight
		inset := (bounds.Dx() - kept) / 2

		return image.Rect(bounds.Min.X+inset, bounds.Min.Y, bounds.Min.X+inset+kept, bounds.Max.Y)
	}

	kept := bounds.Dx() * cellHeight / cellWidth
	inset := (bounds.Dy() - kept) / 2

	return image.Rect(bounds.Min.X, bounds.Min.Y+inset, bounds.Max.X, bounds.Min.Y+inset+kept)
}
