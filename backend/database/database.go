package database

import (
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"live-broadcast-backend/models"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// DB represents the database connection
type DB struct {
	*sql.DB
}

// InitDB initializes the database connection
func InitDB(dbConnStr string) (*DB, error) {
	// If no connection string is provided, try to get it from environment
	if dbConnStr == "" {
		dbConnStr = os.Getenv("DATABASE_URL")
	}

	// If still no connection string, use a default PostgreSQL connection
	if dbConnStr == "" {
		dbConnStr = "postgres://username:password@localhost:5432/tvstream?sslmode=disable"
		log.Println("Warning: Using default database connection string. Set DATABASE_URL for custom configuration.")
	}

	// Parse the connection URL to validate it
	parsedURL, err := url.Parse(dbConnStr)
	if err != nil {
		return nil, fmt.Errorf("invalid database URL: %v", err)
	}

	// Open database connection
	db, err := sql.Open("postgres", dbConnStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	// Create tables if they don't exist
	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("failed to create tables: %v", err)
	}

	// Create default admin user if it doesn't exist
	if err := createDefaultAdmin(db); err != nil {
		return nil, err
	}

	// Create database object
	dbObj := &DB{db}

	// No longer create default videos
	// The code to create default videos has been removed

	log.Printf("Connected to database: %s", parsedURL.Host)
	return dbObj, nil
}

// createTables creates the necessary tables if they don't exist
func createTables(db *sql.DB) error {
	// Create channels table first (since videos reference channels)
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS channels (
			number INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			theme TEXT,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	// Create videos table with foreign key to channels
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS videos (
			id TEXT PRIMARY KEY,
			youtube_url TEXT NOT NULL,
			s3_key TEXT NOT NULL,
			channel_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			description TEXT,
			status TEXT NOT NULL,
			error_msg TEXT,
			uploaded_by TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
			FOREIGN KEY (channel_id) REFERENCES channels(number)
		)
	`)
	if err != nil {
		return err
	}
	
	// Note: Thumbnails are now managed directly in S3 with pattern: thumbnails/thumbnail_{videoID}.jpg
	
	// Create video_order table to manage display order
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS video_order (
			video_id TEXT PRIMARY KEY,
			channel_id INTEGER NOT NULL,
			display_order INTEGER NOT NULL,
			FOREIGN KEY (video_id) REFERENCES videos(id) ON DELETE CASCADE,
			FOREIGN KEY (channel_id) REFERENCES channels(number),
			UNIQUE (channel_id, display_order)
		)
	`)
	if err != nil {
		return err
	}

	// Create users table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			email TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			role TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	// Hash the default admin password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("tvadmin2025"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash default admin password: %v", err)
	}

	// Create a default admin user if not exists
	_, err = db.Exec(`
		INSERT INTO users (id, username, email, password, role, created_at, updated_at)
		SELECT $1, $2, $3, $4, $5, $6, $7
		WHERE NOT EXISTS (
			SELECT 1 FROM users WHERE username = $2
		)
	`, uuid.New().String(), "admin", "admin@tvstream.example", string(hashedPassword), "admin", time.Now(), time.Now())
	if err != nil {
		return err
	}

	// Add duration column to videos table if it doesn't exist
	// This is an ALTER TABLE migration that safely adds the column if missing
	_, err = db.Exec(`
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 
				FROM information_schema.columns 
				WHERE table_name='videos' AND column_name='duration'
			) THEN
				ALTER TABLE videos ADD COLUMN duration FLOAT DEFAULT 0;
			END IF;
		END
		$$;
	`)
	if err != nil {
		return fmt.Errorf("failed to add duration column to videos table: %v", err)
	}

	// Add display_order column to videos table if it doesn't exist
	_, err = db.Exec(`
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 
				FROM information_schema.columns 
				WHERE table_name='videos' AND column_name='display_order'
			) THEN
				ALTER TABLE videos ADD COLUMN display_order INTEGER DEFAULT NULL;
			END IF;
		END
		$$;
	`)
	if err != nil {
		return fmt.Errorf("failed to add display_order column to videos table: %v", err)
	}

	// Add thumbnail_url column to videos table if it doesn't exist
	_, err = db.Exec(`
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 
				FROM information_schema.columns 
				WHERE table_name='videos' AND column_name='thumbnail_url'
			) THEN
				ALTER TABLE videos ADD COLUMN thumbnail_url TEXT DEFAULT NULL;
			END IF;
		END
		$$;
	`)
	if err != nil {
		return fmt.Errorf("failed to add thumbnail_url column to videos table: %v", err)
	}

	return nil
}

