// Package domain contains domain models for the application.
package domain

import (
	"errors"
	"time"
)

// Snippet represents a code snippet entity.
type Snippet struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

var (
	// ErrTitleRequired is returned when a snippet title is missing.
	ErrTitleRequired = errors.New("title required")
	// ErrSlugTaken is returned when a snippet slug already exists.
	ErrSlugTaken = errors.New("slug already exists")
)
