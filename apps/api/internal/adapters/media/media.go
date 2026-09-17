// Package media implements domain.MediaStore: S3-compatible object storage
// (MinIO) and the local filesystem. The driver is chosen by STORAGE_DRIVER.
package media

import (
	"context"
	"fmt"

	"heatseeker/api/internal/domain"
	"heatseeker/api/internal/platform/config"
)

// New builds the configured store (Factory).
func New(ctx context.Context, cfg config.Storage) (domain.MediaStore, error) {
	switch cfg.Driver {
	case "s3":
		return NewS3(ctx, cfg)
	case "local":
		return NewLocal(cfg.LocalRoot)
	default:
		return nil, fmt.Errorf("unknown storage driver %q", cfg.Driver)
	}
}
