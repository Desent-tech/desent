package auth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	store *Store
	ts    *TokenService
	cost  int
}

func NewHandler(store *Store, ts *TokenService, bcryptCost int) *Handler {
	return &Handler{store: store, ts: ts, cost: bcryptCost}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/register", h.register)
	mux.HandleFunc("POST /api/auth/login", h.login)
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
