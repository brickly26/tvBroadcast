// VideoPlayer.jsx
import React, { useEffect, useRef, useState } from "react";
import MP4Box from "mp4box";

/**
 * Props
 * ───────────────────────────────────────────────────────────
 * • channelNumber   – backend channel id (required)
 * • autoPlay        – start automatically (default true)
 * • className       – extra css classes for the outer container
 */
export default function VideoPlayer({
  channelNumber,
  autoPlay = true,
  className = "",
}) {
  /* ───────── refs / state ───────── */
  const videoRef = useRef(null);
  const mediaSourceRef = useRef(null);
  const mp4Ref = useRef(null);
  const sbRefs = useRef({});
  const queues = useRef({});
  const nextPos = useRef(0);
  const sessionRef = useRef(null);
  const fetchAbortRef = useRef(null);

  const [objURL, setObjURL] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [endpointStatus, setEndpointStatus] = useState(null);

  /* flags for the overlay status line */
  const hasInitSegmentRef = useRef(false);
  const moofSeen = useRef(false);

  const log = (...m) => console.log("[VideoPlayer]", ...m);

  /* ───────── queue helpers ───────── */
  const pump = (id) => {
    const sb = sbRefs.current[id];
    const q = queues.current[id];
    if (!sb || sb.updating || !q?.length) return;
    sb.appendBuffer(q.shift());
  };
  const enqueue = (id, buf) => {
    queues.current[id].push(buf);
    pump(id);
  };

  /* ───────── manual click handler (background play toggle) ───────── */
  const handlePlayerClick = () => {
    const v = videoRef.current;
    if (!v) return;
    if (v.paused) v.play().catch(() => {});
    // if playing, do nothing – or pause() if you prefer
  };

  /* ───────── main effect ───────── */
  useEffect(() => {
    /* reset ui flags */
    setLoading(true);
    setError("");
    setEndpointStatus(null);
    hasInitSegmentRef.current = false;
    moofSeen.current = false;

    /* quick HEAD probe so the overlay can show CORS / 404 issues early */
    (async () => {
      try {
        const res = await fetch(`/live/${channelNumber}`, { method: "HEAD" });
        setEndpointStatus({
          success: res.ok,
          message: res.ok
            ? "Endpoint looks healthy"
            : `HEAD ${res.status} ${res.statusText}`,
          isCORS: false,
        });
      } catch (err) {
        setEndpointStatus({
          success: false,
          message: err.message,
          isCORS: /TypeError: Failed to fetch/.test(err.toString()),
        });
      }
    })();

    /* new session token */
    const token = Symbol("session");
    sessionRef.current = token;

    /* MediaSource + URL */
    const ms = new MediaSource();
    mediaSourceRef.current = ms;
    const url = URL.createObjectURL(ms);
    setObjURL(url);
    if (videoRef.current) videoRef.current.src = url;
    log("Created MS URL:", url);

    /* MP4Box */
    const mp4 = MP4Box.createFile();
    mp4Ref.current = mp4;
    sbRefs.current = {};
    queues.current = {};
    nextPos.current = 0;
    let haveSeeked = false;

    mp4.onReady = (info) => {
      if (sessionRef.current !== token || ms.readyState !== "open") return;

      info.tracks.forEach((t) => {
        const mime = `${t.type === "audio" ? "audio" : "video"}/mp4; codecs="${
          t.codec
        }"`;
        const sb = ms.addSourceBuffer(mime);
        sbRefs.current[t.id] = sb;
        queues.current[t.id] = [];
        sb.mode = "segments";
        sb.addEventListener("updateend", () => {
          pump(t.id);

          /* seek to live edge once first video data buffered */
          if (
            !haveSeeked &&
            t.type === "video" &&
            sb.buffered.length &&
            sb.buffered.end(sb.buffered.length - 1) > 0
          ) {
            const edge = sb.buffered.end(sb.buffered.length - 1);
            videoRef.current.currentTime = edge - 0.3;
            videoRef.current
              .play()
              .catch(() => (videoRef.current.muted = true));
            haveSeeked = true;
          }
        });

        mp4.setSegmentOptions(t.id, null, { nbSamples: 100 });
      });

      mp4.initializeSegmentation()?.forEach(({ id, buffer }) => {
        buffer.fileStart = 0;
        hasInitSegmentRef.current = true;
        enqueue(id, buffer);
      });
      mp4.start();
    };

    mp4.onSegment = (id, _u, buf) => {
      if (sessionRef.current !== token || !buf) return;
      if (!moofSeen.current) moofSeen.current = true;
      enqueue(id, buf);
    };

    /* start fetch when MediaSource opens */
    ms.addEventListener("sourceopen", () => {
      const ctrl = new AbortController();
      fetchAbortRef.current = ctrl;
      (async () => {
        try {
          const res = await fetch(`/live/${channelNumber}`, {
            signal: ctrl.signal,
          });
          const rd = res.body?.getReader();
          if (!rd) throw new Error("No stream body");
          let i = 0;
          while (true) {
            const { value, done } = await rd.read();
            if (done || sessionRef.current !== token) break;
            if (!value?.byteLength) continue;
            const ab = value.buffer.slice(
              value.byteOffset,
              value.byteOffset + value.byteLength
            );
            if (i++ === 0) log("first chunk", ab.byteLength, "B");
            ab.fileStart = nextPos.current;
            nextPos.current += ab.byteLength;
            mp4.appendBuffer(ab);
          }
          mp4.flush();
        } catch (err) {
          if (sessionRef.current === token) setError(err.message);
        }
      })();
    });

    /* when the <video> actually starts rendering, loading overlay goes away */
    const onPlaying = () => setLoading(false);
    videoRef.current?.addEventListener("playing", onPlaying);

    /* cleanup */
    return () => {
      sessionRef.current = null;
      fetchAbortRef.current?.abort();
      mp4.stop();
      videoRef.current?.removeEventListener("playing", onPlaying);
      videoRef.current?.removeAttribute("src");
      if (objURL) URL.revokeObjectURL(objURL);
      log("cleanup complete");
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [channelNumber]);

  /* ───────── render ───────── */
  return (
    <div
      className={`video-player-container relative w-full h-full flex items-center justify-center ${className}`}
      onClick={handlePlayerClick}
    >
      {/* LOADING OVERLAY */}
      {loading && !error && (
        <div className="absolute inset-0 flex items-center justify-center bg-black/70 text-white z-10">
          {/* … unchanged overlay content … */}
        </div>
      )}

      {/* ERROR OVERLAY */}
      {error && (
        <div className="absolute inset-0 flex items-center justify-center bg-black/70 text-red-500 z-10">
          {error}
        </div>
      )}

      {/* VIDEO ELEMENT — scaled to fit */}
      <video
        ref={videoRef}
        className="max-w-full max-h-full object-contain bg-black"
        autoPlay
        muted
        playsInline
        controlsList="nodownload noremoteplayback nofullscreen noplaybackrate"
      />

      {/* LIVE BADGE */}
      <div className="absolute bottom-4 left-4 bg-black/60 text-white px-3 py-1 rounded">
        Live&nbsp;·&nbsp;Channel&nbsp;{channelNumber}
      </div>
    </div>
  );
}

{
  /* <div
      className="video-player-container relative w-full h-full"
      onClick={handlePlayerClick}
    >
      {loading && !error && (
        <div className="absolute inset-0 flex items-center justify-center bg-black/70 text-white z-10">
          <div className="text-center">
            <div>Loading channel&nbsp;{channelNumber}…</div>
            <div className="text-xs mt-1 opacity-80">
              Status:{" "}
              {hasInitSegmentRef.current
                ? "Processing initial segments..."
                : moofSeen.current
                ? "Found media fragments..."
                : "Waiting for stream..."}
            </div>
            {endpointStatus && !endpointStatus.success && (
              <div className="text-red-400 text-sm mt-2">
                {endpointStatus.message}
                {endpointStatus.isCORS && (
                  <div className="text-xs mt-1">
                    This appears to be a CORS issue. Please check your network
                    configuration.
                  </div>
                )}
              </div>
            )}
          </div>
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
        muted
        playsInline
        controlsList="nodownload noremoteplayback nofullscreen noplaybackrate"
      />

      <div className="absolute bottom-4 left-4 bg-black/60 text-white px-3 py-1 rounded">
        Live&nbsp;·&nbsp;Channel&nbsp;{channelNumber}
      </div>
    </div> */
}
