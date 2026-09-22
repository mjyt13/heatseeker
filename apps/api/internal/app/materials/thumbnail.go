package materials

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"

	"github.com/google/uuid"

	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/platform/imaging"
)

const (
	// thumbnailSide is the longer side of a thumbnail: sharp on a phone
	// screen across the card, about 50 KB.
	thumbnailSide = 640
	// thumbnailSourceMax bounds the pictures read to make one.
	thumbnailSourceMax = 40 << 20
	thumbnailMime      = "image/jpeg"
)

type thumbClaims struct {
	Material uuid.UUID `json:"m"`
	Version  uuid.UUID `json:"v"`
}

// ThumbnailURL is a link to a small JPEG of a picture, for lists and task
// cards; nil for other files. Like presigned links it needs no token; the
// client caches the picture by version id, so an expired link only matters
// for pictures never seen.
func (s *Service) ThumbnailURL(v *domain.MaterialVersion) *string {
	if !imaging.Supported(v.Mime) || v.SizeBytes > thumbnailSourceMax ||
		v.ScanStatus == domain.ScanPending || v.ScanStatus == domain.ScanInfected {
		return nil
	}
	token, err := s.thumbs.Sign(thumbClaims{Material: v.MaterialID, Version: v.ID}, s.Clock.Now().Add(s.cfg.ThumbnailTTL))
	if err != nil {
		return nil
	}
	u := fmt.Sprintf("%s/materials/%s/thumbnail?token=%s", s.cfg.APIBaseURL, v.MaterialID, url.QueryEscape(token))
	return &u
}

// Thumbnail returns the small JPEG of a picture, made on the first request
// and kept next to the files.
func (s *Service) Thumbnail(ctx context.Context, materialID uuid.UUID, token string) (*Content, error) {
	var c thumbClaims
	if err := s.thumbs.Verify(token, s.Clock.Now(), &c); err != nil {
		return nil, err
	}
	if c.Material != materialID {
		return nil, fmt.Errorf("%w: link does not match the material", domain.ErrUnauthorized)
	}
	v, err := s.Materials.GetVersion(ctx, c.Version)
	if err != nil {
		return nil, err
	}
	m, err := s.Materials.Get(ctx, materialID)
	if err != nil {
		return nil, err
	}
	if v.MaterialID != m.ID || m.Status == domain.MaterialDeleted || v.ScanStatus == domain.ScanInfected {
		return nil, domain.NotFound("thumbnail")
	}
	key := thumbnailKey(m.GroupID, v.ID)
	if body, info, err := s.Store.Open(ctx, key, 0, -1); err == nil {
		return thumbnailContent(body, info.Size), nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	src, err := s.openSource(ctx, m, v)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(src, thumbnailSourceMax+1))
	_ = src.Close()
	if err != nil {
		return nil, err
	}
	if len(data) > thumbnailSourceMax {
		return nil, domain.NotFound("thumbnail")
	}
	thumb, err := imaging.Thumbnail(data, thumbnailSide)
	if errors.Is(err, imaging.ErrUnsupported) {
		return nil, domain.NotFound("thumbnail")
	}
	if err != nil {
		return nil, err
	}
	if err := s.Store.Put(ctx, key, bytes.NewReader(thumb), int64(len(thumb)), thumbnailMime); err != nil {
		s.Log.Warn("store thumbnail", "version", v.ID, "err", err)
	}
	return thumbnailContent(io.NopCloser(bytes.NewReader(thumb)), int64(len(thumb))), nil
}

func thumbnailKey(groupID, versionID uuid.UUID) string {
	return fmt.Sprintf("groups/%s/thumbs/%s.jpg", groupID, versionID)
}

// thumbnailContent never changes for a version: clients may keep it.
func thumbnailContent(body io.ReadCloser, size int64) *Content {
	return &Content{
		Body: body, ContentType: thumbnailMime, ContentLength: size, FileName: "thumbnail.jpg", Inline: true,
		CacheControl: "private, max-age=2592000, immutable",
	}
}
