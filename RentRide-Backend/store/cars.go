package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// Car คือรถหนึ่งคันในรายการ ใช้ทั้งหน้ารายการและหน้ารายละเอียด
type Car struct {
	ID          int64   `json:"id"`
	Brand       string  `json:"brand"`
	Model       string  `json:"model"`
	Type        string  `json:"type"`
	Seats       *int    `json:"seats"`
	PricePerDay float64 `json:"price_per_day"`
	ImageURL    *string `json:"image_url"`
	Status      string  `json:"status"`
}

// ListCars คืนรถที่พร้อมให้เช่า ค้นจากยี่ห้อหรือรุ่น (q) และกรองประเภท (carType: "car" / "motorcycle")
// ถ้า q หรือ carType ว่างคือไม่กรองตามนั้น
func (s *Store) ListCars(ctx context.Context, q, carType string) ([]Car, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, brand, model, type, seats, price_per_day::float8, image_url, status
         FROM cars
         WHERE status = 'available'
           AND ($1 = '' OR brand ILIKE '%' || $1 || '%' OR model ILIKE '%' || $1 || '%')
           AND ($2 = '' OR type = $2)
         ORDER BY id`, q, carType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Car{}
	for rows.Next() {
		var c Car
		if err := rows.Scan(&c.ID, &c.Brand, &c.Model, &c.Type, &c.Seats, &c.PricePerDay, &c.ImageURL, &c.Status); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

// GetCar หารถจากเลข ไม่พบตอบ ErrNotFound
func (s *Store) GetCar(ctx context.Context, id int64) (Car, error) {
	var c Car
	err := s.Pool.QueryRow(ctx,
		`SELECT id, brand, model, type, seats, price_per_day::float8, image_url, status
         FROM cars WHERE id = $1`, id).
		Scan(&c.ID, &c.Brand, &c.Model, &c.Type, &c.Seats, &c.PricePerDay, &c.ImageURL, &c.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Car{}, ErrNotFound
	}
	return c, err
}
