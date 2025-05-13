package models

import "time"

type BroadcastMessage struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}
