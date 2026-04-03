package chat

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"nhooyr.io/websocket"

	"desent/internal/auth"
)

type Handler struct {
	hub          *Hub
	store        *Store
	tokenService *auth.TokenService
	banChecker   BanChecker
	maxMsgLen    int
	rateLimitMS  int
}

func NewHandler(hub *Hub, store *Store, ts *auth.TokenService, bc BanChecker, maxMsgLen, rateLimitMS int) *Handler {
	return &Handler{
		hub:          hub,
		store:        store,
		tokenService: ts,
		banChecker:   bc,
		maxMsgLen:    maxMsgLen,
		rateLimitMS:  rateLimitMS,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /ws/chat", h.handleWS)
	mux.HandleFunc("GET /api/chat/history/{sessionId}", h.handleHistory)
	mux.HandleFunc("GET /api/chat/sessions", h.handleSessions)
}

func (h *Handler) RegisterModRoutes(mux *http.ServeMux, mw func(http.Handler) http.Handler) {
	mux.Handle("DELETE /api/chat/messages/{id}", mw(http.HandlerFunc(h.deleteMessage)))
	mux.Handle("POST /api/chat/timeout", mw(http.HandlerFunc(h.timeoutUser)))
}

func (h *Handler) handleWS(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)
		return
	}

	claims, err := h.tokenService.Validate(token)
	if err != nil {
		http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
		return
	}

	banned, err := h.banChecker.IsBanned(r.Context(), claims.UserID)
	if err != nil {
		slog.Error("chat: check ban", "err", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if banned {
		http.Error(w, `{"error":"you are banned"}`, http.StatusForbidden)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // allow any origin for now
	})
	if err != nil {
		slog.Error("chat: websocket accept", "err", err)
		return
	}

	client := &Client{
		hub:            h.hub,
		conn:           conn,
		send:           make(chan []byte, 256),
		userID:         claims.UserID,
		username:       claims.Username,
		role:           claims.Role,
		maxMsgLen:      h.maxMsgLen,
		rateLimitMS:    h.rateLimitMS,
		timeoutChecker: h.store,
	}

	h.hub.register <- client

	ctx := r.Context()
	go client.writePump(ctx)
	client.readPump(ctx)
}

func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	sessionID, err := strconv.ParseInt(r.PathValue("sessionId"), 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid session id"}`, http.StatusBadRequest)
		return
	}

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	var beforeID int64
	if v := r.URL.Query().Get("before"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			beforeID = n
		}
	}

	msgs, err := h.store.GetMessages(r.Context(), sessionID, limit, beforeID)
	if err != nil {
		slog.Error("chat: get history", "err", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	if msgs == nil {
		msgs = []ChatMessage{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"session_id": sessionID,
		"messages":   msgs,
	})
}

func (h *Handler) handleSessions(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	sessions, err := h.store.GetSessions(r.Context(), limit)
	if err != nil {
		slog.Error("chat: get sessions", "err", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	if sessions == nil {
		sessions = []StreamSession{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"sessions": sessions,
	})
}

func (h *Handler) deleteMessage(w http.ResponseWriter, r *http.Request) {
	msgID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid message ID"})
		return
	}

	if err := h.store.DeleteMessage(r.Context(), msgID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "message not found"})
			return
		}
		slog.Error("chat: delete message", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	h.hub.DeleteMessage(msgID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

type timeoutRequest struct {
	UserID   int64  `json:"user_id"`
	Duration int    `json:"duration_minutes"`
	Reason   string `json:"reason"`
}

func (h *Handler) timeoutUser(w http.ResponseWriter, r *http.Request) {
	var req timeoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.UserID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id is required"})
		return
	}
	if req.Duration <= 0 || req.Duration > 1440 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "duration_minutes must be 1-1440"})
		return
	}

	claims := auth.ClaimsFromContext(r.Context())
	if claims.UserID == req.UserID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot timeout yourself"})
		return
	}

	if err := h.store.TimeoutUser(r.Context(), req.UserID, req.Duration, claims.UserID, req.Reason); err != nil {
		slog.Error("chat: timeout user", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "timed out"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
