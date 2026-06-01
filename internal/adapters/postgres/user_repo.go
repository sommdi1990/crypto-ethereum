package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domainuser "github.com/soroush/eth-exchange-lab/internal/domain/user"
)

// UserRepo implements user persistence.
type UserRepo struct{ db *DB }

func NewUserRepo(db *DB) *UserRepo { return &UserRepo{db: db} }

func (r *UserRepo) Create(ctx context.Context, email string) (*domainuser.User, error) {
	q := exec(ctx, r.db)
	row := q.QueryRow(ctx, `
		INSERT INTO users (email) VALUES ($1)
		RETURNING id, email, status, created_at, updated_at`, email)
	var u domainuser.User
	if err := row.Scan(&u.ID, &u.Email, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}
	return &u, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domainuser.User, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, email, status, created_at, updated_at FROM users WHERE id = $1`, id)
	var u domainuser.User
	if err := row.Scan(&u.ID, &u.Email, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}
	return &u, nil
}
