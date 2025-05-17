import React, { useState, useEffect, useRef } from "react";

/**
 * A simple component to test WebSocket connectivity.
 * This can be imported and used temporarily to diagnose connection issues.
 */
function WebSocketTest() {
  const [status, setStatus] = useState("Initializing...");
  const [log, setLog] = useState([]);
  const wsRef = useRef(null);

  const addLog = (message) => {
    setLog((prev) => [
      ...prev,
      `${new Date().toISOString().substring(11, 19)}: ${message}`,
    ]);
  };

  useEffect(() => {
    // Helper to get WebSocket URL
    const getWsUrl = () => {
      const wsProtocol = window.location.protocol === "https:" ? "wss:" : "ws:";
      return `${wsProtocol}//${window.location.host}/ws`;
    };

    // Create WebSocket connection
    const connectWs = () => {
      try {
        const wsUrl = getWsUrl();
        addLog(`Connecting to ${wsUrl}`);

        const ws = new WebSocket(wsUrl);
        wsRef.current = ws;

        ws.onopen = () => {
          setStatus("Connected");
          addLog("WebSocket connected successfully");

          // Send a channel guide request
          const message = JSON.stringify({ type: "getChannelGuide" });
          ws.send(message);
          addLog(`Sent: ${message}`);
        };

        ws.onmessage = (event) => {
          addLog(`Received: ${event.data.substring(0, 100)}...`);
          try {
            const data = JSON.parse(event.data);
            addLog(`Message type: ${data.type}`);
          } catch (err) {
            addLog(`Error parsing message: ${err.message}`);
          }
        };

        ws.onclose = (event) => {
          setStatus("Disconnected");
          addLog(
            `WebSocket closed: Code ${event.code}, Reason: ${
              event.reason || "none"
            }`
          );
        };

        ws.onerror = (error) => {
          setStatus("Error");
          addLog(`WebSocket error occurred`);
        };

        return ws;
      } catch (err) {
        setStatus("Connection Error");
        addLog(`Error creating WebSocket: ${err.message}`);
        return null;
      }
    };

    // Connect on component mount
    const ws = connectWs();

    // Cleanup on unmount
    return () => {
      if (ws) {
        addLog("Closing WebSocket due to component unmount");
        ws.close(1000, "Component unmounting");
      }
    };
  }, []);

  // Send a test message
  const sendTestMessage = () => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      const message = JSON.stringify({ type: "joinChannel", channel: 1 });
      wsRef.current.send(message);
      addLog(`Sent: ${message}`);
    } else {
      addLog("Cannot send message - WebSocket not connected");
    }
  };

  return (
    <div
      style={{
        padding: "1rem",
        fontFamily: "monospace",
        border: "1px solid #ccc",
        borderRadius: "4px",
        maxWidth: "600px",
      }}
    >
      <h3>WebSocket Test</h3>
      <div style={{ marginBottom: "1rem" }}>
        <strong>Status:</strong>{" "}
        <span
          style={{
            color:
              status === "Connected"
                ? "green"
                : status === "Disconnected"
                ? "orange"
                : status === "Error"
                ? "red"
                : "blue",
          }}
        >
          {status}
        </span>
      </div>

      <button
        onClick={sendTestMessage}
        style={{
          padding: "0.5rem 1rem",
          backgroundColor: "#4CAF50",
          color: "white",
          border: "none",
          borderRadius: "4px",
          cursor: "pointer",
          marginBottom: "1rem",
        }}
      >
        Send Test Message
      </button>

      <div
        style={{
          border: "1px solid #ddd",
          padding: "0.5rem",
          height: "200px",
          overflowY: "auto",
          backgroundColor: "#f8f8f8",
        }}
      >
        {log.map((entry, index) => (
          <div key={index} style={{ fontSize: "0.9rem", marginBottom: "4px" }}>
            {entry}
          </div>
        ))}
      </div>
    </div>
  );
}

export default WebSocketTest;
