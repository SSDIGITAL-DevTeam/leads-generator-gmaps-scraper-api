package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type stubPool struct {
	queryRowFunc func(ctx context.Context, query string, args ...any) pgx.Row
	queryFunc    func(ctx context.Context, query string, args ...any) (pgx.Rows, error)
	execFunc     func(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
	beginTxFunc  func(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}


func (s *stubPool) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	if s.queryRowFunc != nil {
		return s.queryRowFunc(ctx, query, args...)
	}
	return &stubRow{scan: func(dest ...any) error { return nil }}
}

func (s *stubPool) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	if s.queryFunc != nil {
		return s.queryFunc(ctx, query, args...)
	}
	return nil, errors.New("query not implemented")
}

func (s *stubPool) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	if s.execFunc != nil {
		return s.execFunc(ctx, query, args...)
	}
	return pgconn.CommandTag{}, errors.New("exec not implemented")
}

func (s *stubPool) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
	if s.beginTxFunc != nil {
		return s.beginTxFunc(ctx, txOptions)
	}
	return nil, errors.New("begin tx not implemented")
}

type stubRow struct {
	scan func(dest ...any) error
}

func (s *stubRow) Scan(dest ...any) error {
	if s.scan != nil {
		return s.scan(dest...)
	}
	return nil
}

type stubRows struct {
	scans []func(dest ...any) error
	idx   int
	err   error
}

func (s *stubRows) Close() {}

func (s *stubRows) Err() error { return s.err }

func (s *stubRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (s *stubRows) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (s *stubRows) Next() bool {
	if s.err != nil {
		return false
	}
	if s.idx < len(s.scans) {
		s.idx++
		return true
	}
	return false
}

func (s *stubRows) Scan(dest ...any) error {
	if s.idx == 0 || s.idx > len(s.scans) {
		return errors.New("scan called out of order")
	}
	return s.scans[s.idx-1](dest...)
}

func (s *stubRows) Values() ([]any, error) { return nil, nil }

func (s *stubRows) RawValues() [][]byte { return nil }

func (s *stubRows) Conn() *pgx.Conn { return nil }

func TestPGXUsersRepository_FindByEmail(t *testing.T) {
	repo := &PGXUsersRepository{pool: &stubPool{
		queryRowFunc: func(ctx context.Context, query string, args ...any) pgx.Row {
			return &stubRow{scan: func(dest ...any) error {
				id := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
				created := time.Now()
				updated := created.Add(time.Minute)
				*dest[0].(*uuid.UUID) = id
				*dest[1].(*string) = "user@example.com"
				*dest[2].(*string) = "hashed"
				*dest[3].(*string) = "admin"
				*dest[4].(*time.Time) = created
				*dest[5].(*time.Time) = updated
				return nil
			}}
		},
	}}

	user, err := repo.FindByEmail(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Email != "user@example.com" || user.Role != "admin" {
		t.Fatalf("unexpected user: %+v", user)
	}

	repo.pool = &stubPool{
		queryRowFunc: func(ctx context.Context, query string, args ...any) pgx.Row {
			return &stubRow{scan: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}
	if _, err := repo.FindByEmail(context.Background(), "missing@example.com"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestPGXUsersRepository_Create(t *testing.T) {
	repo := &PGXUsersRepository{pool: &stubPool{
		queryRowFunc: func(ctx context.Context, query string, args ...any) pgx.Row {
			return &stubRow{scan: func(dest ...any) error {
				id := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
				created := time.Now()
				updated := created
				*dest[0].(*uuid.UUID) = id
				*dest[1].(*string) = "user@example.com"
				*dest[2].(*string) = "hashed"
				*dest[3].(*string) = "user"
				*dest[4].(*time.Time) = created
				*dest[5].(*time.Time) = updated
				return nil
			}}
		},
	}}

	user, err := repo.Create(context.Background(), "user@example.com", "hashed", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Email != "user@example.com" {
		t.Fatalf("expected created user, got %+v", user)
	}

}

func TestPGXUsersRepository_List(t *testing.T) {
	repo := &PGXUsersRepository{pool: &stubPool{
		queryFunc: func(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
			return &stubRows{
				scans: []func(dest ...any) error{
					func(dest ...any) error {
						id := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
						created := time.Now()
						updated := created
						*dest[0].(*uuid.UUID) = id
						*dest[1].(*string) = "admin@example.com"
						*dest[2].(*string) = "hash"
						*dest[3].(*string) = "admin"
						*dest[4].(*time.Time) = created
						*dest[5].(*time.Time) = updated
						return nil
					},
				},
			}, nil
		},
	}}

	rows, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 || rows[0].Email != "admin@example.com" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}

func TestPGXUsersRepository_Update(t *testing.T) {
	repo := &PGXUsersRepository{pool: &stubPool{
		queryRowFunc: func(ctx context.Context, query string, args ...any) pgx.Row {
			return &stubRow{scan: func(dest ...any) error {
				id := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
				created := time.Now()
				updated := created.Add(time.Minute)
				*dest[0].(*uuid.UUID) = id
				*dest[1].(*string) = "updated@example.com"
				*dest[2].(*string) = "hash"
				*dest[3].(*string) = "manager"
				*dest[4].(*time.Time) = created
				*dest[5].(*time.Time) = updated
				return nil
			}}
		},
	}}

	email := "updated@example.com"
	role := "manager"
	user, err := repo.Update(context.Background(), uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), &email, nil, &role)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Email != "updated@example.com" {
		t.Fatalf("unexpected user: %+v", user)
	}

	repo.pool = &stubPool{
		queryRowFunc: func(ctx context.Context, query string, args ...any) pgx.Row {
			return &stubRow{scan: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}
	if _, err := repo.Update(context.Background(), uuid.New(), &email, nil, &role); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestPGXUsersRepository_Delete(t *testing.T) {
	repo := &PGXUsersRepository{pool: &stubPool{
		execFunc: func(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}}

	if err := repo.Delete(context.Background(), uuid.New()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	repo.pool = &stubPool{
		execFunc: func(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 0"), nil
		},
	}
	if err := repo.Delete(context.Background(), uuid.New()); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}
