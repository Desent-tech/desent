package chat

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"nhooyr.io/websocket"

	"desent/internal/auth"
)

type Handler struct {
	hub          *Hub
	store        *Store
	tokenService *auth.TokenService
	maxMsgLen    int
	rateLimitMS  int
}

func NewHandler(hub *Hub, store *Store, ts *auth.TokenService, maxMsgLen, rateLimitMS int) *Handler {
	return &Handler{
		hub:          hub,
		store:        store,
		tokenService: ts,
		maxMsgLen:    maxMsgLen,
		rateLimitMS:  rateLimitMS,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /ws/chat", h.handleWS)
	mux.HandleFunc("GET /api/chat/history/{sessionId}", h.handleHistory)
	mux.HandleFunc("GET /api/chat/sessions", h.handleSessions)
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

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // allow any origin for now
	})
	if err != nil {
		slog.Error("chat: websocket accept", "err", err)
		return
	}

	client := &Client{
		hub:         h.hub,
		conn:        conn,
		send:        make(chan []byte, 256),
		userID:      claims.UserID,
		username:    claims.Username,
		role:        claims.Role,
		maxMsgLen:   h.maxMsgLen,
		rateLimitMS: h.rateLimitMS,
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
