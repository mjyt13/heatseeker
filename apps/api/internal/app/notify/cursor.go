package notify

import (
	"encoding/base64"
	"encoding/binary"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/domain"
)

// makeCursor points at the last row of a page: its moment and id, so rows
// created in the same microsecond still page correctly.
func makeCursor(at time.Time, id uuid.UUID) string {
	buf := make([]byte, 8, 24)
	binary.BigEndian.PutUint64(buf, uint64(at.UnixMicro())) //nolint:gosec // round-trips parseCursor
	buf = append(buf, id[:]...)
	return base64.RawURLEncoding.EncodeToString(buf)
}

func parseCursor(s string) (time.Time, uuid.UUID, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || len(raw) != 24 {
		return time.Time{}, uuid.Nil, domain.Invalid("cursor", "malformed cursor")
	}
	id, err := uuid.FromBytes(raw[8:])
	if err != nil {
		return time.Time{}, uuid.Nil, domain.Invalid("cursor", "malformed cursor")
	}
	micros := int64(binary.BigEndian.Uint64(raw[:8])) //nolint:gosec // round-trips makeCursor
	return time.UnixMicro(micros).UTC(), id, nil
}
