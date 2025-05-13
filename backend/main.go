package main

import (
	"live-broadcast-backend/database"
	"live-broadcast-backend/handlers"
	"live-broadcast-backend/services"
	"live-broadcast-backend/state"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	gorillaHandlers "github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func main() {
	/* ─── ENV ───────────────────────────────────────────────────────────── */
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found – falling back to system env")
	}

	/* ─── CORE SINGLETONS ──────────────────────────────────────────────── */
	channelManager := state.NewChannelManager()

	/* database ---------------------------------------------------------------- */
	dbPath := os.Getenv("DATABASE_URL")
	if dbPath == "" {
		dbPath = "./tvstream.db"
	}
	db, err := database.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	if err := db.CreateDefaultChannels(); err != nil {
		log.Printf("Warning: could not create default channels: %v", err)
	}
	channelManager.SetDBProvider(db)

	/* S3 / video store -------------------------------------------------------- */
	s3Bucket := getenvDefault("S3_VIDEO_BUCKET", "tvstream")
	videoService, err := services.NewVideoService(s3Bucket)
	if err != nil {
		log.Fatalf("Failed to initialize video service: %v", err)
	}

	videoDir := getenvDefault("VIDEO_DIR", "./videos")
	s3Manager, err := services.NewS3Manager(videoService, videoDir)
	if err != nil {
		log.Fatalf("Failed to initialize S3 manager: %v", err)
	}
	channelManager.SetVideoProvider(s3Manager)

	/* sync S3 → local -------------------------------------------------------- */
	syncMinutes := 15
	syncService := services.NewSyncService(s3Manager, channelManager, syncMinutes)
	syncService.Start()
	log.Printf("Started S3 sync service (%d‑minute interval, JIT download)", syncMinutes)

	/* initialise channels from DB first, fall back to S3 on error ---------- */
	if err := channelManager.InitializeFromDatabase(); err != nil {
		log.Printf("DB init failed (%v); initial content will arrive via S3 sync", err)
	} else {
		log.Println("ChannelManager initialised from database")
	}

	/* YouTube downloader (optional admin feature) -------------------------- */
	tempDir := getenvDefault("TEMP_DIR", "./temp")
	youtubeDownloader, err := services.NewYouTubeDownloader(videoService, db, tempDir)
	if err != nil {
		log.Fatalf("Failed to init YouTube downloader: %v", err)
	}

	/* ─── ROUTER ─────────────────────────────────────────────────────────── */
	adminHandler := handlers.NewAdminHandler(db, youtubeDownloader, videoService)
	router := mux.NewRouter()

	/* secure file server for already‑downloaded MP4s */
	handlers.SetupVideoRoutes(router, channelManager, videoDir)

	/* JSON APIs */
	handlers.SetupChannelRoutes(router, channelManager)

	/* NEW live‑stream push */
	handlers.SetupLiveStreamRoutes(router, channelManager)

	/* WebSocket (chat / pings / admin dashboard) */
	router.HandleFunc("/ws", handlers.WebSocketHandler(channelManager, videoService))

	/* admin & auth sub‑routes */
	apiRouter := router.PathPrefix("/api").Subrouter()

	authRouter := apiRouter.PathPrefix("/auth").Subrouter()
	authRouter.HandleFunc("/login",        adminHandler.LoginHandler()).Methods("POST")
	authRouter.HandleFunc("/verify-admin", adminHandler.VerifyAdminHandler()).Methods("GET")

	adminRouter := apiRouter.PathPrefix("/admin").Subrouter()
	adminRouter.HandleFunc("/upload-video",       adminHandler.UploadVideoHandler()).Methods("POST")
	adminRouter.HandleFunc("/videos",             adminHandler.GetChannelVideosHandler()).Methods("GET")
	adminRouter.HandleFunc("/delete-video",       adminHandler.DeleteVideoHandler()).Methods("POST")
	adminRouter.HandleFunc("/update-video-order", adminHandler.UpdateVideoOrderHandler()).Methods("POST")
	adminRouter.HandleFunc("/channel",            adminHandler.GetChannelDetailsHandler()).Methods("GET")
	adminRouter.HandleFunc("/channel/{channelID}",adminHandler.UpdateChannelDetailsHandler()).Methods("PUT")
	apiRouter.PathPrefix("/thumbnails/").HandlerFunc(adminHandler.ThumbnailHandler())

	/* single‑page frontend build ------------------------------------------- */
	frontendDist := filepath.Join(".", "frontend", "dist")
	if _, err := os.Stat(frontendDist); err == nil {
		router.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// protect raw video downloads – they go through /videos/
			if strings.HasPrefix(r.URL.Path, "/videos/") { return }

			requested := filepath.Join(frontendDist, r.URL.Path)
			if _, err := os.Stat(requested); os.IsNotExist(err) || r.URL.Path == "/" {
				http.ServeFile(w, r, filepath.Join(frontendDist, "index.html"))
			} else {
				http.FileServer(http.Dir(frontendDist)).ServeHTTP(w, r)
			}
		})
	} else {
		log.Printf("Frontend build not found at %s (serving API only)", frontendDist)
	}

	/* ─── CORS & SERVER ──────────────────────────────────────────────────── */
	cors := gorillaHandlers.CORS(
		gorillaHandlers.AllowedOrigins([]string{
			"http://localhost:3000", "http://127.0.0.1:3000",
			"http://localhost:5173", // Vite
		}),
		gorillaHandlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		gorillaHandlers.AllowedHeaders([]string{"Content-Type", "Authorization"}),
		gorillaHandlers.AllowCredentials(),
	)

	addr := getenvDefault("LISTEN_ADDR", ":8080")
	log.Printf("Backend listening on %s …", addr)
	log.Fatal(http.ListenAndServe(addr, cors(router)))
}

/* utility */
func getenvDefault(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
