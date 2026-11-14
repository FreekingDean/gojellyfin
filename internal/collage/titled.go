package collage

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"strings"
	"sync"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/FreekingDean/gojellyfin/internal/shelves"
)

const (
	stacked       = 5
	stackLeft     = 440
	stackStep     = 76
	stackTop      = 100
	stackDrop     = 36
	stackHeight   = 330
	posterAspect  = 2.0 / 3.0
	backdropDim   = 0.35
	blurWidth     = 24
	blurHeight    = 14
	fadeReach     = 0.55
	fadeOpacity   = 235.0
	shadowDrop    = 12
	shadowShrink  = 8
	shadowOpacity = 170
	shadowPasses  = 2
	cornerRadius  = 12
	titleLeft     = 80
	titleTop      = 64
	titleLine     = 60
	titleSize     = 54.0
	titleWidth    = 340
	countSize     = 22.0
	countGap      = 14
	barLeft       = 56
	barWidth      = 6
	barInset      = 6
)

var (
	shade  = color.RGBA{R: 10, G: 10, B: 14, A: 255}
	accent = color.RGBA{R: 0, G: 164, B: 220, A: 255}
	ink    = color.RGBA{R: 250, G: 250, B: 250, A: 255}
	muted  = color.RGBA{R: 180, G: 190, B: 200, A: 255}

	typeface = sync.OnceValues(func() (*opentype.Font, error) { return opentype.Parse(gobold.TTF) })
)

func titled(name string, fresh int) layout {
	return func(posters []image.Image) ([]byte, error) {
		canvas := image.NewRGBA(image.Rect(0, 0, width, height))
		blurred(canvas, posters[0])
		faded(canvas)

		if err := caption(canvas, name, fresh); err != nil {
			return nil, err
		}

		wide := int(stackHeight * posterAspect)
		for index := min(len(posters), stacked) - 1; index >= 0; index-- {
			slot := stacked - 1 - index
			place(canvas, posters[index], image.Pt(stackLeft+slot*stackStep, stackTop+(slot%2)*stackDrop), wide, stackHeight)
		}

		written := &bytes.Buffer{}
		if err := jpeg.Encode(written, canvas, &jpeg.Options{Quality: quality}); err != nil {
			return nil, err
		}

		return written.Bytes(), nil
	}
}

func headline(fresh int) string {
	switch {
	case fresh <= 0:
		return ""
	case fresh >= shelves.FreshLimit:
		return fmt.Sprintf("%d+ new this week", shelves.FreshLimit)
	default:
		return fmt.Sprintf("%d new this week", fresh)
	}
}

func caption(canvas *image.RGBA, name string, fresh int) error {
	parsed, err := typeface()
	if err != nil {
		return err
	}

	words := strings.Fields(strings.ToUpper(name))
	size := titleSize
	for _, word := range words {
		measured, err := measure(parsed, word, size)
		if err != nil {
			return err
		}
		if measured > titleWidth {
			size *= float64(titleWidth) / float64(measured)
		}
	}

	baseline := titleTop + int(size)
	for _, word := range words {
		if err := write(canvas, parsed, word, size, ink, image.Pt(titleLeft, baseline)); err != nil {
			return err
		}
		baseline += titleLine
	}
	bar := image.Rect(barLeft, titleTop+barInset, barLeft+barWidth, titleTop+len(words)*titleLine-titleLine/3)
	draw.Draw(canvas, bar, image.NewUniform(accent), image.Point{}, draw.Src)

	text := headline(fresh)
	if text == "" {
		return nil
	}

	return write(canvas, parsed, text, countSize, muted, image.Pt(titleLeft+2, baseline-titleLine+countGap+int(countSize)))
}

func measure(parsed *opentype.Font, text string, size float64) (int, error) {
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return 0, err
	}
	defer func() { _ = face.Close() }()

	return font.MeasureString(face, text).Ceil(), nil
}

func write(canvas *image.RGBA, parsed *opentype.Font, text string, size float64, colour color.RGBA, at image.Point) error {
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return err
	}
	defer func() { _ = face.Close() }()

	drawer := &font.Drawer{Dst: canvas, Src: image.NewUniform(colour), Face: face, Dot: fixed.P(at.X, at.Y)}
	drawer.DrawString(text)

	return nil
}

