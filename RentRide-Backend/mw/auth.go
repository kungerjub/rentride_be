package mw

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"rentride/auth"
)

type ctxKey int

const (
	userIDKey ctxKey = iota
	roleKey
)

// UserID คืนเลขผู้ใช้จากตั๋วที่ตรวจแล้ว ok = false แปลว่าคำขอนี้ไม่ได้ผ่าน RequireLogin
func UserID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDKey).(int64)
	return id, ok
}

// Role คืนบทบาทจากตั๋ว ("member" หรือ "admin") ว่างถ้ายังไม่ได้ login
func Role(ctx context.Context) string {
	role, _ := ctx.Value(roleKey).(string)
	return role
}

func fail(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

// RequireLogin คือพนักงานตรวจตั๋ว: ต้องมี Authorization: Bearer <token> ที่ตรวจผ่าน
func RequireLogin(iss *auth.Issuer, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			fail(w, http.StatusUnauthorized, "NO_TOKEN", "กรุณาเข้าสู่ระบบก่อน")
			return
		}
		id, role, err := iss.Parse(strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			fail(w, http.StatusUnauthorized, "BAD_TOKEN", "ตั๋วไม่ถูกต้องหรือหมดอายุ กรุณาเข้าสู่ระบบใหม่")
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, id)
		ctx = context.WithValue(ctx, roleKey, role)
		next(w, r.WithContext(ctx))
	}
}

// RequireRole ใช้ต่อจาก RequireLogin: ตั๋วถูกต้องแต่บทบาทไม่ตรง ตอบ 403
func RequireRole(role string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if Role(r.Context()) != role {
			fail(w, http.StatusForbidden, "FORBIDDEN", "ต้องเป็น "+role+" เท่านั้น")
			return
		}
		next(w, r)
	}
}
