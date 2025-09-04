// Package domain contains domain models for the application.
package domain

import (
	"errors"
	"time"
)

// CreateSnippetRequestDTO represents the expected request body for creating a snippet.
type CreateSnippetRequestDTO struct {
	Content   string   `json:"content" binding:"required,max=10240"`
	ExpiresIn int      `json:"expires_in" binding:"omitempty,gte=0,lte=2592000"`
	Tags      []string `json:"tags"`
}

// UpdateSnippetRequestDTO represents the expected request body for updating a snippet.
type UpdateSnippetRequestDTO struct {
	Content   string   `json:"content" binding:"required,max=10240"`
	ExpiresIn int      `json:"expires_in" binding:"omitempty,gte=0,lte=2592000"`
	Tags      []string `json:"tags"`
}

// SnippetResponseDTO represents the response for a single snippet.
type SnippetResponseDTO struct {
	ID        string   `json:"id"`
	Content   string   `json:"content"`
	CreatedAt string   `json:"created_at"`
	ExpiresAt *string  `json:"expires_at,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

// ListSnippetsResponseDTO represents the response for listing snippets.
type ListSnippetsResponseDTO struct {
	Page  int                  `json:"page"`
	Limit int                  `json:"limit"`
	Items []SnippetListItemDTO `json:"items"`
}

// SnippetListItemDTO represents a snippet in a list response.
type SnippetListItemDTO struct {
	ID        string  `json:"id"`
	CreatedAt string  `json:"created_at"`
	ExpiresAt *string `json:"expires_at,omitempty"`
}

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
