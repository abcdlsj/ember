package ui

import (
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"ember/internal/logging"

	"github.com/charmbracelet/lipgloss"
	chafa "github.com/ploMP4/chafa-go"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

var (
	imageCache   = make(map[string]string)
	imageCacheMu sync.RWMutex
)

func fetchImage(url string) (image.Image, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		logging.ImageError(url, 0, "", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		err = fmt.Errorf("image request failed with status %d", resp.StatusCode)
		logging.ImageError(url, resp.StatusCode, resp.Header.Get("Content-Type"), err)
		return nil, err
	}

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		logging.ImageError(url, resp.StatusCode, resp.Header.Get("Content-Type"), err)
	}
	return img, err
}

func RenderImage(urls []string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	filtered := make([]string, 0, len(urls))
	for _, url := range urls {
		if strings.TrimSpace(url) != "" {
			filtered = append(filtered, url)
		}
	}
	if len(filtered) == 0 {
		return renderPlaceholder(width, height)
	}

	cacheKey := fmt.Sprintf("%s|%dx%d", strings.Join(filtered, "\n"), width, height)
	imageCacheMu.RLock()
	if cached, ok := imageCache[cacheKey]; ok {
		imageCacheMu.RUnlock()
		return cached
	}
	imageCacheMu.RUnlock()

	for _, url := range filtered {
		img, err := fetchImage(url)
		if err != nil {
			continue
		}

		bounds := img.Bounds()
		imgWidth := bounds.Dx()
		imgHeight := bounds.Dy()

		renderWidth, renderHeight := calculateRenderSize(imgWidth, imgHeight, width, height)
		if renderWidth <= 0 || renderHeight <= 0 {
			continue
		}

		result := renderChafa(img, renderWidth, renderHeight)
		if strings.TrimSpace(result) == "" {
			continue
		}

		imageCacheMu.Lock()
		imageCache[cacheKey] = result
		imageCacheMu.Unlock()
		return result
	}

	placeholder := renderPlaceholder(width, height)
	imageCacheMu.Lock()
	imageCache[cacheKey] = placeholder
	imageCacheMu.Unlock()
	return placeholder
}

func RenderImageGrid(urlGroups [][]string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	filtered := make([][]string, 0, len(urlGroups))
	var keyParts []string
	for _, urls := range urlGroups {
		var group []string
		for _, url := range urls {
			if strings.TrimSpace(url) != "" {
				group = append(group, url)
			}
		}
		if len(group) > 0 {
			filtered = append(filtered, group)
			keyParts = append(keyParts, strings.Join(group, ","))
		}
	}
	if len(filtered) == 0 {
		return ""
	}

	const maxGridImages = 9
	if len(filtered) > maxGridImages {
		filtered = filtered[:maxGridImages]
		keyParts = keyParts[:maxGridImages]
	}

	cacheKey := fmt.Sprintf("grid|%s|%dx%d", strings.Join(keyParts, "\n"), width, height)
	imageCacheMu.RLock()
	if cached, ok := imageCache[cacheKey]; ok {
		imageCacheMu.RUnlock()
		return cached
	}
	imageCacheMu.RUnlock()

	images := make([]image.Image, 0, len(filtered))
	for _, urls := range filtered {
		for _, url := range urls {
			img, err := fetchImage(url)
			if err != nil {
				continue
			}
			images = append(images, img)
			break
		}
	}
	if len(images) == 0 {
		return ""
	}

	canvasWidth := width * 8
	canvasHeight := height * 8
	if canvasWidth < 8 {
		canvasWidth = 8
	}
	if canvasHeight < 8 {
		canvasHeight = 8
	}

	canvas := image.NewRGBA(image.Rect(0, 0, canvasWidth, canvasHeight))
	bg := image.NewUniform(color.RGBA{R: 30, G: 30, B: 34, A: 255})
	stddraw.Draw(canvas, canvas.Bounds(), bg, image.Point{}, stddraw.Src)

	cols := int(math.Ceil(math.Sqrt(float64(len(images)))))
	rows := int(math.Ceil(float64(len(images)) / float64(cols)))
	tileWidth := canvasWidth / cols
	tileHeight := canvasHeight / rows

	for i, img := range images {
		col := i % cols
		row := i / cols
		rect := image.Rect(col*tileWidth, row*tileHeight, (col+1)*tileWidth, (row+1)*tileHeight)
		if col == cols-1 {
			rect.Max.X = canvasWidth
		}
		if row == rows-1 {
			rect.Max.Y = canvasHeight
		}
		drawCover(canvas, rect, img)
	}

	result := renderChafa(canvas, width, height)
	if strings.TrimSpace(result) == "" {
		return ""
	}

	imageCacheMu.Lock()
	imageCache[cacheKey] = result
	imageCacheMu.Unlock()
	return result
}

