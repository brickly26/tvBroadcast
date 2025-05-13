package services

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// Broadcaster streams one fragmented MP4 to many clients in real‑time.
type Broadcaster struct {
	channelNumber int
	targetBps     int           // bytes per second (≈ bitrate / 8)
	srcPath       string        // fMP4 on disk
	initSegment   []byte        // cached ftyp+moov
	mu            sync.Mutex
	clients       map[*client]struct{}
}

type client struct {
	w    http.ResponseWriter
	done chan struct{}
}

func NewBroadcaster(chNum int, path string, _chunkSize, targetBps int) (*Broadcaster, error) {
	b := &Broadcaster{
		channelNumber: chNum,
		targetBps:     targetBps,
		srcPath:       path,
		clients:       make(map[*client]struct{}),
	}

	initSeg, err := buildInitSegment(path)
	if err != nil {
		return nil, fmt.Errorf("init extract: %w", err)
	}
	b.initSegment = initSeg

	go b.loop()
	return b, nil
}

func (b *Broadcaster) AddClient(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Cache-Control", "no-cache")

	cl := &client{w: w, done: make(chan struct{})}

	// push init immediately
	if _, err := w.Write(b.initSegment); err == nil {
		w.(http.Flusher).Flush()
	}

	b.mu.Lock()
	b.clients[cl] = struct{}{}
	b.mu.Unlock()

	<-r.Context().Done() // wait for disconnect
	b.mu.Lock()
	delete(b.clients, cl)
	b.mu.Unlock()
	close(cl.done)
}

func (b *Broadcaster) SwitchSource(path string) error {
	// read new init and swap src
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

/* ---------- internal pump ---------- */

func (b *Broadcaster) loop() {
	bytesPerSleep := b.targetBps / 20 // 50 ms cadence
	sleepDur := 50 * time.Millisecond

	for {
		f, err := os.Open(b.srcPath)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		// seek past cached init boxes
		if _, err := f.Seek(int64(len(b.initSegment)), io.SeekStart); err != nil {
			f.Close()
			time.Sleep(time.Second)
			continue
		}

		for {
			frag, err := nextFragment(f) // moof+mdat
			if err == io.EOF {
				f.Close()
				break // finished file – wait for ChannelManager to swap
			}
			if err != nil {
				log.Printf("channel %d: fragment error (%v) – skipping file", b.channelNumber, err)
				f.Close()

				// signal fatal error by clearing srcPath
				b.mu.Lock()
				b.srcPath = ""
				b.mu.Unlock()
				break
			}

			/* fan‑out to clients */
			b.mu.Lock()
			for cl := range b.clients {
				if _, werr := cl.w.Write(frag); werr == nil {
					cl.w.(http.Flusher).Flush()
				}
			}
			b.mu.Unlock()

			/* simple pacing */
			for sent := 0; sent < len(frag); sent += bytesPerSleep {
				time.Sleep(sleepDur)
			}
		}
	}
}

/* ---------- fragment helpers ---------- */

type mp4box struct {
	typ  string
	data []byte
}

func (b *Broadcaster) CurrentPath() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.srcPath
}

func readBox(r io.Reader) (mp4box, error) {
	hdr := make([]byte, 8)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return mp4box{}, err
	}
	size32 := binary.BigEndian.Uint32(hdr[:4])
	typ := string(hdr[4:8])

	var payloadSize uint64
	switch size32 {
	case 0: // box extends to EOF – treat as EOF padding and stop
		return mp4box{}, io.EOF
	case 1: // 64‑bit size follows
		ext := make([]byte, 8)
		if _, err := io.ReadFull(r, ext); err != nil {
			return mp4box{}, err
		}
		payloadSize = binary.BigEndian.Uint64(ext) - 16
		hdr = append(hdr, ext...)
	default:
		payloadSize = uint64(size32) - 8
	}

	payload := make([]byte, payloadSize)
	if _, err := io.ReadFull(r, payload); err != nil {
		return mp4box{}, err
	}
	return mp4box{typ: typ, data: append(hdr, payload...)}, nil
}

func buildInitSegment(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var buf bytes.Buffer
	for {
		box, err := readBox(f)
		if err != nil {
			return nil, err
		}
		if box.typ == "moof" {
			// rewind  the file cursor by the size of the moof box header
			// so Broadcaster.loop() sees the moof again.
			if _, err := f.Seek(-int64(len(box.data)), io.SeekCurrent); err != nil {
				return nil, err
			}
			break
		}
		buf.Write(box.data)
	}
	return buf.Bytes(), nil
}

func nextFragment(r io.Reader) ([]byte, error) {
	for {
		box, err := readBox(r)
		if err != nil {
			return nil, err
		}
		if box.typ != "moof" {
			// silently ignore padding / free / uuid / ftyp etc.
			continue
		}

		mdat, err := readBox(r)
		if err != nil {
			return nil, err
		}
		if mdat.typ != "mdat" {
			// very unlikely, but skip and keep searching
			continue
		}
		return append(box.data, mdat.data...), nil
	}
}

/* ---------- naive init extractor ---------- */

func extractInit(data []byte) []byte {
    moof := bytes.Index(data, []byte("moof"))
    if moof < 4 {
        return nil // moof not found in probe
    }
    return data[:moof-4] // include size field of moof's header ⇒ exclude moof itself
}