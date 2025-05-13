import React, { useState, useEffect } from "react";
import VideoPlayer from "./components/VideoPlayer";
import FloatingMenu from "./components/FloatingMenu";
import ChannelGuide from "./components/ChannelGuide";
import AdminDashboard from "./components/AdminDashboard";
import UnmuteButton from "./components/UnmuteButton";
import ConnectionStatus from "./components/ConnectionStatus";
import LoginForm from "./components/LoginForm";
import useWebSocket from "./hooks/useWebSocket";
import { Toaster } from "react-hot-toast";

const App = () => {
  // State for managing the application
  const [muted, setMuted] = useState(true); // Default to muted
  const [volume, setVolume] = useState(50); // Default volume (0-100)
  const [showGuide, setShowGuide] = useState(false);
  const [showDebug, setShowDebug] = useState(false);
  const [showAdmin, setShowAdmin] = useState(false);
  const [isAdmin, setIsAdmin] = useState(false); // User role state
  const [showLoginForm, setShowLoginForm] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [authError, setAuthError] = useState("");

  // Using our enhanced WebSocket hook for real-time updates
  const {
    connectionStatus,
    channelData,
    channelGuide,
    currentChannel,
    joinChannel,
    requestChannelGuide,
    lastMessage,
  } = useWebSocket();

  // Get current video info for the selected channel
  const currentVideoInfo = channelData[currentChannel] || {
    url: "",
    currentTime: 0,
  };

  // Handle channel change
  const changeChannel = (channelNumber) => {
    if (channelNumber < 1 || channelNumber > 5) return;

    joinChannel(channelNumber);

    // Close the guide when changing channels
    setShowGuide(false);
  };

  // Handle volume change
  const changeVolume = (newVolume) => {
    setVolume(Math.max(0, Math.min(100, newVolume)));
  };

  // Toggle mute state
  const toggleMute = () => {
    setMuted(!muted);
  };

  // Toggle channel guide
  const toggleGuide = () => {
    if (!showGuide && channelGuide.length === 0) {
      // If opening the guide and we don't have data, request it
      requestChannelGuide();
    }
    setShowGuide(!showGuide);
  };

  // Toggle admin dashboard
  const toggleAdmin = () => {
    if (isAdmin) {
      setShowAdmin(!showAdmin);
    } else {
      setShowLoginForm(true);
    }
  };

  // Handle login attempt
  const handleLogin = async (username, password) => {
    setIsLoading(true);
    setAuthError("");

    try {
      const response = await fetch("/api/auth/login", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({ username, password }),
      });

      const data = await response.json();

      if (response.ok && data.success) {
        setIsAdmin(true);
        setShowLoginForm(false);
        setShowAdmin(true);
      } else {
        setAuthError(data.message || "Invalid credentials");
      }
    } catch (error) {
      console.error("Login error:", error);
      setAuthError("Server error. Please try again.");
    } finally {
      setIsLoading(false);
    }
  };

  // Close login form
  const closeLoginForm = () => {
    setShowLoginForm(false);
    setAuthError("");
  };

  // Check for admin credentials
  useEffect(() => {
    const checkAdminStatus = async () => {
      setIsLoading(true);
      try {
        const response = await fetch("/api/auth/verify-admin", {
          credentials: "include",
        });

        if (response.ok) {
          const data = await response.json();
          if (data.success && data.isAdmin) {
            setIsAdmin(true);
          }
        }
      } catch (error) {
        console.error("Error checking admin status:", error);
      } finally {
        setIsLoading(false);
      }
    };

    checkAdminStatus();
  }, []);

  // Toggle debug display with keyboard shortcut
  useEffect(() => {
    const handleKeyDown = (e) => {
      // Press 'D' key to toggle debug display
      if (e.key === "d" || e.key === "D") {
        setShowDebug((prev) => !prev);
      }

      // Admin shortcut (Ctrl+Shift+A)
      if (e.ctrlKey && e.shiftKey && (e.key === "a" || e.key === "A")) {
        if (isAdmin) {
          setShowAdmin(true);
        } else {
          setShowLoginForm(true);
        }
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  return (
    <div className="fixed inset-0 h-screen w-screen bg-black overflow-hidden">
      {/* Toast Notifications */}
      <Toaster
        position="top-right"
        toastOptions={{
          style: {
            background: "#333",
            color: "#fff",
          },
          success: {
            duration: 3000,
            iconTheme: {
              primary: "#10B981",
              secondary: "#fff",
            },
          },
          error: {
            duration: 4000,
            iconTheme: {
              primary: "#EF4444",
              secondary: "#fff",
            },
          },
        }}
      />

      {/* Video Player */}
      <VideoPlayer
        muted={muted}
        volume={volume / 100} // Convert to 0-1 range for HTML5 video
        channelNumber={currentChannel}
      />

      {/* Floating Menu (Bottom Right) */}
      <FloatingMenu
        currentChannel={currentChannel}
        changeChannel={changeChannel}
        volume={volume}
        changeVolume={changeVolume}
        muted={muted}
        toggleMute={toggleMute}
        toggleGuide={toggleGuide}
        toggleAdmin={toggleAdmin}
        isAdmin={isAdmin}
      />

      {/* Unmute Button (Bottom Left) */}
      <UnmuteButton muted={muted} toggleMute={toggleMute} />

      {/* Channel Guide (Modal) */}
      {showGuide && (
        <ChannelGuide
          channelInfo={channelGuide}
          currentChannel={currentChannel}
          changeChannel={changeChannel}
          closeGuide={() => setShowGuide(false)}
        />
      )}

      {/* Admin Dashboard (Modal) */}
      {showAdmin && isAdmin && (
        <AdminDashboard
          closeAdmin={() => setShowAdmin(false)}
          isOpen={showAdmin}
        />
      )}

      {/* Login Form (Modal) */}
      {showLoginForm && (
        <LoginForm
          onLogin={handleLogin}
          onCancel={closeLoginForm}
          isLoading={isLoading}
          error={authError}
        />
      )}

      {/* Connection Status Bar */}
      {connectionStatus !== "connected" && (
        <div className="absolute top-0 left-0 right-0 bg-red-500 text-white text-center py-1 z-50">
          {connectionStatus === "connecting"
            ? "Connecting..."
            : "Connection lost. Reconnecting..."}
        </div>
      )}

      {/* Debug Connection Status Display - Toggle with 'D' key */}
      {showDebug && <ConnectionStatus connectionStatus={connectionStatus} />}
    </div>
  );
};

export default App;
