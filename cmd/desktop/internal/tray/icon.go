package tray

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
)

// iconData is a 16x16 PNG icon for the system tray (blue).
var iconData = generateIcon(color.RGBA{R: 66, G: 133, B: 244, A: 255})

// iconHighlightData is a 16x16 PNG icon for the flashing state (red/orange).
var iconHighlightData = generateIcon(color.RGBA{R: 234, G: 67, B: 53, A: 255})

func generateIcon(fillColor color.RGBA) []byte {
	// Create a 16x16 image with a circle shape
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))

	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}

	// Draw a filled circle
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			// Simple circle approximation
			dx := float64(x) - 7.5
			dy := float64(y) - 7.5
			if dx*dx+dy*dy <= 49 { // radius ~7
				img.Set(x, y, fillColor)
			}
		}
	}

	// Draw a simple "O" letter in white in the center
	for y := 4; y < 12; y++ {
		for x := 5; x < 11; x++ {
			dx := float64(x) - 7.5
			dy := float64(y) - 7.5
			dist := dx*dx + dy*dy
			if dist >= 4 && dist <= 12 {
				img.Set(x, y, white)
			}
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// For Windows, systray needs ICO format. Let's provide a minimal ICO wrapper.
func init() {
	// Convert PNG to ICO format for Windows compatibility
	iconData = pngToICO(iconData)
	iconHighlightData = pngToICO(iconHighlightData)
}

// pngToICO wraps a PNG image in a minimal ICO container.
func pngToICO(pngData []byte) []byte {
	var buf bytes.Buffer

	// ICO header: reserved(2) + type(2) + count(2)
	binary.Write(&buf, binary.LittleEndian, uint16(0))    // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1))    // type: 1 = ICO
	binary.Write(&buf, binary.LittleEndian, uint16(1))    // image count

	// ICO directory entry
	buf.WriteByte(16)                                              // width
	buf.WriteByte(16)                                              // height
	buf.WriteByte(0)                                               // color palette
	buf.WriteByte(0)                                               // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1))             // color planes
	binary.Write(&buf, binary.LittleEndian, uint16(32))            // bits per pixel
	binary.Write(&buf, binary.LittleEndian, uint32(len(pngData)))  // size of image data
	binary.Write(&buf, binary.LittleEndian, uint32(22))            // offset to image data (6 + 16 = 22)

	// PNG data
	buf.Write(pngData)

	return buf.Bytes()
}
