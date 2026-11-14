package collage

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func fanned(t *testing.T, posters []image.Image) image.Image {
	t.Helper()

	body, err := fan(posters)
	if err != nil {
		t.Fatalf("failed to fan: %v", err)
	}

	picture, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	return picture
}

func sampled(picture image.Image, x, y int) color.RGBA {
	red, green, blue, _ := picture.At(x, y).RGBA()

	return color.RGBA{R: uint8(red >> 8), G: uint8(green >> 8), B: uint8(blue >> 8), A: 255}
}

func shade(poster image.Image) color.RGBA {
	return sampled(poster, 0, 0)
}

func TestFan(t *testing.T) {
	t.Run("draws a card the size the client shows", func(t *testing.T) {
		if bounds := fanned(t, shades(len(slots))).Bounds(); bounds.Dx() != width || bounds.Dy() != height {
			t.Errorf("card = %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), width, height)
		}
	})

	t.Run("puts the first poster in front", func(t *testing.T) {
		posters := shades(len(slots))
		picture := fanned(t, posters)

		if got, want := sampled(picture, width/2, height/2), shade(posters[0]); !near(t, got, want) {
			t.Errorf("centre = %v, want the first poster, %v", got, want)
		}
	})

	t.Run("fans the next posters out to either side", func(t *testing.T) {
		posters := shades(len(slots))
		picture := fanned(t, posters)

		for index, x := range map[int]int{3: 820, 4: 140} {
			if got, want := sampled(picture, x, height/2+40), shade(posters[index]); !near(t, got, want) {
				t.Errorf("x %d = %v, want poster %d, %v", x, got, index, want)
			}
		}
	})

	t.Run("sets the stack over a dimmed backdrop of the front poster", func(t *testing.T) {
		posters := shades(len(slots))
		picture := fanned(t, posters)

		front := shade(posters[0])
		got := sampled(picture, 4, 4)
		dimmed := color.RGBA{R: uint8(float64(front.R) * backdropDim), G: uint8(float64(front.G) * backdropDim), B: uint8(float64(front.B) * backdropDim), A: 255}
		if !near(t, got, dimmed) {
			t.Errorf("corner = %v, want the front poster dimmed, %v", got, dimmed)
		}
	})

	t.Run("draws a single poster alone", func(t *testing.T) {
		posters := shades(1)
		picture := fanned(t, posters)

		if got, want := sampled(picture, width/2, height/2), shade(posters[0]); !near(t, got, want) {
			t.Errorf("centre = %v, want the poster, %v", got, want)
		}
		if got := sampled(picture, 820, height/2); near(t, got, shade(posters[0])) {
			t.Errorf("x 820 = %v, want the backdrop where no second poster is", got)
		}
	})
}
