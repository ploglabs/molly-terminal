package tui

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/ploglabs/molly-terminal/internal/model"
)

type ImageProtocol int

const (
	ProtocolNone   ImageProtocol = iota
	ProtocolITerm2
	ProtocolKitty
	ProtocolSixel // external tool (chafa / viu)
)

var protocol ImageProtocol
var detectOnce sync.Once
var mu sync.RWMutex

// InitImageProtocol forces a specific protocol before auto-detection runs.
// proto: "iterm2", "kitty", "sixel", "none", or "" / "auto" for auto-detect.
func InitImageProtocol(proto string) {
	switch strings.ToLower(strings.TrimSpace(proto)) {
	case "iterm2":
		protocol = ProtocolITerm2
		detectOnce.Do(func() {})
	case "kitty":
		protocol = ProtocolKitty
		detectOnce.Do(func() {})
	case "sixel":
		protocol = ProtocolSixel
		detectOnce.Do(func() {})
	case "none":
		protocol = ProtocolNone
		detectOnce.Do(func() {})
	}
}

type imageCacheEntry struct {
	data      []byte
	timestamp time.Time
}

var imageCache = make(map[string]imageCacheEntry)
var pendingFetches = make(map[string]bool)

// processedPNGCache stores resized/re-encoded PNG bytes for Kitty rendering.
type processedPNGEntry struct {
	data []byte
	w, h int
}

var processedPNGCache = make(map[string]processedPNGEntry)

// sixelCache stores rendered output from external tools (chafa/viu).
var sixelCache = make(map[string]string)
var sixelPending = make(map[string]bool)

func ImageProtocolString() string {
	detectOnce.Do(detectImageProtocol)
	switch protocol {
	case ProtocolITerm2:
		return "iterm2"
	case ProtocolKitty:
		return "kitty"
	case ProtocolSixel:
		return "sixel"
	default:
		return "none"
	}
}

func detectImageProtocol() {
	termProg := strings.ToLower(os.Getenv("TERM_PROGRAM"))
	termEnv := strings.ToLower(os.Getenv("TERM"))
	lcTerminal := strings.ToLower(os.Getenv("LC_TERMINAL"))

	if termProg == "iterm.app" || termProg == "iterm2" ||
		os.Getenv("ITERM_SESSION_ID") != "" ||
		lcTerminal == "iterm2" {
		protocol = ProtocolITerm2
		return
	}
	// WezTerm supports both iTerm2 and Kitty protocols; prefer iTerm2
	if termProg == "wezterm" {
		protocol = ProtocolITerm2
		return
	}
	if os.Getenv("KITTY_WINDOW_ID") != "" || strings.Contains(termEnv, "kitty") {
		protocol = ProtocolKitty
		return
	}
	// Ghostty supports the Kitty graphics protocol
	if os.Getenv("GHOSTTY_RESOURCES_DIR") != "" ||
		termProg == "ghostty" ||
		termEnv == "xterm-ghostty" {
		protocol = ProtocolKitty
		return
	}
	// External tool fallback for other terminals (sixel via chafa, or viu)
	if _, err := exec.LookPath("chafa"); err == nil {
		protocol = ProtocolSixel
		return
	}
	if _, err := exec.LookPath("viu"); err == nil {
		protocol = ProtocolSixel
		return
	}
	protocol = ProtocolNone
}

// wrapTmuxPassthrough wraps a terminal escape sequence for passthrough via tmux DCS.
// Required when the app is running inside tmux so the outer terminal receives the sequence.
func wrapTmuxPassthrough(seq string) string {
	// All ESC bytes inside the inner sequence must be doubled.
	inner := strings.ReplaceAll(seq, "\033", "\033\033")
	return "\033Ptmux;" + inner + "\033\\"
}

func insideTmux() bool {
	return os.Getenv("TMUX") != ""
}

func getCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".cache", "molly", "images")
	os.MkdirAll(dir, 0700)
	return dir
}

func imageCacheKey(url string) string {
	return url
}

func getCachedImage(url string) []byte {
	mu.RLock()
	defer mu.RUnlock()
	if entry, ok := imageCache[url]; ok {
		return entry.data
	}
	return nil
}

func setCachedImage(url string, data []byte) {
	mu.Lock()
	defer mu.Unlock()
	imageCache[url] = imageCacheEntry{
		data:      data,
		timestamp: time.Now(),
	}
}

func isPending(url string) bool {
	mu.RLock()
	defer mu.RUnlock()
	return pendingFetches[url]
}

func setPending(url string) {
	mu.Lock()
	defer mu.Unlock()
	pendingFetches[url] = true
}

func clearPending(url string) {
	mu.Lock()
	defer mu.Unlock()
	delete(pendingFetches, url)
}

func FetchImage(url string) {
	if url == "" || isPending(url) || getCachedImage(url) != nil {
		return
	}
	setPending(url)
	go func() {
		defer clearPending(url)
		var data []byte
		var err error
		if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
			data, err = downloadImage(url)
		} else {
			data, err = os.ReadFile(url)
		}
		if err != nil {
			return
		}
		setCachedImage(url, data)
		if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
			saveToDisk(url, data)
		}
	}()
}