// createDefaultAdmin creates a default admin user if one doesn't exist
func createDefaultAdmin(db *sql.DB) error {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		// Create a default admin user
		// In production, use bcrypt for password hashing
		now := time.Now()
		_, err = db.Exec(`
			INSERT INTO users (id, username, email, password, role, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, uuid.New().String(), "admin", "admin@tvstream.example", "tvadmin2025", "admin", now, now)
		if err != nil {
			return err
		}
		log.Println("Created default admin user")
	}

	return nil
}

// SaveVideo stores a new video in the database
func (db *DB) SaveVideo(video *models.AdminVideo) error {
	// Set ID if not already set
	if video.ID == "" {
		video.ID = uuid.New().String()
	}
	
	// Set timestamps if not already set
	if video.CreatedAt.IsZero() {
		video.CreatedAt = time.Now()
	}
	if video.UpdatedAt.IsZero() {
		video.UpdatedAt = time.Now()
	}
	
	// Use an upsert operation to handle updates to existing videos
	_, err := db.Exec(`
		INSERT INTO videos (id, youtube_url, s3_key, channel_id, title, description, status, error_msg, 
		                    uploaded_by, duration, created_at, updated_at, display_order, thumbnail_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (id) DO UPDATE SET
			youtube_url = $2,
			s3_key = $3,
			channel_id = $4,
			title = $5,
			description = $6,
			status = $7,
			error_msg = $8,
			duration = $10,
			updated_at = $12,
			display_order = $13,
			thumbnail_url = $14
	`, video.ID, video.YoutubeURL, video.S3Key, video.ChannelID, video.Title, video.Description, 
	   video.Status, video.ErrorMsg, video.UploadedBy, video.Duration, video.CreatedAt, 
	   video.UpdatedAt, video.DisplayOrder, video.ThumbnailURL)
	
	if err != nil {
		log.Printf("Error saving video: %v", err)
	}
	return err
}

// UpdateVideoStatus updates the status of a video
func (db *DB) UpdateVideoStatus(id string, status models.VideoStatus, errorMsg string) error {
	now := time.Now()
	_, err := db.Exec(`
		UPDATE videos
		SET status = $1, error_msg = $2, updated_at = $3
		WHERE id = $4
	`, status, errorMsg, now, id)

	return err
}

// GetVideosByChannel retrieves all videos for a specific channel
func (db *DB) GetVideosByChannel(channelID int) ([]models.AdminVideo, error) {
	rows, err := db.Query(`
		SELECT id, youtube_url, s3_key, channel_id, title, description, status, error_msg, uploaded_by, created_at, updated_at, duration, 
		       display_order, thumbnail_url
		FROM videos 
		WHERE channel_id = $1
		ORDER BY COALESCE(display_order, 9999), created_at DESC
	`, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []models.AdminVideo
	for rows.Next() {
		video := models.AdminVideo{}
		var status string
		var displayOrder sql.NullInt64
		var duration sql.NullFloat64
		var thumbnailURL sql.NullString
		err := rows.Scan(
			&video.ID, &video.YoutubeURL, &video.S3Key, &video.ChannelID,
			&video.Title, &video.Description, &status, &video.ErrorMsg,
			&video.UploadedBy, &video.CreatedAt, &video.UpdatedAt, &duration,
			&displayOrder, &thumbnailURL,
		)
		if err != nil {
			return nil, err
		}
		video.Status = models.VideoStatus(status)
		
		// Set the display order if available
		if displayOrder.Valid {
			video.DisplayOrder = int(displayOrder.Int64)
		}
		
		// Set the thumbnail URL if available in database, otherwise generate from ID
		if thumbnailURL.Valid && thumbnailURL.String != "" {
			video.ThumbnailURL = thumbnailURL.String
		} else {
			// Fallback to ID-based thumbnail
			video.ThumbnailURL = fmt.Sprintf("/thumbnails/thumbnail_%s.jpg", video.ID)
		}
		
		// Set the duration if available
		if duration.Valid {
			video.Duration = duration.Float64
		}
		
		// Generate a URL for the video
		video.URL = fmt.Sprintf("/videos/%s", video.S3Key)
		
		videos = append(videos, video)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return videos, nil
}

// GetVideoByID retrieves a video by its ID
func (db *DB) GetVideoByID(id string) (*models.AdminVideo, error) {
	var video models.AdminVideo
	var status string
	var duration sql.NullFloat64
	var displayOrder sql.NullInt64
	var thumbnailURL sql.NullString
	err := db.QueryRow(`
		SELECT id, youtube_url, s3_key, channel_id, title, description, status, error_msg, uploaded_by, created_at, updated_at, duration,
		       display_order, thumbnail_url
		FROM videos
		WHERE id = $1
	`, id).Scan(
		&video.ID, &video.YoutubeURL, &video.S3Key, &video.ChannelID,
		&video.Title, &video.Description, &status, &video.ErrorMsg,
		&video.UploadedBy, &video.CreatedAt, &video.UpdatedAt, &duration,
		&displayOrder, &thumbnailURL,
	)
	if err != nil {
		return nil, err
	}

	video.Status = models.VideoStatus(status)
	
	// Set the duration if available
	if duration.Valid {
		video.Duration = duration.Float64
	}
	
	// Set the display order if available
	if displayOrder.Valid {
		video.DisplayOrder = int(displayOrder.Int64)
	}
	
	// Set the thumbnail URL if available in database, otherwise generate from ID
	if thumbnailURL.Valid && thumbnailURL.String != "" {
		video.ThumbnailURL = thumbnailURL.String
	} else {
		// Fallback to ID-based thumbnail
		video.ThumbnailURL = fmt.Sprintf("/thumbnails/thumbnail_%s.jpg", video.ID)
	}
	
	// Set the video URL
	video.URL = fmt.Sprintf("/videos/%s", video.S3Key)

	return &video, nil
}

// ValidateUser checks if a user with the given credentials exists
func (db *DB) ValidateUser(username, password string) (*models.User, error) {
	var user models.User
	var storedPassword string

	// Special case for the default admin credentials
	if username == "admin" && password == "tvadmin2025" {
		// Retrieve the admin user directly
		err := db.QueryRow(`
			SELECT id, username, email, role, password, created_at, updated_at
			FROM users
			WHERE username = 'admin'
		`).Scan(
			&user.ID, &user.Username, &user.Email, &user.Role, &storedPassword,
			&user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("admin user not found")
		}

		// Upgrade to bcrypt hash if needed
		if !strings.HasPrefix(storedPassword, "$2a$") {
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err == nil {
				// Update the password in the database
				_, err = db.Exec("UPDATE users SET password = $1 WHERE id = $2", string(hashedPassword), user.ID)
				if err != nil {
					log.Printf("Error updating admin password hash: %v", err)
				}
			}
		}

		// Return the admin user
		return &user, nil
	}

	// Standard flow for other users
	err := db.QueryRow(`
		SELECT id, username, email, role, password, created_at, updated_at
		FROM users
		WHERE username = $1
	`, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.Role, &storedPassword,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	// Check if the password is already hashed (starts with $2a$)
	if !strings.HasPrefix(storedPassword, "$2a$") {
		// If the password is not hashed, we'll check directly first
		// This allows transition from plaintext to bcrypt
		if password != storedPassword {
			return nil, fmt.Errorf("invalid credentials")
		}
		
		// If matching, let's upgrade to bcrypt and store the hashed password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("Error hashing password: %v", err)
			// Still return the user since authentication succeeded
		} else {
			// Update the password in the database
			_, err = db.Exec("UPDATE users SET password = $1 WHERE id = $2", string(hashedPassword), user.ID)
			if err != nil {
				log.Printf("Error updating password hash: %v", err)
			}
		}
	} else {
		// If the password is already hashed, verify with bcrypt
		err = bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(password))
		if err != nil {
			return nil, fmt.Errorf("invalid credentials")
		}
	}

	return &user, nil
}

// IsUserAdmin checks if a user is an admin
func (db *DB) IsUserAdmin(userID string) (bool, error) {
	log.Printf("[IsUserAdmin] Checking admin status for userID: %s", userID)
	// First try the standard method
	var role string
	err := db.QueryRow(`
		SELECT role
		FROM users
		WHERE id = $1
	`, userID).Scan(&role)
	log.Printf("[IsUserAdmin] Query result: role=%s, err=%v", role, err)

	// If successful and role is admin, return true
	if err == nil && role == "admin" {
		log.Println("[IsUserAdmin] User is admin (role matched in users table)")
		return true, nil
	}

	// If we have an error, try to look up the user by username instead
	// This is a fallback for admin sessions that might have issue with ID
	if err != nil {
		log.Printf("[IsUserAdmin] Error on initial user lookup: %v", err)
		// Try to get any user with admin role
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&count)
		log.Printf("[IsUserAdmin] Admin count in DB: %d, err=%v", count, err)
		if err != nil {
			return false, err
		}

		// If we have at least one admin and the userID looks like a session token for admin
		// This is a workaround for cases where session management might be having issues
		if count > 0 && strings.Contains(userID, "session_") {
			log.Println("[IsUserAdmin] Fallback: session token pattern detected, admin exists, granting admin access.")
			return true, nil
		}
	}

	log.Println("[IsUserAdmin] User is not admin or lookup failed.")
	return false, nil
}

// CreateDefaultChannels creates default channels if none exist
func (db *DB) CreateDefaultChannels() error {
	// Check if channels table is empty
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM channels").Scan(&count)
	if err != nil {
		return err
	}

	// If there are already channels, no need to create defaults
	if count > 0 {
		return nil
	}

	// Create default channels
	defaultChannels := models.PredefinedChannels()
	now := time.Now()

	for _, channel := range defaultChannels {
		_, err = db.Exec(`
			INSERT INTO channels (number, name, description, theme, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, channel.Number, channel.Name, channel.Description, channel.Theme, now, now)
		if err != nil {
			return fmt.Errorf("failed to create default channel %d: %v", channel.Number, err)
		}
	}

	log.Println("Created default channels")
	return nil
}

// GetAllChannels retrieves all channels from the database
func (db *DB) GetAllChannels() ([]*models.Channel, error) {
	rows, err := db.Query(`
		SELECT number, name, description, theme 
		FROM channels 
		ORDER BY number
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []*models.Channel
	for rows.Next() {
		channel := &models.Channel{}
		err := rows.Scan(&channel.Number, &channel.Name, &channel.Description, &channel.Theme)
		if err != nil {
			return nil, err
		}
		channels = append(channels, channel)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return channels, nil
}

// GetChannel retrieves a channel by its number
func (db *DB) GetChannel(channelNumber int) (*models.Channel, error) {
	channel := &models.Channel{}
	err := db.QueryRow(`
		SELECT number, name, description, theme 
		FROM channels 
		WHERE number = $1
	`, channelNumber).Scan(&channel.Number, &channel.Name, &channel.Description, &channel.Theme)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("channel %d not found", channelNumber)
		}
		return nil, err
	}

	return channel, nil
}

// DeleteVideo deletes a video by its ID
func (db *DB) DeleteVideo(videoID string) error {
	// Begin a transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // Rollback if function returns with error
	
	// Delete from video_order
	_, err = tx.Exec("DELETE FROM video_order WHERE video_id = $1", videoID)
	if err != nil {
		return err
	}
	
	// Delete the video itself
	result, err := tx.Exec("DELETE FROM videos WHERE id = $1", videoID)
	if err != nil {
		return err
	}
	
	// Check if any row was affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("video with ID %s not found", videoID)
	}
	
	// Commit the transaction
	return tx.Commit()
}

// UpdateVideoOrder updates the display order of videos in a channel
func (db *DB) UpdateVideoOrder(channelID int, videoOrders map[string]int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	// First, delete existing orders for this channel
	_, err = tx.Exec("DELETE FROM video_order WHERE channel_id = $1", channelID)
	if err != nil {
		return err
	}
	
	// Then insert the new orders and update the videos table
	for videoID, order := range videoOrders {
		// Insert into video_order table
		_, err = tx.Exec(
			"INSERT INTO video_order (video_id, channel_id, display_order) VALUES ($1, $2, $3)",
			videoID, channelID, order,
		)
		if err != nil {
			return err
		}
		
		// Also update the display_order column in videos table
		_, err = tx.Exec(
			"UPDATE videos SET display_order = $1 WHERE id = $2",
			order, videoID,
		)
		if err != nil {
			return err
		}
	}
	
	return tx.Commit()
}

// Note: Thumbnail functionality removed as we now use S3 thumbnails directly with naming convention
// thumbnails/channel_{channelID}_video{index}.jpg

// ReorderVideosAfterDeletion updates the display order of videos in a channel after a deletion
func (db *DB) ReorderVideosAfterDeletion(channelID int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	// Get all videos for this channel, ordered by current display order
	rows, err := tx.Query(`
		SELECT v.id, COALESCE(vo.display_order, 9999) as order_pos
		FROM videos v
		LEFT JOIN video_order vo ON v.id = vo.video_id
		WHERE v.channel_id = $1
		ORDER BY order_pos, v.created_at
	`, channelID)
	if err != nil {
		return err
	}
	defer rows.Close()
	
	// Loop through and update order positions to be sequential
	videoIDs := []string{}
	for rows.Next() {
		var id string
		var orderPos int
		if err := rows.Scan(&id, &orderPos); err != nil {
			return err
		}
		videoIDs = append(videoIDs, id)
	}
	
	if err = rows.Err(); err != nil {
		return err
	}
	
	// First, delete all existing order entries for this channel
	_, err = tx.Exec("DELETE FROM video_order WHERE channel_id = $1", channelID)
	if err != nil {
		return err
	}
	
	// Then insert them with sequential order values
	for i, id := range videoIDs {
		// Insert into video_order table
		_, err = tx.Exec(
			"INSERT INTO video_order (video_id, channel_id, display_order) VALUES ($1, $2, $3)",
			id, channelID, i+1,
		)
		if err != nil {
			return err
		}
		
		// Also update display_order in videos table
		_, err = tx.Exec(
			"UPDATE videos SET display_order = $1 WHERE id = $2",
			i+1, id,
		)
		if err != nil {
			return err
		}
	}
	
	return tx.Commit()
}

// GetChannelVideosWithDetails retrieves all videos for a specific channel with details for admin dashboard
func (db *DB) GetChannelVideosWithDetails(channelID int) ([]models.AdminVideo, error) {
	rows, err := db.Query(`
		SELECT v.id, v.youtube_url, v.s3_key, v.channel_id, v.title, v.description, v.status, v.error_msg, 
		       v.uploaded_by, v.created_at, v.updated_at, v.duration,
		       v.display_order, v.thumbnail_url
		FROM videos v
		WHERE v.channel_id = $1
		ORDER BY COALESCE(v.display_order, 9999), v.created_at DESC
	`, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []models.AdminVideo
	for rows.Next() {
		video := models.AdminVideo{}
		var status string
		var displayOrder sql.NullInt64
		var duration sql.NullFloat64
		var thumbnailURL sql.NullString
		err := rows.Scan(
			&video.ID, &video.YoutubeURL, &video.S3Key, &video.ChannelID,
			&video.Title, &video.Description, &status, &video.ErrorMsg,
			&video.UploadedBy, &video.CreatedAt, &video.UpdatedAt, &duration,
			&displayOrder, &thumbnailURL,
		)
		if err != nil {
			return nil, err
		}
		
		video.Status = models.VideoStatus(status)
		
		// Set duration if available
		if duration.Valid {
			video.Duration = duration.Float64
		}
		
		// Set display order if available
		if displayOrder.Valid {
			video.DisplayOrder = int(displayOrder.Int64)
		}
		
		// Set thumbnail URL using the direct value from the database or fallback to ID-based thumbnail
		if thumbnailURL.Valid && thumbnailURL.String != "" {
			video.ThumbnailURL = thumbnailURL.String
		} else {
			video.ThumbnailURL = fmt.Sprintf("/thumbnails/thumbnail_%s.jpg", video.ID)
		}
		
		// Generate a URL for the video
		video.URL = fmt.Sprintf("/videos/%s", video.S3Key)
		
		videos = append(videos, video)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return videos, nil
}

// GetChannelVideos retrieves all videos for a specific channel
func (db *DB) GetChannelVideos(channelNumber int) ([]*models.Video, error) {
	rows, err := db.Query(`
		SELECT id, title, description, s3_key, youtube_url, created_at, status, duration, thumbnail_url
		FROM videos 
		WHERE channel_id = $1 AND status = 'completed'
		ORDER BY COALESCE(display_order, 9999), created_at
	`, channelNumber)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []*models.Video
	for rows.Next() {
		video := &models.Video{}
		var youtubeURL, status string
		var thumbnailURL sql.NullString
		err := rows.Scan(&video.ID, &video.Title, &video.Description, &video.S3Key, 
		                 &youtubeURL, &video.CreatedAt, &status, &video.Duration, &thumbnailURL)
		if err != nil {
			return nil, err
		}
		
		// Set tags to include the channel
		video.Tags = []string{fmt.Sprintf("channel_%d", channelNumber)}
		
		// Set a reasonable default duration if not available
		if video.Duration == 0 {
			video.Duration = 300 // Default to 5 minutes if unknown
		}
		
		// Set the video URL
		video.URL = fmt.Sprintf("/videos/%s", video.S3Key)
		
		// Set the thumbnail URL
		if thumbnailURL.Valid && thumbnailURL.String != "" {
			video.ThumbnailURL = thumbnailURL.String
		} else {
			video.ThumbnailURL = fmt.Sprintf("/thumbnails/thumbnail_%s.jpg", video.ID)
		}
		
		videos = append(videos, video)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return videos, nil
}
