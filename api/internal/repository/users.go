package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/octobees/leads-generator/api/internal/entity"
)

// ErrUserNotFound is returned when no user matches the lookup criteria.
var ErrUserNotFound = errors.New("user not found")

// UsersRepository declares readonly operations for users.
type UsersRepository interface {
	FindByEmail(ctx context.Context, email string) (*entity.User, error)
}

// PGXUsersRepository implements UsersRepository with pgx.
type PGXUsersRepository struct {
	pool *pgxpool.Pool
}

// NewPGXUsersRepository instantiates a users repository.
func NewPGXUsersRepository(pool *pgxpool.Pool) *PGXUsersRepository {
	return &PGXUsersRepository{pool: pool}
}

// FindByEmail fetches a user by email if present.
func (r *PGXUsersRepository) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	row := r.pool.QueryRow(ctx, `SELECT id, email, password_hash, role, created_at FROM users WHERE email = $1`, email)

	var user entity.User
	if err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Role, &user.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("query user by email: %w", err)
	}

	return &user, nil
}
