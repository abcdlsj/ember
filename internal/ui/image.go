package ui

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"strings"
	"sync"
	"time"

	"ember/internal/logging"

	"github.com/charmbracelet/lipgloss"
	chafa "github.com/ploMP4/chafa-go"
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

func calculateRenderSize(imgWidth, imgHeight, maxWidth, maxHeight int) (int, int) {
	if imgWidth <= 0 || imgHeight <= 0 {
		return maxWidth, maxHeight
	}

	// 终端字符宽高比约 0.5，使用 2.0 作为修正系数
	// 即终端一个字符的高度约等于两个字符的宽度
	const terminalAspectRatio = 2.0

	// 计算图片的实际宽高比（考虑终端字符比例）
	imgAspect := float64(imgWidth) / float64(imgHeight) * terminalAspectRatio

	// 按高度适配：使用最大高度，计算对应宽度
	widthByHeight := int(float64(maxHeight) * imgAspect)

	// 按宽度适配：使用最大宽度，计算对应高度
	heightByWidth := int(float64(maxWidth) / imgAspect)

	// 选择不超出边界的方案
	if widthByHeight <= maxWidth {
		// 按高度适配不会超出宽度限制
		return widthByHeight, maxHeight
	} else {
		// 按宽度适配
		return maxWidth, heightByWidth
	}
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
