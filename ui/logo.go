package ui

import (
	"bytes"
	"image/png"

	_ "embed"

	"github.com/vivaui/viva/app"
	vivaui "github.com/vivaui/viva/ui"
)

//go:embed logo.png
var logoPNG []byte

func editHostID(index int) vivaui.ID {
	return editHosts.ID(vivaui.Index(index))
}

func deleteHostID(index int) vivaui.ID {
	return deleteHosts.ID(vivaui.Index(index))
}

func skiffMark() vivaui.Icon {
	return vivaui.Icon{
		ViewBox: 24,
		Stroke:  2,
		Cap:     vivaui.CapRound,
		Join:    vivaui.JoinRound,
		Commands: []vivaui.PathCommand{
			vivaui.MoveTo(3, 18),
			vivaui.QuadTo(12, 22, 21, 18),
			vivaui.MoveTo(7, 17),
			vivaui.LineTo(17, 17),
			vivaui.LineTo(15, 13),
			vivaui.LineTo(9, 13),
			vivaui.LineTo(7, 17),
			vivaui.MoveTo(12, 13),
			vivaui.LineTo(12, 4),
			vivaui.LineTo(18, 11),
			vivaui.LineTo(12, 12),
		},
	}
}

func windowIcon() app.WindowIcon {
	img, err := png.Decode(bytes.NewReader(logoPNG))
	if err != nil {
		return app.WindowIcon{}
	}
	bounds := img.Bounds()
	size := bounds.Dx()
	if bounds.Dy() < size {
		size = bounds.Dy()
	}
	if size > 256 {
		size = 256
	}
	rgba := make([]byte, size*size*4)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			red, green, blue, alpha := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			offset := (y*size + x) * 4
			if alpha == 0 {
				continue
			}
			rgba[offset] = byte(red * 255 / alpha)
			rgba[offset+1] = byte(green * 255 / alpha)
			rgba[offset+2] = byte(blue * 255 / alpha)
			rgba[offset+3] = byte(alpha >> 8)
		}
	}
	return app.WindowIcon{Size: size, RGBA: rgba}
}
