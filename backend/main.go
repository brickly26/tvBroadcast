package main

import (
	"fmt"
	"live-broadcast-backend/database"
	"live-broadcast-backend/handlers"
	"live-broadcast-backend/services"
	"live-broadcast-backend/state"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	gorillaHandlers "github.com/gorilla/handlers" // Correct alias syntax
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: No .env file found, using system environment variables")
	}

	// Initialize the channel state manager
	channelManager := state.NewChannelManager()

	// Initialize PostgreSQL database
	dbPath := os.Getenv("DATABASE_URL")
	if dbPath == "" {
		dbPath = "./tvstream.db" // Default path
	}
	db, err := database.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	
	// Create default channels in the database if needed
	err = db.CreateDefaultChannels()
	if err != nil {
		log.Printf("Warning: Failed to create default channels: %v", err)
	}

	// Initialize S3 video service
	s3Bucket := os.Getenv("S3_VIDEO_BUCKET")
	if s3Bucket == "" {
		log.Println("Warning: S3_VIDEO_BUCKET not set, using default")
		s3Bucket = "tvstream"
	}
	
	// Log AWS region for debugging
	awsRegion := os.Getenv("AWS_REGION")
	log.Printf("Using AWS region from environment: %s", awsRegion)
	
	videoService, err := services.NewVideoService(s3Bucket)
	if err != nil {
		log.Fatalf("Failed to initialize video service: %v", err)
	}
	
	// Configure local videos directory
	videoDir := os.Getenv("VIDEO_DIR")
	if videoDir == "" {
		videoDir = "./videos" // Default videos directory
	}
	
	// Initialize S3 manager for handling video content
	s3Manager, err := services.NewS3Manager(videoService, videoDir)
	if err != nil {
		log.Fatalf("Failed to initialize S3 manager: %v", err)
	}
	
	// Set the database provider for the channel manager
	channelManager.SetDBProvider(db)
	
	// Set the video provider for the channel manager
	channelManager.SetVideoProvider(s3Manager)
	
	// Initialize synchronization service for periodic updates from S3
	syncInterval := 15 // 15 minute interval for checking S3 updates
	syncService := services.NewSyncService(s3Manager, channelManager, syncInterval)
	
	// Start the sync service - this will immediately load only the first video for each channel
	// and set up just-in-time downloads for subsequent videos
	syncService.Start()
	log.Printf("Started S3 sync service with %d minute update interval (Just-In-Time video downloading enabled)", syncInterval)
	
	// Initialize the channel manager with database content
	if err := channelManager.InitializeFromDatabase(); err != nil {
		log.Printf("Warning: Failed to initialize channel manager from database: %v, falling back to S3 content", err)
		// Initial sync from S3 will be done by the sync service
	} else {
		log.Printf("Successfully initialized channel manager from database")
	}

	// Initialize YouTube downloader
	tempDir := os.Getenv("TEMP_DIR")
	if tempDir == "" {
		tempDir = "./temp" // Default temp directory
	}
	youtubeDownloader, err := services.NewYouTubeDownloader(videoService, db, tempDir)
	if err != nil {
		log.Fatalf("Failed to initialize YouTube downloader: %v", err)
	}

	// Initialize admin handler
	adminHandler := handlers.NewAdminHandler(db, youtubeDownloader, videoService)

	// Create a new router
	router := mux.NewRouter()
	
	// Setup secure video serving
	handlers.SetupVideoRoutes(router, channelManager, videoDir)

	// Setup API routes
	handlers.SetupChannelRoutes(router, channelManager)
	
	// Setup WebSocket handler with video service
	router.HandleFunc("/ws", handlers.WebSocketHandler(channelManager, videoService))

	// Setup Admin API routes
	apiRouter := router.PathPrefix("/api").Subrouter()
	
	// Admin authentication routes
	authRouter := apiRouter.PathPrefix("/auth").Subrouter()
	authRouter.HandleFunc("/login", adminHandler.LoginHandler()).Methods("POST")
	authRouter.HandleFunc("/verify-admin", adminHandler.VerifyAdminHandler()).Methods("GET")
	
	// Admin video management routes
	adminRouter := apiRouter.PathPrefix("/admin").Subrouter()
	adminRouter.HandleFunc("/upload-video", adminHandler.UploadVideoHandler()).Methods("POST")
	adminRouter.HandleFunc("/videos", adminHandler.GetChannelVideosHandler()).Methods("GET")
	adminRouter.HandleFunc("/delete-video", adminHandler.DeleteVideoHandler()).Methods("POST")
	adminRouter.HandleFunc("/update-video-order", adminHandler.UpdateVideoOrderHandler()).Methods("POST")
	adminRouter.HandleFunc("/channel", adminHandler.GetChannelDetailsHandler()).Methods("GET")
	adminRouter.HandleFunc("/channel/{channelID}", adminHandler.UpdateChannelDetailsHandler()).Methods("PUT")
	
	// Thumbnail route - serves S3 thumbnails
	apiRouter.PathPrefix("/thumbnails/").HandlerFunc(adminHandler.ThumbnailHandler())

	// Serve frontend static files from the frontend/dist directory
	frontendPath := filepath.Join("..", "frontend", "dist")
	if _, err := os.Stat(frontendPath); os.IsNotExist(err) {
		log.Printf("Warning: Frontend build directory not found at %s\n", frontendPath)
	} else {
		// Serve the index.html for the root path and SPA routing
		router.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip if the path is for videos (handled by the video routes)
			if strings.HasPrefix(r.URL.Path, "/videos/") {
				return
			}
			
			// Check if the file exists
			path := filepath.Join(frontendPath, r.URL.Path)
			_, err := os.Stat(path)
			
			// If the file doesn't exist or the request is for a directory, serve index.html
			if os.IsNotExist(err) || r.URL.Path == "/" {
				http.ServeFile(w, r, filepath.Join(frontendPath, "index.html"))
				return
			}
			
			// Otherwise, serve the file
			http.FileServer(http.Dir(frontendPath)).ServeHTTP(w, r)
		})
	}

	// CORS configuration
	// Allow requests from the typical React dev server port (3000)
	// and potentially a production frontend URL
	allowedOrigins := gorillaHandlers.AllowedOrigins([]string{"http://localhost:3000", "http://127.0.0.1:3000", "http://localhost:5173"}) // Add Vite dev server port
	allowedMethods := gorillaHandlers.AllowedMethods([]string{"GET", "POST", "OPTIONS"})
	allowedHeaders := gorillaHandlers.AllowedHeaders([]string{"Content-Type", "Authorization"})
	allowedCredentials := gorillaHandlers.AllowCredentials()

	// Create CORS middleware handler
	coredRouter := gorillaHandlers.CORS(allowedOrigins, allowedMethods, allowedHeaders, allowedCredentials)(router)

	// Start the server
	port := "8080"
	fmt.Printf("Backend server starting on port %s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, coredRouter))
} 