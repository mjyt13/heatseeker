package gdrive

import (
	"errors"
	"net/http"
	"testing"

	"google.golang.org/api/googleapi"

	"heatseeker/api/internal/domain"
)

func TestMapErr(t *testing.T) {
	cases := []struct {
		name     string
		err      *googleapi.Error
		wantIs   error
		wantCode string
	}{
		{
			name:     "api disabled (legacy reason)",
			err:      &googleapi.Error{Code: http.StatusForbidden, Message: "Google Drive API has not been used", Errors: []googleapi.ErrorItem{{Reason: "accessNotConfigured"}}},
			wantIs:   domain.ErrUnavailable,
			wantCode: domain.CodeDriveAPIDisabled,
		},
		{
			name:     "api disabled (error info)",
			err:      &googleapi.Error{Code: http.StatusForbidden, Details: []any{map[string]any{"@type": "type.googleapis.com/google.rpc.ErrorInfo", "reason": "SERVICE_DISABLED"}}},
			wantIs:   domain.ErrUnavailable,
			wantCode: domain.CodeDriveAPIDisabled,
		},
		{
			name:   "no access to the file",
			err:    &googleapi.Error{Code: http.StatusForbidden, Errors: []googleapi.ErrorItem{{Reason: "insufficientFilePermissions"}}},
			wantIs: domain.ErrForbidden,
		},
		{
			name:   "not shared",
			err:    &googleapi.Error{Code: http.StatusNotFound},
			wantIs: domain.ErrNotFound,
		},
		{
			name:     "bad key",
			err:      &googleapi.Error{Code: http.StatusUnauthorized},
			wantIs:   domain.ErrUnavailable,
			wantCode: domain.CodeDriveAuthFailed,
		},
		{
			name:   "quota",
			err:    &googleapi.Error{Code: http.StatusForbidden, Errors: []googleapi.ErrorItem{{Reason: "storageQuotaExceeded"}}},
			wantIs: domain.ErrDriveQuota,
		},
		{
			name:   "rate limit",
			err:    &googleapi.Error{Code: http.StatusForbidden, Errors: []googleapi.ErrorItem{{Reason: "userRateLimitExceeded"}}},
			wantIs: domain.ErrRateLimited,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapErr(tc.err, "drive file")
			if !errors.Is(got, tc.wantIs) {
				t.Fatalf("mapErr = %v, want %v", got, tc.wantIs)
			}
			if code := domain.ErrorCode(got); code != tc.wantCode {
				t.Fatalf("code = %q, want %q", code, tc.wantCode)
			}
		})
	}
}
