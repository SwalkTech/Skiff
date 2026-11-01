package ui

import (
	"bytes"
	"image"
	"image/draw"
	_ "image/png"
	"sync"

	_ "embed"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/app/iconpng"
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

func windowIcon() app.WindowIcon {
	icon, err := iconpng.Decode(logoPNG)
	if err != nil {
		return app.WindowIcon{}
	}
	return icon
}

func logoBitmap() vivaui.Bitmap {
	logoOnce.Do(func() {
		decoded, _, err := image.Decode(bytes.NewReader(logoPNG))
		if err != nil {
			return
		}
		bounds := decoded.Bounds()
		picture := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
		draw.Draw(picture, picture.Bounds(), decoded, bounds.Min, draw.Src)
		width, height := picture.Rect.Dx(), picture.Rect.Dy()
		pix := make([]byte, width*height*4)
		for i := 0; i < width*height; i++ {
			red := picture.Pix[i*4]
			green := picture.Pix[i*4+1]
			blue := picture.Pix[i*4+2]
			alpha := picture.Pix[i*4+3]
			pix[i*4] = byte(uint16(blue) * uint16(alpha) / 255)
			pix[i*4+1] = byte(uint16(green) * uint16(alpha) / 255)
			pix[i*4+2] = byte(uint16(red) * uint16(alpha) / 255)
			pix[i*4+3] = alpha
		}
		cachedLogo = vivaui.Bitmap{Pix: pix, Width: width, Height: height, Stride: width * 4, ID: 1, Version: 1}
	})
	return cachedLogo
}

var (
	logoOnce   sync.Once
	cachedLogo vivaui.Bitmap
)