func downloadImage(url string) ([]byte, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("image download HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func saveToDisk(url string, data []byte) {
	dir := getCacheDir()
	if dir == "" {
		return
	}
	fname := filepath.Join(dir, sanitizeFilename(url))
	_ = os.WriteFile(fname, data, 0600)
}

func sanitizeFilename(url string) string {
	sanitized := strings.NewReplacer(
		"/", "_", ":", "_", "?", "_", "&", "_", "=", "_",
	).Replace(url)
	if len(sanitized) > 200 {
		sanitized = sanitized[:200]
	}
	return sanitized
}

func isImageAttachment(att model.Attachment) bool {
	if strings.HasPrefix(att.ContentType, "image/") {
		return true
	}
	ext := strings.ToLower(filepath.Ext(att.Filename))
	return MimeTypeFromExt(ext) != ""
}

func resolvedContentType(att model.Attachment) string {
	if att.ContentType != "" {
		return att.ContentType
	}
	ext := strings.ToLower(filepath.Ext(att.Filename))
	return MimeTypeFromExt(ext)
}

func RenderAttachment(att model.Attachment) string {
	if len(att.Filename) == 0 {
		return ""
	}

	ct := resolvedContentType(att)
	if !strings.HasPrefix(ct, "image/") {
		return nonImageAttachment(att)
	}

	return imageAttachmentPlaceholder(att)
}

func nonImageAttachment(att model.Attachment) string {
	size := ""
	if att.Size > 0 {
		size = fmt.Sprintf(" (%s)", formatBytes(att.Size))
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Italic(true).
		PaddingLeft(3).
		Render(fmt.Sprintf("📎 %s%s", att.Filename, size))
}

func renderITerm2Image(att model.Attachment) string {
	url := att.ProxyURL
	if url == "" {
		url = att.URL
	}
	if url == "" {
		return nonImageAttachment(att)
	}

	data := getCachedImage(url)
	if data == nil {
		FetchImage(url)
		return imagePlaceholder(att, "downloading...")
	}

	b64 := base64.StdEncoding.EncodeToString(data)
	fnameB64 := base64.StdEncoding.EncodeToString([]byte(att.Filename))

	width := "auto"
	height := 3
	if att.Width > 0 && att.Height > 0 {
		height = maxInt(1, att.Height*3/att.Width)
		if height > 10 {
			height = 10
		}
	}

	seq := fmt.Sprintf("\033]1337;File=name=%s;size=%d;width=%s;height=%d;inline=1:%s\a",
		fnameB64, len(data), width, height, b64)
	if insideTmux() {
		return wrapTmuxPassthrough(seq)
	}
	return seq
}

// renderKittyImage renders any supported image format via the Kitty graphics protocol.
// Images are decoded, resized to ≤400×300, and re-encoded as PNG before transmission.
// This covers Kitty, Ghostty, and any other Kitty-protocol-capable terminal.
// Payload is automatically chunked into ≤4096-byte base64 chunks per the Kitty protocol.
func renderKittyImage(att model.Attachment) string {
	url := att.ProxyURL
	if url == "" {
		url = att.URL
	}
	if url == "" {
		return nonImageAttachment(att)
	}

	data := getCachedImage(url)
	if data == nil {
		FetchImage(url)
		return imagePlaceholder(att, "downloading...")
	}

	mu.RLock()
	pe, hasPE := processedPNGCache[url]
	mu.RUnlock()

	var pngData []byte
	var w, h int
	if hasPE {
		pngData, w, h = pe.data, pe.w, pe.h
	} else {
		var err error
		pngData, w, h, err = decodeAndEncodeAsPNG(data)
		if err != nil {
			return nonImageAttachment(att)
		}
		mu.Lock()
		processedPNGCache[url] = processedPNGEntry{data: pngData, w: w, h: h}
		mu.Unlock()
	}

	cols := 40
	rows := 3
	if w > 0 && h > 0 {
		rows = maxInt(1, h*cols/w/2)
		if rows > 10 {
			rows = 10
		}
	}

	b64 := base64.StdEncoding.EncodeToString(pngData)
	inTmux := insideTmux()

	var b strings.Builder
	const maxChunk = 4096
	for i := 0; i < len(b64); i += maxChunk {
		end := i + maxChunk
		if end > len(b64) {
			end = len(b64)
		}
		chunk := b64[i:end]
		more := 0
		if end < len(b64) {
			more = 1
		}
		var params string
		if i == 0 {
			params = fmt.Sprintf("a=T,q=2,f=100,s=%d,v=%d,c=%d,r=%d,m=%d", w, h, cols, rows, more)
		} else {
			params = fmt.Sprintf("m=%d", more)
		}
		seq := fmt.Sprintf("\033_G%s;%s\033\\", params, chunk)
		if inTmux {
			seq = wrapTmuxPassthrough(seq)
		}
		b.WriteString(seq)
	}
	return b.String()
}

// decodeAndEncodeAsPNG decodes any stdlib-supported image format (PNG, JPEG, GIF)
// and returns a PNG-encoded version resized to fit within 400×300 pixels.
func decodeAndEncodeAsPNG(data []byte) (pngData []byte, w, h int, err error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, err
	}

	img = resizeImageNN(img, 400, 300)
	bounds := img.Bounds()
	w = bounds.Dx()
	h = bounds.Dy()

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, 0, 0, err
	}
	return buf.Bytes(), w, h, nil
}

