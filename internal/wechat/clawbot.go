package wechat

import (
	"encoding/json"
	"net/http"
	"strings"

	"hermesclaw/internal/app"
)

type ClawbotHandler struct {
	App   app.Service
	Token string
}

type ClawbotEvent struct {
	From      string `json:"from"`
	UserID    string `json:"user_id"`
	RoomID    string `json:"room_id"`
	Channel   string `json:"channel"`
	Text      string `json:"text"`
	Content   string `json:"content"`
	Message   string `json:"message"`
	MessageID string `json:"message_id"`
}

func (h ClawbotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.Token != "" && r.Header.Get("X-Clawbot-Token") != h.Token && r.URL.Query().Get("token") != h.Token {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	defer r.Body.Close()
	var event ClawbotEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	text := firstNonEmpty(event.Text, event.Content, event.Message)
	if text == "" {
		writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "ignored"})
		return
	}
	userID := firstNonEmpty(event.UserID, event.From, event.RoomID, "wechat-user")
	channel := firstNonEmpty(event.Channel, "wechat")
	if event.RoomID != "" {
		channel = "wechat:room:" + event.RoomID
	}
	resp, err := h.App.HandleMessage(r.Context(), userID, channel, text)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code":    0,
		"message": "ok",
		"data": map[string]any{
			"reply": resp.Text,
			"url":   resp.URL,
		},
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
