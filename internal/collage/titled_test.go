package collage

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func drawn(t *testing.T, name string, fresh int, posters []image.Image) image.Image {
	t.Helper()

	body, err := titled(name, fresh)(posters)
	if err != nil {
		t.Fatalf("failed to draw: %v", err)
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

func brightest(picture image.Image, area image.Rectangle) uint8 {
	most := uint8(0)
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			pixel := sampled(picture, x, y)
			most = max(most, min(pixel.R, pixel.G, pixel.B))
		}
	}

	return most
}

func TestTitled(t *testing.T) {
	t.Run("draws a card the size the client shows", func(t *testing.T) {
		if bounds := drawn(t, "Recently Aired", 3, shades(stacked)).Bounds(); bounds.Dx() != width || bounds.Dy() != height {
			t.Errorf("card = %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), width, height)
		}
	})

	t.Run("puts the first poster in front, on the right", func(t *testing.T) {
		posters := shades(stacked)
		picture := drawn(t, "Recently Aired", 3, posters)

		if got, want := sampled(picture, 850, height/2), sampled(posters[0], 0, 0); !near(t, got, want) {
			t.Errorf("front = %v, want the first poster, %v", got, want)
		}
		if got, want := sampled(picture, stackLeft+20, stackTop+40), sampled(posters[stacked-1], 0, 0); !near(t, got, want) {
			t.Errorf("back = %v, want the last poster, %v", got, want)
		}
	})

	t.Run("writes the name in white on the dark left", func(t *testing.T) {
		picture := drawn(t, "Recently Aired", 0, shades(stacked))

		if most := brightest(picture, image.Rect(titleLeft, titleTop, titleWidth+titleLeft, titleTop+2*titleLine)); most < 200 {
			t.Errorf("brightest pixel under the title = %d, want lettering", most)
		}
		if got := sampled(picture, 20, height-20); got.R > 40 || got.G > 40 || got.B > 40 {
			t.Errorf("left edge = %v, want it dark enough to read the title on", got)
		}
	})

	t.Run("writes the count under the name only when something is new", func(t *testing.T) {
		under := image.Rect(titleLeft, titleTop+2*titleLine+countGap, titleLeft+200, titleTop+2*titleLine+countGap+2*int(countSize))

		if most := brightest(drawn(t, "Recently Aired", 0, shades(stacked)), under); most > 100 {
			t.Errorf("brightest pixel under the name with nothing new = %d, want nothing written", most)
		}
		if most := brightest(drawn(t, "Recently Aired", 4, shades(stacked)), under); most < 120 {
			t.Errorf("brightest pixel under the name with four new = %d, want the count", most)
		}
	})

	t.Run("draws one poster alone", func(t *testing.T) {
		posters := shades(1)
		picture := drawn(t, "New Releases", 1, posters)

		if got, want := sampled(picture, 850, height/2), sampled(posters[0], 0, 0); !near(t, got, want) {
			t.Errorf("front = %v, want the poster, %v", got, want)
		}
	})
}

func TestHeadline(t *testing.T) {
	for fresh, want := range map[int]string{0: "", 1: "1 new this week", 12: "12 new this week", 99: "99+ new this week", 140: "99+ new this week"} {
		if got := headline(fresh); got != want {
			t.Errorf("headline(%d) = %q, want %q", fresh, got, want)
		}
	}
}
