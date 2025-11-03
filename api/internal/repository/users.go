package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/octobees/leads-generator/api/internal/entity"
)

// ErrUserNotFound is returned when no user matches the lookup criteria.
var (
	ErrUserNotFound   = errors.New("user not found")
	ErrEmailDuplicate = errors.New("email already exists")
)

// UsersRepository declares readonly operations for users.
type UsersRepository interface {
	FindByEmail(ctx context.Context, email string) (*entity.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error)
	Create(ctx context.Context, email, passwordHash, role string) (*entity.User, error)
	List(ctx context.Context) ([]entity.User, error)
	Update(ctx context.Context, id uuid.UUID, email, passwordHash, role *string) (*entity.User, error)
	Delete(ctx context.Context, id uuid.UUID) error
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
	row := r.pool.QueryRow(ctx, `SELECT id, email, password_hash, role, created_at, updated_at FROM users WHERE email = $1`, email)

	var user entity.User
	if err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("query user by email: %w", err)
	}

	return &user, nil
}

// FindByID retrieves a user by identifier.
func (r *PGXUsersRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	row := r.pool.QueryRow(ctx, `SELECT id, email, password_hash, role, created_at, updated_at FROM users WHERE id = $1`, id)

	var user entity.User
	if err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("query user by id: %w", err)
	}

	return &user, nil
}

// Create inserts a new user row.
func (r *PGXUsersRepository) Create(ctx context.Context, email, passwordHash, role string) (*entity.User, error) {
	row := r.pool.QueryRow(ctx, `
        INSERT INTO users (email, password_hash, role)
        VALUES ($1, $2, $3)
        RETURNING id, email, password_hash, role, created_at, updated_at
    `, email, passwordHash, role)

	var user entity.User
	if err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.Message, "users_email_key") {
			return nil, fmt.Errorf("%w: %v", ErrEmailDuplicate, pgErr)
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}

	return &user, nil
}

// List returns all users ordered by creation date (desc).
func (r *PGXUsersRepository) List(ctx context.Context) ([]entity.User, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, email, password_hash, role, created_at, updated_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []entity.User
	for rows.Next() {
		var user entity.User
		if err := rows.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user row: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return users, nil
}

// Update patches user attributes.
func (r *PGXUsersRepository) Update(ctx context.Context, id uuid.UUID, email, passwordHash, role *string) (*entity.User, error) {
	setClauses := make([]string, 0)
	args := make([]any, 0)
	idx := 1

	if email != nil {
		setClauses = append(setClauses, fmt.Sprintf("email = $%d", idx))
		args = append(args, *email)
		idx++
	}
	if passwordHash != nil {
		setClauses = append(setClauses, fmt.Sprintf("password_hash = $%d", idx))
		args = append(args, *passwordHash)
		idx++
	}
	if role != nil {
		setClauses = append(setClauses, fmt.Sprintf("role = $%d", idx))
		args = append(args, *role)
		idx++
	}

	if len(setClauses) == 0 {
		return r.FindByID(ctx, id)
	}

	setClauses = append(setClauses, "updated_at = NOW()")
	args = append(args, id)

	query := fmt.Sprintf(`UPDATE users SET %s WHERE id = $%d RETURNING id, email, password_hash, role, created_at, updated_at`, strings.Join(setClauses, ", "), idx)

	row := r.pool.QueryRow(ctx, query, args...)

	var user entity.User
	if err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.Message, "users_email_key") {
			return nil, fmt.Errorf("%w: %v", ErrEmailDuplicate, pgErr)
		}
		return nil, fmt.Errorf("update user: %w", err)
	}

	return &user, nil
}

// Delete removes a user by id.
func (r *PGXUsersRepository) Delete(ctx context.Context, id uuid.UUID) error {
	cmd, err := r.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}
