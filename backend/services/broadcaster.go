package services

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// Broadcaster streams one video to many HTTP clients in real‑time.
type Broadcaster struct {
	channelNumber int
	chunkSize     int           // bytes per write
	targetBps     int           // bytes per second (≈bitrate / 8)
	srcPath       string        // current local file
	initSegment   []byte        // cached ftyp+moov
	mu            sync.Mutex
	clients       map[*client]struct{}
}

type client struct {
	w    http.ResponseWriter
	done chan struct{}
}

func NewBroadcaster(channelNumber int, path string, chunkSize, targetBps int) (*Broadcaster, error) {
	b := &Broadcaster{
		channelNumber: channelNumber,
		chunkSize:     chunkSize,
		targetBps:     targetBps,
		srcPath:       path,
		clients:       make(map[*client]struct{}),
	}

	// sniff the init boxes once
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := make([]byte, 2*1024*1024) // read first 2 MB
	n, _ := io.ReadFull(f, buf)
	if init := extractInit(buf[:n]); len(init) > 0 {
		b.initSegment = init
	}
	go b.loop() // start pumping immediately
	return b, nil
}

// AddClient wires a fresh HTTP connection into the fan‑out.
func (b *Broadcaster) AddClient(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Cache-Control", "no-cache")

	cl := &client{w: w, done: make(chan struct{})}

	// send init boxes first so the decoder can attach
	if _, err := w.Write(b.initSegment); err == nil {
		w.(http.Flusher).Flush()
	}

	b.mu.Lock()
	b.clients[cl] = struct{}{}
	b.mu.Unlock()

	<-r.Context().Done() // block until the browser tab closes

	b.mu.Lock()
	delete(b.clients, cl)
	b.mu.Unlock()
	close(cl.done)
}

// SwitchSource is called by ChannelManager the instant it rotates to the
// next video for this channel.
func (b *Broadcaster) SwitchSource(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, 2*1024*1024)
	n, _ := io.ReadFull(f, buf)
	if init := extractInit(buf[:n]); len(init) > 0 {
		b.initSegment = init
	}
	b.srcPath = path
	return nil
}

// --- private ---

func (b *Broadcaster) loop() {
	sleepChunk := b.targetBps / 20 // 50 ms cadence
	sleepDur := 50 * time.Millisecond
	readBuf := make([]byte, b.chunkSize)

	for {
		f, err := os.Open(b.srcPath)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		for {
			n, err := f.Read(readBuf)
			if err == io.EOF {
				f.Close()
				break // done – ChannelManager will flip src and we reopen
			}
			chunk := readBuf[:n]

			b.mu.Lock()
			for cl := range b.clients {
				if _, werr := cl.w.Write(chunk); werr == nil {
					cl.w.(http.Flusher).Flush()
				}
			}
			b.mu.Unlock()

			// pacing
			for sent := 0; sent < n; sent += sleepChunk {
				time.Sleep(sleepDur)
			}
		}
	}
}

// naive MP4 init extractor – identical to the one in the prototype
func extractInit(data []byte) []byte {
	m := bytes.Index(data, []byte("moov"))
	if m < 4 {
		return nil
	}
	size := int(data[m-4])<<24 | int(data[m-3])<<16 | int(data[m-2])<<8 | int(data[m-1])
	end := m - 4 + size
	if end > len(data) {
		return nil
	}
	return data[:end]
}