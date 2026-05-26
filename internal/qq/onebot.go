package qq

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"hermesclaw/internal/app"
)

type Handler struct {
	App         app.Service
	HTTPAPI     string
	AccessToken string
	AllowedIDs  map[string]bool
}

type Event struct {
	PostType    string `json:"post_type"`
	MessageType string `json:"message_type"`
	UserID      int64  `json:"user_id"`
	GroupID     int64  `json:"group_id"`
	Message     any    `json:"message"`
	RawMessage  string `json:"raw_message"`
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var event Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	text := strings.TrimSpace(event.RawMessage)
	if text == "" {
		text = messageText(event.Message)
	}
	if text == "" || event.PostType != "message" {
		writeJSON(w, map[string]string{"status": "ignored"})
		return
	}
	targetID := strconv.FormatInt(event.UserID, 10)
	if event.GroupID != 0 {
		targetID = strconv.FormatInt(event.GroupID, 10)
	}
	if !h.allowed(targetID) && !h.allowed(strconv.FormatInt(event.UserID, 10)) {
		writeJSON(w, map[string]string{"status": "forbidden"})
		return
	}
	resp, err := h.App.HandleMessage(r.Context(), strconv.FormatInt(event.UserID, 10), "qq:"+event.MessageType, text)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = h.send(event, resp.Text)
	writeJSON(w, map[string]any{"status": "ok", "reply": resp.Text})
}

func (h Handler) allowed(id string) bool {
	if len(h.AllowedIDs) == 0 {
		return true
	}
	return h.AllowedIDs[id]
}

func (h Handler) send(event Event, text string) error {
	if h.HTTPAPI == "" {
		return nil
	}
	action := "send_private_msg"
	payload := map[string]any{"user_id": event.UserID, "message": text}
	if event.GroupID != 0 {
		action = "send_group_msg"
		payload = map[string]any{"group_id": event.GroupID, "message": text}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(h.HTTPAPI, "/")+"/"+action, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if h.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+h.AccessToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func messageText(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []any:
		var b strings.Builder
		for _, item := range v {
			part, ok := item.(map[string]any)
			if !ok || part["type"] != "text" {
				continue
			}
			data, ok := part["data"].(map[string]any)
			if !ok {
				continue
			}
			if text, ok := data["text"].(string); ok {
				b.WriteString(text)
			}
		}
		return b.String()
	default:
		return ""
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(value)
}
