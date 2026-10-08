package services

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"atish/internal/apperr"
	"atish/internal/storage"

	"github.com/google/uuid"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// MediaSigner creates and verifies short-lived signed URLs so original storage
// locations are never exposed and photos can't be hot-linked forever.
type MediaSigner struct {
	key  []byte
	base string
}

func NewMediaSigner(key []byte, publicURL string) *MediaSigner {
	return &MediaSigner{key: key, base: strings.TrimRight(publicURL, "/")}
}

func (m *MediaSigner) sig(id uuid.UUID, exp int64) string {
	h := hmac.New(sha256.New, m.key)
	fmt.Fprintf(h, "%s|%d", id, exp)
	return hex.EncodeToString(h.Sum(nil))[:32]
}

// URL returns a signed URL valid until the end of the next UTC day. The expiry
// is bucketed by day so the same URL is reused all day and browsers can cache it.
func (m *MediaSigner) URL(id uuid.UUID) string {
	exp := (time.Now().Unix()/86400 + 2) * 86400
	return fmt.Sprintf("%s/media/%s?e=%d&s=%s", m.base, id, exp, m.sig(id, exp))
}

func (m *MediaSigner) Verify(id uuid.UUID, expStr, sig string) bool {
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return false
	}
	return hmac.Equal([]byte(m.sig(id, exp)), []byte(sig))
}

const (
	maxUploadBytes = 8 << 20
	maxImageEdge   = 1280
)

// ProcessImage decodes (jpeg/png/webp), downsizes and re-encodes to JPEG.
// Re-encoding strips EXIF metadata, including any GPS coordinates.
func ProcessImage(r io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(r, maxUploadBytes+1))
	if err != nil {
		return nil, apperr.BadRequest("invalid_image", "Could not read image")
	}
	if len(raw) > maxUploadBytes {
		return nil, apperr.BadRequest("image_too_large", "Image must be smaller than 8 MB")
	}
	switch http.DetectContentType(raw) {
	case "image/jpeg", "image/png", "image/webp":
	default:
		return nil, apperr.BadRequest("invalid_image", "Only JPEG, PNG or WebP images are allowed")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width < 100 || cfg.Height < 100 || cfg.Width*cfg.Height > 24_000_000 {
		return nil, apperr.BadRequest("invalid_image", "Image dimensions are not supported")
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, apperr.BadRequest("invalid_image", "Could not decode image")
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > maxImageEdge || h > maxImageEdge {
		scale := float64(maxImageEdge) / float64(max(w, h))
		nw, nh := int(float64(w)*scale), int(float64(h)*scale)
		dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
		img = dst
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, apperr.BadRequest("invalid_image", "Could not process image")
	}
	return out.Bytes(), nil
}

// FetchTelegramPhoto downloads a profile photo URL that came from signed
// Telegram init data. Only Telegram CDN hosts are allowed (SSRF protection).
func FetchTelegramPhoto(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" {
		return nil, errors.New("invalid photo url")
	}
	host := strings.ToLower(u.Hostname())
	if host != "t.me" && !strings.HasSuffix(host, ".telegram.org") && !strings.HasSuffix(host, ".t.me") && !strings.HasSuffix(host, ".telesco.pe") {
		return nil, errors.New("photo host not allowed")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(r *http.Request, _ []*http.Request) error {
		h := strings.ToLower(r.URL.Hostname())
		if r.URL.Scheme != "https" || (h != "t.me" && !strings.HasSuffix(h, ".telegram.org") && !strings.HasSuffix(h, ".t.me") && !strings.HasSuffix(h, ".telesco.pe")) {
			return errors.New("redirect not allowed")
		}
		return nil
	}}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("photo fetch status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxUploadBytes))
}

func PhotoKey(userID, photoID uuid.UUID) string {
	return fmt.Sprintf("photos/%s/%s.jpg", userID, photoID)
}

var _ storage.Storage = (*storage.Local)(nil)
