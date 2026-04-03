package auth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	store       *Store
	ts          *TokenService
	cost        int
	rateLimiter *RateLimiter
}

func NewHandler(store *Store, ts *TokenService, bcryptCost int, rl *RateLimiter) *Handler {
	return &Handler{store: store, ts: ts, cost: bcryptCost, rateLimiter: rl}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/register", h.register)
	mux.HandleFunc("POST /api/auth/login", h.login)
}

// RegisterProtectedRoutes registers routes that require auth middleware.
func (h *Handler) RegisterProtectedRoutes(mux *http.ServeMux, mw func(http.Handler) http.Handler) {
	mux.Handle("PUT /api/auth/password", mw(http.HandlerFunc(h.changePassword)))
	mux.Handle("POST /api/auth/refresh", mw(http.HandlerFunc(h.refresh)))
}

type authRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string `json:"token"`
	Role  string `json:"role"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	if h.rateLimiter != nil && !h.rateLimiter.Allow(ClientIP(r)) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many attempts, please try again later"})
		return
	}

	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if len(req.Username) < 3 || len(req.Username) > 32 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username must be 3-32 characters"})
		return
	}
	if len(req.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password must be at least 8 characters"})
		return
	}

	user, err := h.store.CreateUser(r.Context(), req.Username, req.Password, h.cost)
	if errors.Is(err, ErrUsernameTaken) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "username taken"})
		return
	}
	if err != nil {
		slog.Error("register: create user failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	token, err := h.ts.Generate(user)
	if err != nil {
		slog.Error("register: generate token failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusCreated, authResponse{Token: token, Role: user.Role})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	if h.rateLimiter != nil && !h.rateLimiter.Allow(ClientIP(r)) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many attempts, please try again later"})
		return
	}

	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	user, err := h.store.GetByUsername(r.Context(), req.Username)
	if errors.Is(err, ErrUserNotFound) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	if err != nil {
		slog.Error("login: get user failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	token, err := h.ts.Generate(user)
	if err != nil {
		slog.Error("login: generate token failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, authResponse{Token: token, Role: user.Role})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if len(req.NewPassword) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "new password must be at least 8 characters"})
		return
	}

	user, err := h.store.GetByID(r.Context(), claims.UserID)
	if err != nil {
		slog.Error("change password: get user failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "current password is incorrect"})
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), h.cost)
	if err != nil {
		slog.Error("change password: bcrypt failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if err := h.store.UpdatePassword(r.Context(), claims.UserID, string(newHash)); err != nil {
		slog.Error("change password: update failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "password changed"})
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	user, err := h.store.GetByID(r.Context(), claims.UserID)
	if err != nil {
		slog.Error("refresh: get user failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	token, err := h.ts.Generate(user)
	if err != nil {
		slog.Error("refresh: generate token failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, authResponse{Token: token, Role: user.Role})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
