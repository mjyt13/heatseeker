package http

import (
	"errors"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"

	"heatseeker/api/internal/domain"
)

// apiErr maps domain errors to RFC 7807 responses. Unknown errors are logged
// and hidden behind a generic 500.
func apiErr(log *slog.Logger, err error) error {
	if err == nil {
		return nil
	}
	var ve *domain.ValidationError
	switch {
	case errors.As(err, &ve):
		return huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{
			Location: "body." + ve.Field,
			Message:  ve.Message,
		})
	case errors.Is(err, domain.ErrInvalid):
		return huma.Error422UnprocessableEntity(err.Error())
	case errors.Is(err, domain.ErrUnauthorized):
		return huma.Error401Unauthorized(err.Error())
	case errors.Is(err, domain.ErrForbidden):
		return huma.Error403Forbidden(err.Error())
	case errors.Is(err, domain.ErrNotFound):
		return huma.Error404NotFound(err.Error())
	case errors.Is(err, domain.ErrConflict):
		return huma.Error409Conflict(err.Error())
	case errors.Is(err, domain.ErrGone):
		return huma.Error410Gone(err.Error())
	case errors.Is(err, domain.ErrRateLimited):
		return huma.Error429TooManyRequests(err.Error())
	}
	if log == nil {
		log = slog.Default()
	}
	log.Error("unhandled error", "err", err)
	return huma.Error500InternalServerError("internal error")
}
