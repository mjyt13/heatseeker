package drive

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"heatseeker/api/internal/authz"
	"heatseeker/api/internal/domain"
)

// oauthStateTTL bounds the time between "connect" and Google's redirect.
const oauthStateTTL = 15 * time.Minute

type oauthState struct {
	Group uuid.UUID `json:"g"`
	User  uuid.UUID `json:"u"`
}

func (s *Service) publisherAvailable() bool {
	return s.Authorizer != nil && s.Secrets != nil && s.states != nil
}

func errOAuthNotConfigured() error {
	return domain.WithCode(domain.CodeDriveOAuthNotConfigured,
		domain.Unavailable("connecting a Google account is not configured on this server (GOOGLE_OAUTH_CLIENT_ID/SECRET)"))
}

// StartPublisher returns the Google sign-in page that connects the account
// publishing uploads to the group's Drive folder (D34).
func (s *Service) StartPublisher(ctx context.Context, actorID, groupID uuid.UUID) (string, error) {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return "", err
	}
	if err := actor.Require(authz.DriveManage); err != nil {
		return "", err
	}
	if !s.publisherAvailable() {
		return "", errOAuthNotConfigured()
	}
	state, err := s.states.Sign(oauthState{Group: groupID, User: actorID}, s.Clock.Now().Add(oauthStateTTL))
	if err != nil {
		return "", err
	}
	return s.Authorizer.AuthURL(state), nil
}

// FinishPublisher completes the sign-in Google redirected back with. The
// account must be able to add files to the connected folder. It returns the
// stored publisher.
func (s *Service) FinishPublisher(ctx context.Context, state, code string) (*domain.DrivePublisher, error) {
	if !s.publisherAvailable() {
		return nil, errOAuthNotConfigured()
	}
	var st oauthState
	if err := s.states.Verify(state, s.Clock.Now(), &st); err != nil {
		return nil, err
	}
	// Rights may have changed while the user was on Google's pages.
	actor, err := s.Access.Actor(ctx, st.User, st.Group)
	if err != nil {
		return nil, err
	}
	if err := actor.Require(authz.DriveManage); err != nil {
		return nil, err
	}
	grant, err := s.Authorizer.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}
	if err := s.checkPublisherAccess(ctx, st.Group, grant); err != nil {
		if rerr := s.Authorizer.Revoke(ctx, grant.RefreshToken); rerr != nil {
			s.Log.Warn("revoke rejected publisher token", "group", st.Group, "err", rerr)
		}
		return nil, err
	}
	sealed, err := s.Secrets.Seal([]byte(grant.RefreshToken), st.Group[:])
	if err != nil {
		return nil, err
	}
	var pub *domain.DrivePublisher
	err = s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		previous, err := s.Repo.GetPublisher(ctx, st.Group)
		switch {
		case err == nil:
			s.Tx.AfterCommit(ctx, func() { s.revokeSealed(previous) })
		case !errors.Is(err, domain.ErrNotFound):
			return err
		}
		pub, err = s.Repo.UpsertPublisher(ctx, domain.DrivePublisher{
			GroupID: st.Group, Email: grant.Email, RefreshTokenEnc: sealed, Scopes: grant.Scopes, ConnectedBy: &st.User,
		})
		if err != nil {
			return err
		}
		return s.emit(ctx, st.Group, domain.EventDrivePublisherConnected, &st.User, st.Group,
			map[string]any{"email": grant.Email}, true)
	})
	if err != nil {
		return nil, err
	}
	return pub, nil
}

// checkPublisherAccess makes sure the account can add files to the group's
// folder, when one is connected.
func (s *Service) checkPublisherAccess(ctx context.Context, groupID uuid.UUID, grant *domain.DriveGrant) error {
	conn, err := s.Repo.GetConnectionByGroup(ctx, groupID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	client, err := s.Authorizer.Client(ctx, grant.RefreshToken)
	if err != nil {
		return err
	}
	folder, err := client.GetFile(ctx, conn.RootFolderID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) && !errors.Is(err, domain.ErrForbidden) {
		return err
	}
	if err != nil || !folder.CanAddChild {
		return domain.WithCode(domain.CodeDrivePublisherNoAccess, domain.Invalid("account",
			fmt.Sprintf("%s cannot add files to the folder %q: sign in with its owner or share it as an editor", grant.Email, conn.RootFolderName)))
	}
	return nil
}

// DisconnectPublisher forgets the publishing account and revokes its token.
func (s *Service) DisconnectPublisher(ctx context.Context, actorID, groupID uuid.UUID) error {
	actor, err := s.Access.Actor(ctx, actorID, groupID)
	if err != nil {
		return err
	}
	if err := actor.Require(authz.DriveManage); err != nil {
		return err
	}
	return s.Tx.RunInTx(ctx, func(ctx context.Context) error {
		pub, err := s.Repo.GetPublisher(ctx, groupID)
		if err != nil {
			return err
		}
		if err := s.Repo.DeletePublisher(ctx, groupID); err != nil {
			return err
		}
		s.Tx.AfterCommit(ctx, func() { s.revokeSealed(pub) })
		return s.emit(ctx, groupID, domain.EventDrivePublisherDisconnected, &actorID, groupID,
			map[string]any{"email": pub.Email}, true)
	})
}

// revokeSealed revokes a stored token at Google; failures are only logged:
// the token is gone from our database either way.
func (s *Service) revokeSealed(pub *domain.DrivePublisher) {
	token, err := s.openToken(pub)
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err = s.Authorizer.Revoke(ctx, token)
	}
	if err != nil {
		s.Log.Warn("revoke publisher token", "group", pub.GroupID, "email", pub.Email, "err", err)
	}
}

func (s *Service) openToken(pub *domain.DrivePublisher) (string, error) {
	if s.Authorizer == nil || s.Secrets == nil {
		return "", errOAuthNotConfigured()
	}
	raw, err := s.Secrets.Open(pub.RefreshTokenEnc, pub.GroupID[:])
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// publisher returns the group's publishing account and a client acting as it,
// or nils when none is connected (or the server cannot use one).
func (s *Service) publisher(ctx context.Context, groupID uuid.UUID) (*domain.DrivePublisher, domain.DriveClient, error) {
	if !s.publisherAvailable() {
		return nil, nil, nil
	}
	pub, err := s.Repo.GetPublisher(ctx, groupID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	token, err := s.openToken(pub)
	if err != nil {
		return nil, nil, err
	}
	client, err := s.Authorizer.Client(ctx, token)
	if err != nil {
		return nil, nil, err
	}
	return pub, client, nil
}
