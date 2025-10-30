package model

import "time"

type Task struct {
	ID int64 `json:"id"`
	Title string `json:"title"`
	Status string `json:"status"`
	Priority int `json:"priority"`
	Version int `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}


type TaskFilter struct {
	Status *string
}