// resizeImageNN performs nearest-neighbor downscaling while preserving aspect ratio.
// Returns the original image unchanged if it already fits within maxW×maxH.
func resizeImageNN(src image.Image, maxW, maxH int) image.Image {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if srcW <= maxW && srcH <= maxH {
		return src
	}

	scaleW := float64(maxW) / float64(srcW)
	scaleH := float64(maxH) / float64(srcH)
	scale := scaleW
	if scaleH < scale {
		scale = scaleH
	}
	newW := int(float64(srcW) * scale)
	newH := int(float64(srcH) * scale)
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	dst := image.NewNRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		for x := 0; x < newW; x++ {
			sx := bounds.Min.X + x*srcW/newW
			sy := bounds.Min.Y + y*srcH/newH
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

// renderSixelImage renders via an external tool (chafa or viu) for terminals that
// support neither Kitty nor iTerm2 protocols. Rendering is async to avoid blocking
// the TUI; a placeholder is shown until the result is ready.
func renderSixelImage(att model.Attachment) string {
	url := att.ProxyURL
	if url == "" {
		url = att.URL
	}
	if url == "" {
		return nonImageAttachment(att)
	}

	mu.RLock()
	cached, hasCached := sixelCache[url]
	isPend := sixelPending[url]
	mu.RUnlock()

	if hasCached {
		return cached
	}

	data := getCachedImage(url)
	if data == nil {
		FetchImage(url)
		return imagePlaceholder(att, "downloading...")
	}

	if isPend {
		return imagePlaceholder(att, "rendering...")
	}

	mu.Lock()
	sixelPending[url] = true
	mu.Unlock()

	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	filename := att.Filename
	urlCopy := url

	go func() {
		defer func() {
			mu.Lock()
			delete(sixelPending, urlCopy)
			mu.Unlock()
		}()

		result := renderWithExternalTool(dataCopy, filename)
		if result != "" {
			mu.Lock()
			sixelCache[urlCopy] = result
			mu.Unlock()
		}
	}()

	return imagePlaceholder(att, "rendering...")
}

// renderWithExternalTool writes image data to a temp file and runs chafa or viu,
// capturing the terminal escape sequences they produce.
func renderWithExternalTool(data []byte, filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".img"
	}

	tmp, err := os.CreateTemp("", "molly-img-*"+ext)
	if err != nil {
		return ""
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return ""
	}
	tmp.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// chafa: try sixel first, fall back to unicode symbols
	if chafa, err := exec.LookPath("chafa"); err == nil {
		if out, err := exec.CommandContext(ctx, chafa, "--size=40x10", "-f", "sixel", tmpPath).Output(); err == nil && len(out) > 0 {
			return string(out)
		}
		if out, err := exec.CommandContext(ctx, chafa, "--size=40x10", tmpPath).Output(); err == nil && len(out) > 0 {
			return string(out)
		}
	}

	// viu: auto-detects best protocol or falls back to unicode blocks
	if viu, err := exec.LookPath("viu"); err == nil {
		if out, err := exec.CommandContext(ctx, viu, "-w", "40", tmpPath).Output(); err == nil && len(out) > 0 {
			return string(out)
		}
	}

	return ""
}

func imagePlaceholder(att model.Attachment, status string) string {
	size := ""
	if att.Size > 0 {
		size = fmt.Sprintf(" %s", formatBytes(att.Size))
	}
	dims := ""
	if att.Width > 0 && att.Height > 0 {
		dims = fmt.Sprintf(" %dx%d", att.Width, att.Height)
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#91a7ff")).
		PaddingLeft(3).
		Render(fmt.Sprintf("[img] %s [%s%s%s]", att.Filename, status, size, dims))
}

func imageAttachmentPlaceholder(att model.Attachment) string {
	parts := []string{att.Filename}
	if att.Size > 0 {
		parts = append(parts, formatBytes(att.Size))
	}
	if att.Width > 0 && att.Height > 0 {
		parts = append(parts, fmt.Sprintf("%dx%d", att.Width, att.Height))
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#91a7ff")).
		PaddingLeft(3).
		Render(strings.Join(parts, " "))
}

func formatBytes(b int) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func HasRenderableAttachments(atts []model.Attachment) bool {
	for _, a := range atts {
		if a.Filename != "" {
			return true
		}
	}
	return false
}

func shouldReRender() bool {
	mu.RLock()
	defer mu.RUnlock()
	return len(pendingFetches) > 0 || len(sixelPending) > 0
}

var mimeExtensions = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".bmp":  "image/bmp",
	".svg":  "image/svg+xml",
	".ico":  "image/x-icon",
	".tiff": "image/tiff",
	".tif":  "image/tiff",
}

func MimeTypeFromExt(ext string) string {
	if mt, ok := mimeExtensions[ext]; ok {
		return mt
	}
	return ""
}
