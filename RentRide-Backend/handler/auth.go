package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"rentride/auth"
	"rentride/mw"
	"rentride/store"
)

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerReq struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", "อ่านข้อมูล JSON ไม่ออก")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	name := strings.TrimSpace(req.Name)

	if name == "" {
		writeError(w, http.StatusBadRequest, "BAD_NAME", "กรุณากรอกชื่อ")
		return
	}
	if !strings.Contains(email, "@") {
		writeError(w, http.StatusBadRequest, "BAD_EMAIL", "อีเมลไม่ถูกต้อง")
		return
	}
	if len(req.Password) < 6 {
		writeError(w, http.StatusBadRequest, "BAD_PASSWORD", "รหัสผ่านต้องยาวอย่างน้อย 6 ตัวอักษร")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		internal(w, err)
		return
	}

	u, err := h.Store.CreateUser(r.Context(), name, email, hash)
	if errors.Is(err, store.ErrDuplicate) {
		writeError(w, http.StatusConflict, "EMAIL_TAKEN", "อีเมลนี้ถูกใช้แล้ว")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}

	token, err := h.Tickets.Issue(u.ID, u.Role)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"token": token, "user": u})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", "อ่านข้อมูล JSON ไม่ออก")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))

	u, err := h.Store.UserByEmail(r.Context(), email)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "BAD_CREDENTIALS", "อีเมลหรือรหัสผ่านไม่ถูกต้อง")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	if !auth.CheckPassword(u.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "BAD_CREDENTIALS", "อีเมลหรือรหัสผ่านไม่ถูกต้อง")
		return
	}

	token, err := h.Tickets.Issue(u.ID, u.Role)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": u})
}

// Me คืนข้อมูลของคนที่ล็อกอินอยู่ ใช้ได้เฉพาะเส้นทางที่ห่อด้วย RequireLogin
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	id, ok := mw.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "NO_TOKEN", "กรุณาเข้าสู่ระบบก่อน")
		return
	}
	u, err := h.Store.UserByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "USER_GONE", "ไม่พบบัญชีนี้แล้ว กรุณาเข้าสู่ระบบใหม่")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}
