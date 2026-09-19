package gdrive

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"

	"golang.org/x/oauth2"

	"heatseeker/api/internal/domain"
)

func TestEmailFromIDToken(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"email":"head@example.com","sub":"1"}`))
	if got, err := emailFromIDToken("h." + payload + ".sig"); err != nil || got != "head@example.com" {
		t.Fatalf("email = %q, %v", got, err)
	}
	noEmail := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"1"}`))
	for _, bad := range []string{"", "a.b", "h." + noEmail + ".sig", "h.!!!.sig"} {
		if _, err := emailFromIDToken(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestHasScope(t *testing.T) {
	granted := "openid https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/drive"
	if !hasScope(granted, "https://www.googleapis.com/auth/drive") {
		t.Error("drive scope not found")
	}
	if hasScope("openid https://www.googleapis.com/auth/drive.file", "https://www.googleapis.com/auth/drive") {
		t.Error("drive.file must not count as drive")
	}
}

func TestAsTokenErr(t *testing.T) {
	invalidGrant := fmt.Errorf("Get: %w", &oauth2.RetrieveError{ErrorCode: "invalid_grant"})
	if err := asTokenErr(invalidGrant, false); domain.ErrorCode(err) != domain.CodeDrivePublisherRevoked {
		t.Errorf("revoked refresh token: %v", err)
	}
	if err := asTokenErr(invalidGrant, true); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("reused sign-in code: %v", err)
	}
	if err := asTokenErr(&oauth2.RetrieveError{ErrorCode: "invalid_client"}, false); err == nil || !strings.Contains(err.Error(), "invalid_client") {
		t.Errorf("other token error: %v", err)
	}
	if err := asTokenErr(errors.New("network"), false); err != nil {
		t.Errorf("non-token error converted: %v", err)
	}
	// mapErr sees the token failure before anything else.
	if err := mapErr(invalidGrant, "drive upload"); domain.ErrorCode(err) != domain.CodeDrivePublisherRevoked {
		t.Errorf("mapErr: %v", err)
	}
}
