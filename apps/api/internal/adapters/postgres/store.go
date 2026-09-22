// Package postgres implements the domain repositories on top of sqlc-generated
// queries. Transactions travel through context so application services can
// compose repository calls atomically without knowing about pgx.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"heatseeker/api/internal/adapters/postgres/sqlcgen"
	"heatseeker/api/internal/domain"
)

// Store owns the pool and hands out repositories.
type Store struct {
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
}

// NewStore wraps a pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, q: sqlcgen.New(pool)}
}

// Pool exposes the underlying pool for health checks.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

type txState struct {
	tx    pgx.Tx
	after []func()
}

type txKey struct{}

func stateFrom(ctx context.Context) *txState {
	st, _ := ctx.Value(txKey{}).(*txState)
	return st
}

// RunInTx implements domain.TxManager. Nested calls join the outer transaction.
func (s *Store) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if stateFrom(ctx) != nil {
		return fn(ctx)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	st := &txState{tx: tx}
	txCtx := context.WithValue(ctx, txKey{}, st)
	if err := fn(txCtx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	for _, f := range st.after {
		f()
	}
	return nil
}

// AfterCommit implements domain.TxManager.
func (s *Store) AfterCommit(ctx context.Context, fn func()) {
	if st := stateFrom(ctx); st != nil {
		st.after = append(st.after, fn)
		return
	}
	fn()
}

// queries returns the query set bound to the current transaction, if any.
func (s *Store) queries(ctx context.Context) *sqlcgen.Queries {
	if st := stateFrom(ctx); st != nil {
		return sqlcgen.New(st.tx)
	}
	return s.q
}

// Repositories.

// Users returns the user repository.
func (s *Store) Users() domain.UserRepo { return &userRepo{s} }

// Auth returns the auth repository.
func (s *Store) Auth() domain.AuthRepo { return &authRepo{s} }

// Groups returns the group repository.
func (s *Store) Groups() domain.GroupRepo { return &groupRepo{s} }

// Memberships returns the membership repository.
func (s *Store) Memberships() domain.MembershipRepo { return &membershipRepo{s} }

// Invites returns the invite repository.
func (s *Store) Invites() domain.InviteRepo { return &inviteRepo{s} }

// Subjects returns the subject repository.
func (s *Store) Subjects() domain.SubjectRepo { return &subjectRepo{s} }

// Tags returns the tag repository.
func (s *Store) Tags() domain.TagRepo { return &tagRepo{s} }

// Events returns the event repository.
func (s *Store) Events() domain.EventRepo { return &eventRepo{s} }

// Materials returns the material repository.
func (s *Store) Materials() domain.MaterialRepo { return &materialRepo{s} }

// Uploads returns the upload repository.
func (s *Store) Uploads() domain.UploadRepo { return &uploadRepo{s} }

// Drive returns the Drive connection/index repository.
func (s *Store) Drive() domain.DriveRepo { return &driveRepo{s} }

// Tasks returns the task repository.
func (s *Store) Tasks() domain.TaskRepo { return &taskRepo{s} }

// Discussions returns the threads and messages repository.
func (s *Store) Discussions() domain.DiscussionRepo { return &discussionRepo{s} }

// Schedule returns the schedule repository.
func (s *Store) Schedule() domain.ScheduleRepo { return &scheduleRepo{s} }

// mapErr converts driver errors into domain errors.
func mapErr(err error, entity string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NotFound(entity)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return domain.Conflict(entity + " already exists")
		case "23503":
			return fmt.Errorf("%w: %s references a missing row", domain.ErrInvalid, entity)
		case "23514":
			return fmt.Errorf("%w: %s violates a constraint", domain.ErrInvalid, entity)
		}
	}
	return fmt.Errorf("%s: %w", entity, err)
}

func rawJSON(b []byte) []byte {
	if len(b) == 0 {
		return []byte("{}")
	}
	return b
}
