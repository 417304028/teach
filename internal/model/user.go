package model

import "time"

type UserRole string

const (
	RoleAdmin   UserRole = "admin"
	RoleTeacher UserRole = "teacher"
	RoleStudent UserRole = "student"
)

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Role         UserRole  `json:"role"`
	PasswordHash string    `json:"-"`
	DisplayName  string    `json:"display_name"`
	CreatedAt    time.Time `json:"created_at"`
}

type QQAccount struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	QQID        string    `json:"qq_id"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

type Favorite struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	FileID    string    `json:"file_id"`
	CreatedAt time.Time `json:"created_at"`
}

type AuditLog struct {
	ID        string    `json:"id"`
	ActorID   string    `json:"actor_id"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Payload   []byte    `json:"payload"`
	CreatedAt time.Time `json:"created_at"`
}
