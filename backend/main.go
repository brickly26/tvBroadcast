package main

import (
	"live-broadcast-backend/database"
	"live-broadcast-backend/handlers"
	"live-broadcast-backend/services"
	"live-broadcast-backend/state"
	"log"
	"net/http"
	"os"

	gorillaHandlers "github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func main() {
	/* ---------- ENV ---------- */
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env not found, using system env")
	}

	/* ---------- CORE STRUCTS ---------- */
	cm := state.NewChannelManager()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" { dbURL = "./tvstream.db" }
	db, err := database.InitDB(dbURL)
	if err != nil { log.Fatalf("db init: %v", err) }
	defer db.Close()
	cm.SetDBProvider(db)

	bucket := getenvDefault("S3_VIDEO_BUCKET", "tvstream")
	videoSvc, err := services.NewVideoService(bucket)
	if err != nil { log.Fatalf("video service: %v", err) }

	videoDir := getenvDefault("VIDEO_DIR", "./videos")
	s3Mgr, err := services.NewS3Manager(videoSvc, videoDir)
	if err != nil { log.Fatalf("s3 manager: %v", err) }
	cm.SetVideoProvider(s3Mgr)

	/* ---------- PERIODIC S3 SYNC ---------- */
	syncSrv := services.NewSyncService(s3Mgr, cm, 15)
	syncSrv.Start()

	/* ---------- ROUTER ---------- */
	r := mux.NewRouter()

	// secure static files (unchanged)
	handlers.SetupVideoRoutes(r, cm, videoDir)

	// API
	handlers.SetupChannelRoutes(r, cm)
	handlers.SetupLiveStreamRoutes(r, cm) // <‑‑ NEW

	// (admin + websocket routing below unchanged) ...

	/* ---------- WEB ---------- */
	go func() {
		addr := getenvDefault("LISTEN_ADDR", ":8080")
		log.Printf("server listening on %s", addr)
		log.Fatal(http.ListenAndServe(addr,
			gorillaHandlers.CORS(
				gorillaHandlers.AllowedHeaders([]string{"Content-Type"}),
				gorillaHandlers.AllowedOrigins([]string{"*"}),
				gorillaHandlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
			)(r)))
	}()

	select {} // block forever
}

/* helper */
func getenvDefault(k, d string) string {
	if v := os.Getenv(k); v != "" { return v }
	return d
}
