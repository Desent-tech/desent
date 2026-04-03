package chat

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync/atomic"
	"time"
)

// LiveChecker reports whether the stream is currently live.
type LiveChecker interface {
	IsLive() bool
}

// TitleProvider returns the current stream title.
type TitleProvider interface {
	GetStreamTitle(ctx context.Context) string
}

// BanChecker checks if a user is banned.
type BanChecker interface {
	IsBanned(ctx context.Context, userID int64) (bool, error)
}

// Message is the internal broadcast envelope.
type Message struct {
	Type      string `json:"type"`
	UserID    int64  `json:"user_id,omitempty"`
	Username  string `json:"username,omitempty"`
	Text      string `json:"text,omitempty"`
	Timestamp int64  `json:"timestamp"`
	MessageID int64  `json:"message_id,omitempty"`
}

type Hub struct {
	store         *Store
	clients       map[*Client]bool
	register      chan *Client
	unregister    chan *Client
	broadcast     chan *Message
	kick          chan int64
	deleteMsg     chan int64
	liveCheck     LiveChecker
	titleProvider TitleProvider
	sessionID     int64 // current active session (0 = no stream)
	wasLive       bool
	viewerCount   atomic.Int64
}

func NewHub(store *Store, lc LiveChecker, tp TitleProvider) *Hub {
	return &Hub{
		store:         store,
		clients:       make(map[*Client]bool),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		broadcast:     make(chan *Message, 256),
		kick:          make(chan int64, 16),
		deleteMsg:     make(chan int64, 64),
		liveCheck:     lc,
		titleProvider: tp,
	}
}

// DeleteMessage broadcasts a message_deleted event to all clients.
func (h *Hub) DeleteMessage(msgID int64) {
	h.deleteMsg <- msgID
}

// ViewerCount returns the current number of connected chat clients.
func (h *Hub) ViewerCount() int {
	return int(h.viewerCount.Load())
}

// Kick disconnects all clients with the given user ID.
func (h *Hub) Kick(userID int64) {
	h.kick <- userID
}

// Run is the main event loop. Must be called in a goroutine.
func (h *Hub) Run(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.closeAllClients()
			return

		case client := <-h.register:
			h.clients[client] = true
			h.viewerCount.Add(1)
			slog.Info("chat: client connected", "username", client.username, "clients", len(h.clients))
			h.broadcastSystem(client.username + " joined")

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				h.viewerCount.Add(-1)
				slog.Info("chat: client disconnected", "username", client.username, "clients", len(h.clients))
				h.broadcastSystem(client.username + " left")
			}

		case userID := <-h.kick:
			for client := range h.clients {
				if client.userID == userID {
					delete(h.clients, client)
					close(client.send)
					h.viewerCount.Add(-1)
					slog.Info("chat: kicked banned user", "username", client.username, "user_id", userID)
				}
			}

		case msg := <-h.broadcast:
			h.persistMessage(ctx, msg)
			data, err := json.Marshal(msg)
			if err != nil {
				slog.Error("chat: marshal message", "err", err)
				continue
			}
			for client := range h.clients {
				select {
				case client.send <- data:
				default:
					delete(h.clients, client)
					close(client.send)
				}
			}

		case msgID := <-h.deleteMsg:
			evt := &Message{
				Type:      "message_deleted",
				MessageID: msgID,
				Timestamp: time.Now().Unix(),
			}
			data, err := json.Marshal(evt)
			if err != nil {
				continue
			}
			for client := range h.clients {
				select {
				case client.send <- data:
				default:
					delete(h.clients, client)
					close(client.send)
				}
			}

		case <-ticker.C:
			h.checkLiveStatus(ctx)
		}
	}
}

func (h *Hub) checkLiveStatus(ctx context.Context) {
	live := h.liveCheck.IsLive()

	if live && !h.wasLive {
		// Stream just started — create a new session with current title
		title := h.titleProvider.GetStreamTitle(ctx)
		id, err := h.store.CreateSession(ctx, title)
		if err != nil {
			slog.Error("chat: create session", "err", err)
		} else {
			h.sessionID = id
			slog.Info("chat: stream session started", "session_id", id, "title", title)
			h.broadcastSystem("Stream started")
		}
	}

	if !live && h.wasLive {
		// Stream just ended — close session
		if h.sessionID > 0 {
			if err := h.store.CloseSession(ctx, h.sessionID); err != nil {
				slog.Error("chat: close session", "err", err)
			}
			slog.Info("chat: stream session ended", "session_id", h.sessionID)
			h.broadcastSystem("Stream ended")
			h.sessionID = 0
		}
	}

	h.wasLive = live
}

func (h *Hub) persistMessage(ctx context.Context, msg *Message) {
	if msg.Type != "chat" || h.sessionID == 0 {
		return
	}
	if err := h.store.SaveMessage(ctx, h.sessionID, msg.UserID, msg.Username, msg.Text); err != nil {
		slog.Error("chat: persist message", "err", err)
	}
}

func (h *Hub) broadcastSystem(text string) {
	msg := &Message{
		Type:      "system",
		Text:      text,
		Timestamp: time.Now().Unix(),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	for client := range h.clients {
		select {
		case client.send <- data:
		default:
			delete(h.clients, client)
			close(client.send)
		}
	}
}

func (h *Hub) closeAllClients() {
	for client := range h.clients {
		close(client.send)
		delete(h.clients, client)
	}
}
