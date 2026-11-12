package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrDuplicate = errors.New("duplicate")

type User struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	Email        string     `json:"email"`
	Role         string     `json:"role"`
	CreatedAt    *time.Time `json:"createdAt,omitempty"`
	PasswordHash string     `json:"-"` // ไม่มีวันออกไปกับ JSON
}

// UserByEmail ใช้ตอน login: email ต้องเป็นตัวพิมพ์เล็กมาแล้ว ไม่พบตอบ ErrNotFound
func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := s.Pool.QueryRow(ctx,
		`SELECT id, name, email, role, password_hash
         FROM users WHERE lower(email) = $1`, email).
		Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

// CreateUser สมัครสมาชิกใหม่ บทบาทเป็น member เสมอ (ค่าเริ่มต้นของตาราง) อีเมลซ้ำตอบ ErrDuplicate
func (s *Store) CreateUser(ctx context.Context, name, email, passwordHash string) (User, error) {
	var u User
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO users (name, email, password_hash)
         VALUES ($1, $2, $3)
         RETURNING id, name, email, role, password_hash`,
		name, email, passwordHash).
		Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.PasswordHash)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return User{}, ErrDuplicate
		}
		return User{}, err
	}
	return u, nil
}

// UserByID ใช้กับ GET /api/me: เลขมาจากตั๋วที่ตรวจแล้ว
func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	var u User
	err := s.Pool.QueryRow(ctx,
		`SELECT id, name, email, role, created_at
         FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}
