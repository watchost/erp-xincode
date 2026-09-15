// Copyright 2026 zhouhouping. All Rights reserved.

// Package captcha 生成轻量数字图形验证码（仅标准库 image + x/image basicfont，
// 不依赖重型 captcha 库/字体文件）。答案由调用方存入 Redis 并做一次性校验。
package captcha

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	mrand "math/rand"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	width  = 90
	height = 40
	digits = 4
)

// Generate 返回验证码 ID（由调用方生成时也可忽略）、答案与 data URI 图片。
func Generate() (code string, dataURI string, err error) {
	var b [digits]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", err
	}
	code = fmt.Sprintf("%04d", int(b[0])%10000)

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// 浅灰底，深色字
	fill(img, color.RGBA{R: 245, G: 246, B: 248, A: 255})

	rng := mrand.New(mrand.NewSource(int64(b[1])<<56 | int64(b[2])<<32 | int64(b[3])))

	// 干扰线
	for i := 0; i < 4; i++ {
		drawLine(img,
			rng.Intn(width), rng.Intn(height),
			rng.Intn(width), rng.Intn(height),
			color.RGBA{R: uint8(150 + rng.Intn(80)), G: uint8(150 + rng.Intn(80)), B: uint8(150 + rng.Intn(80)), A: 255})
	}
	// 噪点
	for i := 0; i < 80; i++ {
		img.Set(rng.Intn(width), rng.Intn(height),
			color.RGBA{R: uint8(120 + rng.Intn(120)), G: uint8(120 + rng.Intn(120)), B: uint8(120 + rng.Intn(120)), A: 255})
	}

	// 数字（轻微上下抖动 + 深色）
	for i, ch := range code {
		d := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(color.RGBA{R: 33, G: 37, B: 41, A: 255}),
			Face: basicfont.Face7x13,
			Dot:  fixed.Point26_6{X: fixed.I(12 + i*18), Y: fixed.I(27 + rng.Intn(5) - 2)},
		}
		d.DrawString(string(ch))
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", "", err
	}
	dataURI = "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	return code, dataURI, nil
}

func fill(img *image.RGBA, c color.Color) {
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, c)
		}
	}
}

func drawLine(img *image.RGBA, x0, y0, x1, y1 int, c color.Color) {
	// Bresenham
	dx := absInt(x1 - x0)
	dy := -absInt(y1 - y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		img.Set(x0, y0, c)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