func blurred(canvas *image.RGBA, poster image.Image) {
	small := image.NewRGBA(image.Rect(0, 0, blurWidth, blurHeight))
	draw.ApproxBiLinear.Scale(small, small.Bounds(), poster, cropped(poster.Bounds(), width, height), draw.Src, nil)
	for index := range small.Pix {
		if index%4 != 3 {
			small.Pix[index] = uint8(float64(small.Pix[index]) * backdropDim)
		}
	}
	draw.CatmullRom.Scale(canvas, canvas.Bounds(), small, small.Bounds(), draw.Src, nil)
}

func faded(canvas *image.RGBA) {
	reach := float64(width) * fadeReach
	for x := range int(reach) {
		cover := uint8(fadeOpacity * (1 - float64(x)/reach))
		column := image.Rect(x, 0, x+1, height)
		draw.DrawMask(canvas, column, image.NewUniform(shade), image.Point{}, image.NewUniform(color.Alpha{A: cover}), image.Point{}, draw.Over)
	}
}

func place(canvas *image.RGBA, poster image.Image, at image.Point, wide, tall int) {
	card := image.NewRGBA(image.Rect(0, 0, wide, tall))
	draw.CatmullRom.Scale(card, card.Bounds(), poster, cropped(poster.Bounds(), wide, tall), draw.Src, nil)
	rounded(card, cornerRadius)

	shadow(canvas, image.Rect(at.X, at.Y+shadowDrop, at.X+wide, at.Y+shadowDrop+tall))
	draw.Draw(canvas, card.Bounds().Add(at), card, image.Point{}, draw.Over)
}

func shadow(canvas *image.RGBA, under image.Rectangle) {
	small := image.NewRGBA(image.Rect(0, 0, width/shadowShrink, height/shadowShrink))
	scaled := image.Rect(under.Min.X/shadowShrink, under.Min.Y/shadowShrink, under.Max.X/shadowShrink, under.Max.Y/shadowShrink)
	draw.Draw(small, scaled, image.NewUniform(color.RGBA{A: shadowOpacity}), image.Point{}, draw.Src)
	for range shadowPasses {
		softened(small)
	}
	draw.CatmullRom.Scale(canvas, canvas.Bounds(), small, small.Bounds(), draw.Over, nil)
}

func softened(layer *image.RGBA) {
	bounds := layer.Bounds()
	alpha := make([]uint8, bounds.Dx()*bounds.Dy())
	for y := range bounds.Dy() {
		for x := range bounds.Dx() {
			total, count := 0, 0
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if image.Pt(x+dx, y+dy).In(bounds) {
						total += int(layer.Pix[layer.PixOffset(x+dx, y+dy)+3])
						count++
					}
				}
			}
			alpha[y*bounds.Dx()+x] = uint8(total / count)
		}
	}
	for index, value := range alpha {
		layer.Pix[index*4+3] = value
	}
}

func rounded(card *image.RGBA, radius int) {
	bounds := card.Bounds()
	for y := range radius {
		for x := range radius {
			dx, dy := float64(radius-x)-0.5, float64(radius-y)-0.5
			coverage := math.Max(0, math.Min(1, float64(radius)+0.5-math.Hypot(dx, dy)))
			for _, corner := range [][2]int{{x, y}, {bounds.Dx() - 1 - x, y}, {x, bounds.Dy() - 1 - y}, {bounds.Dx() - 1 - x, bounds.Dy() - 1 - y}} {
				offset := card.PixOffset(corner[0], corner[1])
				for channel := range 4 {
					card.Pix[offset+channel] = uint8(float64(card.Pix[offset+channel]) * coverage)
				}
			}
		}
	}
}

func cropped(bounds image.Rectangle, wide, tall int) image.Rectangle {
	if bounds.Dx()*tall > bounds.Dy()*wide {
		kept := bounds.Dy() * wide / tall
		inset := (bounds.Dx() - kept) / 2

		return image.Rect(bounds.Min.X+inset, bounds.Min.Y, bounds.Min.X+inset+kept, bounds.Max.Y)
	}

	kept := bounds.Dx() * tall / wide
	inset := (bounds.Dy() - kept) / 2

	return image.Rect(bounds.Min.X, bounds.Min.Y+inset, bounds.Max.X, bounds.Min.Y+inset+kept)
}
