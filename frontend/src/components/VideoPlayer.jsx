/*  VideoPlayer.jsx  ─────────────────────────────────────────────────────
 *
 *  Requirements :
 *    npm i mp4box
 *  or (if you already added it)
 *    import MP4Box from "mp4box/dist/mp4box.all.min.js";
 *
 *  This component:
 *    • fetches  /live/{channelNumber}  (fragmented‑MP4)
 *    • parses   incoming boxes with mp4box.js
 *    • pushes   ready segments to Media Source Extensions
 *    • shows    spinner / error overlays like the original
 *    • keeps    the same Tailwind classes & channel label
 * -------------------------------------------------------------------- */
import React, { useEffect, useRef, useState } from "react";
import MP4Box from "mp4box"; // ←  gpac/mp4box.js

const API_HOST =
  process.env.NODE_ENV === "production"
    ? window.location.origin
    : "http://localhost:8080";

function VideoPlayer({ channelNumber, muted = true, volume = 1 }) {
  const videoRef = useRef(null);

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  /* ─── create / tear‑down whenever the channel changes ─────────────── */
  useEffect(() => {
    if (channelNumber == null) return;

    const video = videoRef.current;
    if (!video) return;

    setLoading(true);
    setError(null);

    /* 1. boot MediaSource */
    const mediaSource = new MediaSource();
    const sourceBuffers = {}; // keyed by trackId
    video.src = URL.createObjectURL(mediaSource);

    /* 2. boot mp4box.js */
    const mp4boxFile = MP4Box.createFile();
    let nextFilePos = 0; // byte offset fed so far
    let fetchReader; // cancelled on cleanup

    /* ── mp4box callbacks ──────────────────────────────────────────── */
    mp4boxFile.onError = (e) => {
      console.error("mp4box error", e);
      setError("Stream parse error");
    };

    /** called once init segments are parsed */
    mp4boxFile.onReady = (info) => {
      /* For each track (video + audio) create one SourceBuffer */
      info.tracks.forEach((trk) => {
        const mime = `video/mp4; codecs="${trk.codec}"`;
        if (!MediaSource.isTypeSupported(mime)) {
          setError(`Browser cannot play ${trk.codec}`);
          return;
        }

        const sb = mediaSource.addSourceBuffer(mime);
        sb.mode = "segments";
        sourceBuffers[trk.id] = { sb, queue: [] };

        /* flush queued segments when SB finishes updating */
        sb.addEventListener("updateend", () => flushQueue(trk.id));
      });

      /* ask mp4box to generate segments forever */
      info.tracks.forEach((trk) => {
        mp4boxFile.setSegmentOptions(trk.id, null, { nbSegments: Infinity });
      });
      mp4boxFile.initializeSegmentation();
    };

    /** called every time mp4box has a full fMP4 segment ready */
    mp4boxFile.onSegment = (id, user, buffer) => {
      const seg = new Uint8Array(buffer);
      const buf = sourceBuffers[id];
      if (!buf) return; // should not happen
      buf.queue.push(seg);
      flushQueue(id);
      if (loading) setLoading(false);
    };

    /* helper: feed SourceBuffer if it's idle */
    const flushQueue = (id) => {
      const { sb, queue } = sourceBuffers[id];
      if (!sb || sb.updating || !queue.length) return;
      sb.appendBuffer(queue.shift());
    };

    /* ── once MediaSource opens, start the network fetch ───────────── */
    const handleSourceOpen = () => {
      fetch(`${API_HOST}/live/${channelNumber}`)
        .then((res) => {
          fetchReader = res.body.getReader();

          /** recursive read‑loop */
          const pump = () =>
            fetchReader.read().then(({ value, done }) => {
              if (done) {
                mp4boxFile.flush();
                return;
              }
              const ab = value.buffer.slice(
                value.byteOffset,
                value.byteOffset + value.byteLength
              );
              ab.fileStart = nextFilePos;
              nextFilePos += ab.byteLength;
              mp4boxFile.appendBuffer(ab); // hand chunk to mp4box
              pump();
            });

          pump();
        })
        .catch((e) => {
          console.error("Fetch error", e);
          setError("Network error");
        });
    };

    mediaSource.addEventListener("sourceopen", handleSourceOpen);

    /* final player tweaks */
    video.muted = muted;
    video.volume = volume;
    video.play().catch(() => {
      /* wait for user gesture */
    });

    /* ─── cleanup ──────────────────────────────────────────────────── */
    return () => {
      mediaSource.removeEventListener("sourceopen", handleSourceOpen);
      fetchReader?.cancel();
      mp4boxFile.flush();
      video.pause();
      video.removeAttribute("src");
      video.load();
    };
  }, [channelNumber]);

  /* keep mute / volume reactive */
  useEffect(() => {
    const v = videoRef.current;
    if (v) {
      v.muted = muted;
      v.volume = volume;
    }
  }, [muted, volume]);

  /* ─── UI – exact same Tailwind look & feel ────────────────────────── */
  return (
    <div className="video-player-container relative w-full h-full">
      {loading && !error && (
        <div className="absolute inset-0 flex items-center justify-center bg-black/70 text-white z-10">
          Loading channel&nbsp;{channelNumber}…
        </div>
      )}

      {error && (
        <div className="absolute inset-0 flex items-center justify-center bg-black/70 text-red-500 z-10">
          {error}
        </div>
      )}

      <video
        ref={videoRef}
        className="w-full h-full object-cover bg-black"
        autoPlay
        playsInline
        controlsList="nodownload noremoteplayback nofullscreen noplaybackrate"
      />

      <div className="absolute bottom-4 left-4 bg-black/60 text-white px-3 py-1 rounded">
        Live&nbsp;·&nbsp;Channel&nbsp;{channelNumber}
      </div>
    </div>
  );
}

export default VideoPlayer;
