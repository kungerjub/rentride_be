package store

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrCarUnavailable = errors.New("car unavailable")
	ErrOverlap        = errors.New("booking overlap")
)

type Booking struct {
	ID         int64     `json:"id"`
	CarID      int64     `json:"car_id"`
	CarName    string    `json:"car_name"`
	StartDate  string    `json:"start_date"`
	EndDate    string    `json:"end_date"`
	Days       int       `json:"days"`
	TotalPrice float64   `json:"total_price"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

type AdminBooking struct {
	Booking
	UserName  string `json:"user_name"`
	UserEmail string `json:"user_email"`
}

// CreateBooking ล็อกแถวรถก่อนเช็กวันซ้อน เพื่อกันสองคนจองพร้อมกันแล้วชนกัน
func (s *Store) CreateBooking(ctx context.Context, userID, carID int64, start, end time.Time) (Booking, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback(ctx)

	var brand, model, status string
	var price float64
	err = tx.QueryRow(ctx,
		`SELECT brand, model, status, price_per_day::float8
		 FROM cars WHERE id = $1 FOR UPDATE`, carID).
		Scan(&brand, &model, &status, &price)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, ErrNotFound
	}
	if err != nil {
		return Booking{}, err
	}
	if status != "available" {
		return Booking{}, ErrCarUnavailable
	}

	var taken bool
	err = tx.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM bookings
		   WHERE car_id = $1 AND status = 'confirmed'
		     AND start_date <= $3 AND end_date >= $2)`,
		carID, start, end).Scan(&taken)
	if err != nil {
		return Booking{}, err
	}
	if taken {
		return Booking{}, ErrOverlap
	}

	days := int(end.Sub(start).Hours()/24) + 1
	total := math.Round(price*float64(days)*100) / 100

	b := Booking{
		CarID: carID, CarName: brand + " " + model,
		StartDate: start.Format("2006-01-02"), EndDate: end.Format("2006-01-02"),
		Days: days, TotalPrice: total, Status: "confirmed",
	}
	err = tx.QueryRow(ctx,
		`INSERT INTO bookings (user_id, car_id, start_date, end_date, total_price)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at`,
		userID, carID, start, end, total).Scan(&b.ID, &b.CreatedAt)
	if err != nil {
		return Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Booking{}, err
	}
	return b, nil
}

// ListBookingsByUser คืนการจองทั้งหมดของผู้ใช้คนนั้น ใหม่สุดก่อน
func (s *Store) ListBookingsByUser(ctx context.Context, userID int64) ([]Booking, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT b.id, b.car_id, c.brand || ' ' || c.model,
		        to_char(b.start_date, 'YYYY-MM-DD'), to_char(b.end_date, 'YYYY-MM-DD'),
		        (b.end_date - b.start_date + 1), b.total_price::float8, b.status, b.created_at
		 FROM bookings b JOIN cars c ON c.id = b.car_id
		 WHERE b.user_id = $1
		 ORDER BY b.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Booking{}
	for rows.Next() {
		var b Booking
		if err := rows.Scan(&b.ID, &b.CarID, &b.CarName, &b.StartDate, &b.EndDate,
			&b.Days, &b.TotalPrice, &b.Status, &b.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, b)
	}
	return items, rows.Err()
}

// ListAllBookings สำหรับ admin: ทุกการจอง พร้อมชื่อและอีเมลผู้จอง
func (s *Store) ListAllBookings(ctx context.Context) ([]AdminBooking, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT b.id, b.car_id, c.brand || ' ' || c.model,
		        to_char(b.start_date, 'YYYY-MM-DD'), to_char(b.end_date, 'YYYY-MM-DD'),
		        (b.end_date - b.start_date + 1), b.total_price::float8, b.status, b.created_at,
		        u.name, u.email
		 FROM bookings b
		 JOIN cars c ON c.id = b.car_id
		 JOIN users u ON u.id = b.user_id
		 ORDER BY b.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AdminBooking{}
	for rows.Next() {
		var b AdminBooking
		if err := rows.Scan(&b.ID, &b.CarID, &b.CarName, &b.StartDate, &b.EndDate,
			&b.Days, &b.TotalPrice, &b.Status, &b.CreatedAt, &b.UserName, &b.UserEmail); err != nil {
			return nil, err
		}
		items = append(items, b)
	}
	return items, rows.Err()
}

// CancelBooking ยกเลิกได้เฉพาะการจองของตัวเองที่ยัง confirmed ไม่พบตอบ ErrNotFound
func (s *Store) CancelBooking(ctx context.Context, id, userID int64) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE bookings SET status = 'cancelled'
		 WHERE id = $1 AND user_id = $2 AND status = 'confirmed'`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}