func drawCover(dst *image.RGBA, dstRect image.Rectangle, src image.Image) {
	srcBounds := src.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()
	dstW := dstRect.Dx()
	dstH := dstRect.Dy()
	if srcW <= 0 || srcH <= 0 || dstW <= 0 || dstH <= 0 {
		return
	}

	srcAspect := float64(srcW) / float64(srcH)
	dstAspect := float64(dstW) / float64(dstH)
	crop := srcBounds
	if srcAspect > dstAspect {
		newW := int(float64(srcH) * dstAspect)
		x0 := srcBounds.Min.X + (srcW-newW)/2
		crop = image.Rect(x0, srcBounds.Min.Y, x0+newW, srcBounds.Max.Y)
	} else if srcAspect < dstAspect {
		newH := int(float64(srcW) / dstAspect)
		y0 := srcBounds.Min.Y + (srcH-newH)/2
		crop = image.Rect(srcBounds.Min.X, y0, srcBounds.Max.X, y0+newH)
	}

	xdraw.CatmullRom.Scale(dst, dstRect, src, crop, stddraw.Over, nil)
}

func calculateRenderSize(imgWidth, imgHeight, maxWidth, maxHeight int) (int, int) {
	if imgWidth <= 0 || imgHeight <= 0 {
		return maxWidth, maxHeight
	}

	const terminalAspectRatio = 2.0
	imgAspect := float64(imgWidth) / float64(imgHeight) * terminalAspectRatio
	widthByHeight := int(float64(maxHeight) * imgAspect)
	heightByWidth := int(float64(maxWidth) / imgAspect)
	if widthByHeight <= maxWidth {
		return widthByHeight, maxHeight
	}
	return maxWidth, heightByWidth
}

func renderChafa(img image.Image, width, height int) string {
	bounds := img.Bounds()
	imgWidth := bounds.Dx()
	imgHeight := bounds.Dy()

	pixels := make([]uint8, imgWidth*imgHeight*4)
	idx := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			pixels[idx] = uint8(r >> 8)
			pixels[idx+1] = uint8(g >> 8)
			pixels[idx+2] = uint8(b >> 8)
			pixels[idx+3] = uint8(a >> 8)
			idx += 4
		}
	}

	ccfg := chafa.CanvasConfigNew()
	defer chafa.CanvasConfigUnref(ccfg)

	chafa.CanvasConfigSetGeometry(ccfg, int32(width), int32(height))
	chafa.CanvasConfigSetCellGeometry(ccfg, 8, 8)
	chafa.CanvasConfigSetCanvasMode(ccfg, chafa.CHAFA_CANVAS_MODE_TRUECOLOR)
	chafa.CanvasConfigSetColorSpace(ccfg, chafa.CHAFA_COLOR_SPACE_DIN99D)
	chafa.CanvasConfigSetPreprocessingEnabled(ccfg, true)
	chafa.CanvasConfigSetWorkFactor(ccfg, 1.0)

	symbolMap := chafa.SymbolMapNew()
	defer chafa.SymbolMapUnref(symbolMap)
	chafa.SymbolMapAddByTags(symbolMap, chafa.CHAFA_SYMBOL_TAG_BLOCK|chafa.CHAFA_SYMBOL_TAG_HALF|chafa.CHAFA_SYMBOL_TAG_QUAD)
	chafa.CanvasConfigSetSymbolMap(ccfg, symbolMap)

	canvas := chafa.CanvasNew(ccfg)
	defer chafa.CanvasUnRef(canvas)

	chafa.CanvasDrawAllPixels(
		canvas,
		chafa.CHAFA_PIXEL_RGBA8_UNASSOCIATED,
		pixels,
		int32(imgWidth),
		int32(imgHeight),
		int32(imgWidth*4),
	)

	termDb := chafa.TermDbGetDefault()
	termInfo := chafa.TermDbGetFallbackInfo(termDb)
	defer chafa.TermInfoUnref(termInfo)

	gstr := chafa.CanvasPrint(canvas, termInfo)
	result := strings.TrimSuffix(gstr.String(), "\n")

	return result
}

func renderPlaceholder(width, height int) string {
	style := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Background(lipgloss.Color("237")).
		Foreground(lipgloss.Color("244")).
		Align(lipgloss.Center, lipgloss.Center)

	return style.Render("No Image")
}

func ClearImageCache() {
	imageCacheMu.Lock()
	imageCache = make(map[string]string)
	imageCacheMu.Unlock()
}
