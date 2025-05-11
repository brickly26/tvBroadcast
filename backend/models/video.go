package models

import (
	"time"
)

// VideoStatus represents the current status of a video
type VideoStatus string

const (
	StatusPending   VideoStatus = "pending"
	StatusDownloading VideoStatus = "downloading"
	StatusProcessing  VideoStatus = "processing"
	StatusCompleted   VideoStatus = "completed"
	StatusFailed      VideoStatus = "failed"
)

// Update the existing Video struct to include admin fields
func init() {
	// This init function ensures the models package initializes properly
	// but doesn't actually modify any types at runtime
}

// AdminVideo represents a video uploaded through the admin interface
type AdminVideo struct {
	ID           string      `json:"id"`
	YoutubeURL   string      `json:"youtubeUrl"`
	S3Key        string      `json:"s3Key"`
	ChannelID    int         `json:"channelId"`
	Title        string      `json:"title"`
	Description  string      `json:"description,omitempty"`
	Status       VideoStatus `json:"status"`
	ErrorMsg     string      `json:"errorMsg,omitempty"`
	UploadedBy   string      `json:"uploadedBy"`
	CreatedAt    time.Time   `json:"createdAt"`
	UpdatedAt    time.Time   `json:"updatedAt"`
	Duration     float64     `json:"duration,omitempty"`
	Tags         []string    `json:"tags,omitempty"`
	URL          string      `json:"url,omitempty"`
	ThumbnailURL string      `json:"thumbnailUrl,omitempty"`
	DisplayOrder int         `json:"displayOrder,omitempty"`
}

// User represents an admin user who can upload videos
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Password  string    `json:"-"` // Password hash, not returned in JSON
	Role      string    `json:"role"` // "admin" or "user"
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
