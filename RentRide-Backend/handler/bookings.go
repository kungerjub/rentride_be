package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"rentride/mw"
	"rentride/store"
)

// ใช้เวลาไทยตัดสินว่า "วันนี้" คือวันไหน
var ict = time.FixedZone("ICT", 7*3600)

type createBookingReq struct {
	CarID     int64  `json:"car_id"`
	StartDate string `json:"start_date"` // YYYY-MM-DD
	EndDate   string `json:"end_date"`   // YYYY-MM-DD
}

func (h *Handler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "NO_TOKEN", "กรุณาเข้าสู่ระบบก่อน")
		return
	}
	var req createBookingReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", "อ่านข้อมูล JSON ไม่ออก")
		return
	}
	if req.CarID <= 0 {
		writeError(w, http.StatusBadRequest, "BAD_CAR", "กรุณาเลือกรถ")
		return
	}
	start, err1 := time.Parse("2006-01-02", req.StartDate)
	end, err2 := time.Parse("2006-01-02", req.EndDate)
	if err1 != nil || err2 != nil {
		writeError(w, http.StatusBadRequest, "BAD_DATE", "รูปแบบวันที่ต้องเป็น YYYY-MM-DD")
		return
	}
	n := time.Now().In(ict)
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	if start.Before(today) {
		writeError(w, http.StatusBadRequest, "PAST_DATE", "วันเริ่มเช่าต้องไม่ใช่วันที่ผ่านมาแล้ว")
		return
	}
	if end.Before(start) {
		writeError(w, http.StatusBadRequest, "BAD_RANGE", "วันคืนรถต้องไม่ก่อนวันเริ่มเช่า")
		return
	}

	b, err := h.Store.CreateBooking(r.Context(), userID, req.CarID, start, end)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "CAR_NOT_FOUND", "ไม่พบรถคันนี้")
	case errors.Is(err, store.ErrCarUnavailable):
		writeError(w, http.StatusConflict, "CAR_UNAVAILABLE", "รถคันนี้ไม่พร้อมให้เช่า")
	case errors.Is(err, store.ErrOverlap):
		writeError(w, http.StatusConflict, "DATES_TAKEN", "รถไม่ว่างในช่วงวันที่เลือก")
	case err != nil:
		internal(w, err)
	default:
		writeJSON(w, http.StatusCreated, b)
	}
}

func (h *Handler) MyBookings(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "NO_TOKEN", "กรุณาเข้าสู่ระบบก่อน")
		return
	}
	items, err := h.Store.ListBookingsByUser(r.Context(), userID)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) AllBookings(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.ListAllBookings(r.Context())
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) CancelBooking(w http.ResponseWriter, r *http.Request) {
	userID, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "NO_TOKEN", "กรุณาเข้าสู่ระบบก่อน")
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "เลขการจองต้องเป็นตัวเลข")
		return
	}
	err := h.Store.CancelBooking(r.Context(), id, userID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "BOOKING_NOT_FOUND", "ไม่พบการจองที่ยกเลิกได้")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}