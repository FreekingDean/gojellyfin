package collage

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"math"

	"golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

const (
	backdropDim   = 0.45
	blurWidth     = 24
	blurHeight    = 14
	shadowDrop    = 12
	shadowShrink  = 8
	cornerRadius  = 10
	posterAspect  = 2.0 / 3.0
	shadowOpacity = 170
	shadowPasses  = 2
)

type slot struct {
	degrees float64
	centreX float64
	scale   float64
}

var slots = []slot{
	{degrees: 0, centreX: 480, scale: 0.80},
	{degrees: 7, centreX: 620, scale: 0.70},
	{degrees: -7, centreX: 340, scale: 0.70},
	{degrees: 14, centreX: 760, scale: 0.62},
	{degrees: -14, centreX: 200, scale: 0.62},
}

func fan(posters []image.Image) ([]byte, error) {
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	blurred(canvas, posters[0])

	shown := min(len(posters), len(slots))
	for index := shown - 1; index >= 0; index-- {
		place(canvas, posters[index], slots[index])
	}

	written := &bytes.Buffer{}
	if err := jpeg.Encode(written, canvas, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}

	return written.Bytes(), nil
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

func place(canvas *image.RGBA, poster image.Image, at slot) {
	tall := int(height * at.scale)
	wide := int(float64(tall) * posterAspect)

	card := image.NewRGBA(image.Rect(0, 0, wide, tall))
	draw.CatmullRom.Scale(card, card.Bounds(), poster, cropped(poster.Bounds(), wide, tall), draw.Src, nil)
	rounded(card, cornerRadius)

	centreY := float64(height)/2 + math.Abs(at.degrees)*3
	shadow(canvas, wide, tall, at, centreY+shadowDrop)
	draw.CatmullRom.Transform(canvas, turned(wide, tall, at.degrees, at.centreX, centreY), card, card.Bounds(), draw.Over, nil)
}

func shadow(canvas *image.RGBA, wide, tall int, at slot, centreY float64) {
	small := image.NewRGBA(image.Rect(0, 0, width/shadowShrink, height/shadowShrink))
	block := image.NewUniform(color.RGBA{A: shadowOpacity})
	sw, sh := wide/shadowShrink, tall/shadowShrink
	draw.ApproxBiLinear.Transform(small, turned(sw, sh, at.degrees, at.centreX/shadowShrink, centreY/shadowShrink), block, image.Rect(0, 0, sw, sh), draw.Over, nil)
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

func turned(wide, tall int, degrees, centreX, centreY float64) f64.Aff3 {
	radians := degrees * math.Pi / 180
	cos, sin := math.Cos(radians), math.Sin(radians)
	halfW, halfH := float64(wide)/2, float64(tall)/2

	return f64.Aff3{
		cos, -sin, centreX - (cos*halfW - sin*halfH),
		sin, cos, centreY - (sin*halfW + cos*halfH),
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
