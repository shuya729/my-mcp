package domain

import "time"

type User struct {
	ID        string
	Name      string
	AuthID    string
	CreatedAt time.Time
	UpdatedAt time.Time
}
