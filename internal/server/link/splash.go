package link

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"rsc.io/qr"
)

const (
	splashWidth   = 1920
	splashHeight  = 1080
	splashMargin  = 96
	splashModule  = 8
	splashHeading = 44
	splashBody    = 30
	splashLeading = 44
)

var (
	splashBackground = color.RGBA{R: 0x10, G: 0x10, B: 0x10, A: 0xff}
	splashInk        = color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
)

func (h *Handler) Splashscreen(deviceID string) ([]byte, error) {
	device := ""
	if deviceID != "" {
		device = DeviceToken(deviceID)
	}

	code, err := qr.Encode(h.Target(device), qr.M)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the link: %w", err)
	}

	canvas := image.NewRGBA(image.Rect(0, 0, splashWidth, splashHeight))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(splashBackground), image.Point{}, draw.Src)

	side := (code.Size + 2*quietZone) * splashModule
	corner := image.Pt(splashWidth-splashMargin-side, splashHeight-splashMargin-side)
	drawCode(canvas, code, corner)

	lines := []struct {
		text string
		face []byte
		size float64
	}{
		{"Sign in with your phone", gobold.TTF, splashHeading},
		{"Choose Quick Connect here, then scan this code", goregular.TTF, splashBody},
		{"or open " + h.address() + "/link", goregular.TTF, splashBody},
	}
	baseline := corner.Y - splashLeading*len(lines)
	for _, line := range lines {
		baseline += splashLeading
		if err := drawRight(canvas, line.text, line.face, line.size, splashWidth-splashMargin, baseline-splashLeading/2); err != nil {
			return nil, err
		}
	}

	var encoded bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(&encoded, canvas); err != nil {
		return nil, fmt.Errorf("failed to encode the splashscreen: %w", err)
	}

	return encoded.Bytes(), nil
}

func (h *Handler) address() string {
	return strings.TrimPrefix(strings.TrimPrefix(h.published, "https://"), "http://")
}

func drawCode(canvas *image.RGBA, code *qr.Code, corner image.Point) {
	side := (code.Size + 2*quietZone) * splashModule
	draw.Draw(canvas, image.Rectangle{Min: corner, Max: corner.Add(image.Pt(side, side))}, image.NewUniform(color.White), image.Point{}, draw.Src)

	for y := range code.Size {
		for x := range code.Size {
			if !code.Black(x, y) {
				continue
			}
			origin := corner.Add(image.Pt((x+quietZone)*splashModule, (y+quietZone)*splashModule))
			draw.Draw(canvas, image.Rectangle{Min: origin, Max: origin.Add(image.Pt(splashModule, splashModule))}, image.NewUniform(color.Black), image.Point{}, draw.Src)
		}
	}
}

func drawRight(canvas *image.RGBA, text string, ttf []byte, size float64, right, baseline int) error {
	parsed, err := opentype.Parse(ttf)
	if err != nil {
		return fmt.Errorf("failed to parse the splashscreen font: %w", err)
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return fmt.Errorf("failed to size the splashscreen font: %w", err)
	}
	defer func() { _ = face.Close() }()

	drawer := font.Drawer{Dst: canvas, Src: image.NewUniform(splashInk), Face: face}
	drawer.Dot = fixed.P(right-drawer.MeasureString(text).Ceil(), baseline)
	drawer.DrawString(text)

	return nil
}
