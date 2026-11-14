package collage

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func shades(count int) []image.Image {
	posters := make([]image.Image, 0, count)
	for index := range count {
		poster := image.NewRGBA(image.Rect(0, 0, 200, 300))
		shade := color.RGBA{R: uint8(20 + index*20), G: 40, B: uint8(220 - index*20), A: 255}
		for x := range 200 {
			for y := range 300 {
				poster.SetRGBA(x, y, shade)
			}
		}
		posters = append(posters, poster)
	}

	return posters
}

func at(t *testing.T, composed image.Image, cell int) color.RGBA {
	t.Helper()

	x := (cell%columns)*cellWidth + cellWidth/2
	y := (cell/columns)*cellHeight + cellHeight/2
	red, green, blue, _ := composed.At(x, y).RGBA()

	return color.RGBA{R: uint8(red >> 8), G: uint8(green >> 8), B: uint8(blue >> 8), A: 255}
}

func near(t *testing.T, got, want color.RGBA) bool {
	t.Helper()

	off := func(a, b uint8) int {
		if a > b {
			return int(a - b)
		}

		return int(b - a)
	}

	return off(got.R, want.R) <= 8 && off(got.G, want.G) <= 8 && off(got.B, want.B) <= 8
}

func decoded(t *testing.T, posters []image.Image) image.Image {
	t.Helper()

	body, err := compose(posters)
	if err != nil {
		t.Fatalf("failed to compose: %v", err)
	}

	composed, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	return composed
}

func TestCompose(t *testing.T) {
	t.Run("lays the posters out across the grid in order", func(t *testing.T) {
		posters := shades(cells)
		composed := decoded(t, posters)

		for cell := range cells {
			want := posters[cell].At(0, 0)
			red, green, blue, _ := want.RGBA()
			expected := color.RGBA{R: uint8(red >> 8), G: uint8(green >> 8), B: uint8(blue >> 8), A: 255}
			if got := at(t, composed, cell); !near(t, got, expected) {
				t.Errorf("cell %d = %v, want %v", cell, got, expected)
			}
		}
	})

	t.Run("cycles the posters it has to fill every cell", func(t *testing.T) {
		posters := shades(3)
		composed := decoded(t, posters)

		for cell := range cells {
			if got, want := at(t, composed, cell), at(t, composed, cell%3); !near(t, got, want) {
				t.Errorf("cell %d = %v, want the poster at cell %d, %v", cell, got, cell%3, want)
			}
		}
	})

	t.Run("crops a poster to the cell rather than squashing it", func(t *testing.T) {
		wide := image.NewRGBA(image.Rect(0, 0, 400, 100))
		for x := range 400 {
			for y := range 100 {
				shade := color.RGBA{R: 240, G: 240, B: 240, A: 255}
				if x < 100 || x >= 300 {
					shade = color.RGBA{R: 10, G: 10, B: 10, A: 255}
				}
				wide.SetRGBA(x, y, shade)
			}
		}

		composed := decoded(t, []image.Image{wide})

		red, green, blue, _ := composed.At(cellWidth/8, cellHeight/2).RGBA()
		edge := color.RGBA{R: uint8(red >> 8), G: uint8(green >> 8), B: uint8(blue >> 8), A: 255}
		if !near(t, edge, color.RGBA{R: 240, G: 240, B: 240, A: 255}) {
			t.Errorf("cell = %v, want the middle of the poster rather than its squashed edges", edge)
		}
	})
}
