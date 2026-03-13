package chat

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"nhooyr.io/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 30 * time.Second
)

type Client struct {
	hub         *Hub
	conn        *websocket.Conn
	send        chan []byte
	userID      int64
	username    string
	role        string
	maxMsgLen   int
	rateLimitMS int
	lastMsgAt   time.Time
}

// incomingMsg is what the client sends over the wire.
type incomingMsg struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (c *Client) readPump(ctx context.Context) {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	c.conn.SetReadLimit(4096)

	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) != websocket.StatusNormalClosure {
				slog.Debug("chat: read error", "username", c.username, "err", err)
			}
			return
		}

		var incoming incomingMsg
		if err := json.Unmarshal(data, &incoming); err != nil {
			continue
		}

		if incoming.Type != "chat" || incoming.Text == "" {
			continue
		}

		// Length check
		if len(incoming.Text) > c.maxMsgLen {
			incoming.Text = incoming.Text[:c.maxMsgLen]
		}

		// Rate limit
		now := time.Now()
		if now.Sub(c.lastMsgAt) < time.Duration(c.rateLimitMS)*time.Millisecond {
			continue
		}
		c.lastMsgAt = now

		msg := &Message{
			Type:      "chat",
			UserID:    c.userID,
			Username:  c.username,
			Text:      incoming.Text,
			Timestamp: now.Unix(),
		}
		c.hub.broadcast <- msg
	}
}

func (c *Client) writePump(ctx context.Context) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	for {
		select {
		case data, ok := <-c.send:
			if !ok {
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, writeWait)
			err := c.conn.Write(writeCtx, websocket.MessageText, data)
			cancel()
			if err != nil {
				return
			}

		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, writeWait)
			err := c.conn.Ping(pingCtx)
			cancel()
			if err != nil {
				return
			}

		case <-ctx.Done():
			return
		}
	}
}
