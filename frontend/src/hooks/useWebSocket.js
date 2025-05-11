import { useState, useEffect, useCallback, useRef } from "react";

const useWebSocket = (path = "ws") => {
  const [lastMessage, setLastMessage] = useState(null);
  const [lastParsedMessage, setLastParsedMessage] = useState(null);
  const [connectionStatus, setConnectionStatus] = useState("connecting");
  const [channelData, setChannelData] = useState({});
  const [channelGuide, setChannelGuide] = useState([]);
  const [currentChannel, setCurrentChannel] = useState(1);
  const socketRef = useRef(null);
  const reconnectTimeoutRef = useRef(null);
  const sendMessageRef = useRef(null);
  const reconnectAttemptsRef = useRef(0);
  const maxReconnectDelay = 30000; // Maximum reconnect delay of 30 seconds

  // Handle video update messages from the server
  const handleVideoUpdate = useCallback((message) => {
    const { channel, url, currentTime, duration, videoId, title } = message;

    // If we get an empty URL, preserve the previous URL if available
    // This prevents flickering when channel state has temporary issues
    setChannelData((prevData) => {
      const prevChannelData = prevData[channel] || {};
      const prevVideoId = prevChannelData.videoId || '';
      const now = Date.now();
      
      // Only update if we have a URL or if this is a new video
      if (!url && prevChannelData.url && prevVideoId === videoId) {
        // Just update the time but keep the existing URL
        return {
          ...prevData,
          [channel]: {
            ...prevChannelData,
            currentTime: currentTime || prevChannelData.currentTime || 0,
            lastUpdated: now,
            // Calculate the server-client time difference for smoother sync
            syncOffset: now - (prevChannelData.lastServerSync || 0),
            lastServerSync: now,
            lastServerTime: currentTime || 0,
          },
        };
      }

      // Full update with new video data
      return {
        ...prevData,
        [channel]: {
          url: url || prevChannelData.url || "",
          videoId: videoId || prevChannelData.videoId || "",
          title: title || prevChannelData.title || "",
          currentTime: url ? currentTime : prevChannelData.currentTime || 0,
          duration: duration || prevChannelData.duration || 0,
          lastUpdated: now,
          lastServerSync: now,
          lastServerTime: currentTime || 0,
        },
      };
    });
  }, []);

  // Handle channel guide messages from the server
  const handleChannelGuide = useCallback((message) => {
    if (message.data && Array.isArray(message.data)) {
      setChannelGuide(message.data);
    } else if (message.channelInfo && Array.isArray(message.channelInfo)) {
      setChannelGuide(message.channelInfo);
    }
  }, []);

  // Send a message to the WebSocket server
  const sendMessage = useCallback((message) => {
    if (socketRef.current && socketRef.current.readyState === WebSocket.OPEN) {
      // If message is an object, stringify it
      const msgString =
        typeof message === "string" ? message : JSON.stringify(message);
      socketRef.current.send(msgString);
      return true;
    }
    console.warn("WebSocket not connected, message not sent");
    return false;
  }, []);

  // Join a specific channel
  const joinChannel = useCallback(
    (channelNumber) => {
      if (channelNumber < 1 || channelNumber > 5) {
        console.error("Invalid channel number:", channelNumber);
        return false;
      }

      setCurrentChannel(channelNumber);

      return sendMessage({
        type: "joinChannel",
        channel: channelNumber,
      });
    },
    [sendMessage]
  );

  // Request the channel guide from the server
  const requestChannelGuide = useCallback(() => {
    return sendMessage({
      type: "getChannelGuide",
    });
  }, [sendMessage]);

  // Calculate exponential backoff for reconnections
  const getReconnectDelay = useCallback(() => {
    const attempts = reconnectAttemptsRef.current;
    // Base delay of 1 second with exponential backoff (2^attempts)
    const delay = Math.min(1000 * Math.pow(2, attempts), maxReconnectDelay);
    console.log(`Reconnect attempt ${attempts + 1}, delay: ${delay}ms`);
    return delay;
  }, []);

  // Initialize WebSocket connection - only runs once on mount
  useEffect(() => {
    // Function to create the WebSocket connection
    const establishConnection = () => {
      // If there's an existing socket, properly close it first
      if (
        socketRef.current &&
        socketRef.current.readyState !== WebSocket.CLOSED
      ) {
        console.log(
          "Closing existing WebSocket connection before creating a new one"
        );
        socketRef.current.close(1000, "Creating new connection");
      }

      // Connect to the backend WebSocket server
      // Make sure this points to your Go backend server, not the frontend dev server
      const apiBaseUrl = process.env.NODE_ENV === 'production' 
        ? window.location.origin
        : 'http://localhost:8080';

      const wsProtocol = apiBaseUrl.startsWith('https') ? 'wss:' : 'ws:';
      const wsHost = apiBaseUrl.replace(/^https?:\/\//, '');
      const wsUrl = `${wsProtocol}//${wsHost}/ws`;

      console.log("Creating WebSocket connection to:", wsUrl);
      const socket = new WebSocket(wsUrl);
      socketRef.current = socket;

      socket.onopen = () => {
        console.log("WebSocket connected");
        setConnectionStatus("connected");
        reconnectAttemptsRef.current = 0;

        // Request initial channel guide and join default channel
        // Use setTimeout to ensure the socket is fully ready
        setTimeout(() => {
          if (socket.readyState === WebSocket.OPEN) {
            sendMessage({
              type: "getChannelGuide",
            });

            sendMessage({
              type: "joinChannel",
              channel: currentChannel,
            });
          }
        }, 500);
      };

      socket.onmessage = (event) => {
        // Only handle actual data messages - ignore pings
        if (event.data && event.data.length > 0) {
          setLastMessage(event.data);

          try {
            // Try direct parsing first
            let parsed;
            try {
              parsed = JSON.parse(event.data);
            } catch (directError) {
              console.log(
                "Direct parsing failed, trying to extract valid JSON"
              );

              let data = event.data;
              const startIndex = data.indexOf("{");
              const endIndex = data.lastIndexOf("}");

              if (
                startIndex !== -1 &&
                endIndex !== -1 &&
                endIndex > startIndex
              ) {
                data = data.substring(startIndex, endIndex + 1);
                parsed = JSON.parse(data);
              } else {
                throw directError;
              }
            }

            setLastParsedMessage(parsed);

            // Handle different message types
            if (parsed && parsed.type) {
              switch (parsed.type) {
                case "videoUpdate":
                  handleVideoUpdate(parsed);
                  break;
                case "channelGuide":
                  handleChannelGuide(parsed);
                  break;
                default:
                  console.log("Received unknown message type:", parsed.type);
              }
            } else {
              console.warn("Received message with no type field:", parsed);
            }
          } catch (error) {
            console.error(
              "Error parsing WebSocket message:",
              error,
              "Raw data:",
              event.data
            );
          }
        }
      };

      socket.onclose = (event) => {
        console.log("WebSocket disconnected", event.code, event.reason);

        // Don't change to disconnected status for normal closures
        if (event.code !== 1000) {
          setConnectionStatus("disconnected");

          // Start reconnection process if not already in progress
          if (!reconnectTimeoutRef.current) {
            // Mark first reconnect attempt
            if (reconnectAttemptsRef.current === 0) {
              reconnectAttemptsRef.current = 1;
            }

            const delay = getReconnectDelay();
            console.log(`Reconnecting in ${delay}ms...`);

            reconnectTimeoutRef.current = setTimeout(() => {
              console.log("Attempting to reconnect...");
              setConnectionStatus("connecting");
              establishConnection();
              reconnectTimeoutRef.current = null;
            }, delay);
          }
        }
      };

      socket.onerror = (error) => {
        console.error("WebSocket error:", error);
      };

      return socket;
    };

    // Create the initial connection
    establishConnection();

    // Cleanup function when component unmounts
    return () => {
      if (socketRef.current) {
        console.log("Component unmounting, closing WebSocket connection");
        socketRef.current.close(1000, "Component unmounting");
        socketRef.current = null;
      }

      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
        reconnectTimeoutRef.current = null;
      }
    };
  }, []); // Empty dependency array - only runs once on mount

  // Store sendMessage in ref to avoid circular dependencies
  // This is set up after the useEffect to ensure it's available for socket setup
  sendMessageRef.current = sendMessage;

  return {
    lastMessage, // Raw message data from server
    lastParsedMessage, // Parsed JSON message from server
    connectionStatus, // WebSocket connection status
    channelData, // Current video data for all channels
    channelGuide, // Information about all available channels
    currentChannel, // The currently selected channel
    sendMessage, // General-purpose message sending function
    joinChannel, // Function to join a specific channel
    requestChannelGuide, // Function to request the channel guide
  };
};

export default useWebSocket;
