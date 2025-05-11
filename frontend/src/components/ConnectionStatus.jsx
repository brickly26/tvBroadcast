import React, { useState, useEffect } from "react";

const ConnectionStatus = ({ connectionStatus }) => {
  const [startTime, setStartTime] = useState(Date.now());
  const [currentTime, setCurrentTime] = useState(Date.now());
  const [connectionHistory, setConnectionHistory] = useState([]);

  // Update the current time every second
  useEffect(() => {
    const timer = setInterval(() => {
      setCurrentTime(Date.now());
    }, 1000);

    return () => clearInterval(timer);
  }, []);

  // Track connection status changes
  useEffect(() => {
    setConnectionHistory((prev) => [
      ...prev.slice(-9), // Keep last 10 entries
      {
        status: connectionStatus,
        timestamp: new Date().toLocaleTimeString(),
        uptime: Math.floor((Date.now() - startTime) / 1000),
      },
    ]);
  }, [connectionStatus, startTime]);

  // Calculate uptime
  const uptime = Math.floor((currentTime - startTime) / 1000);
  const formatTime = (seconds) => {
    const min = Math.floor(seconds / 60);
    const sec = seconds % 60;
    return `${min}m ${sec}s`;
  };

  // Different colors based on connection status
  const getStatusColor = (status) => {
    switch (status) {
      case "connected":
        return "text-green-500";
      case "connecting":
        return "text-yellow-500";
      case "disconnected":
        return "text-red-500";
      default:
        return "text-white";
    }
  };

  return (
    <div className="absolute bottom-0 left-0 p-2 bg-black bg-opacity-70 text-xs z-50">
      <div className="flex items-center space-x-2">
        <div className={`font-bold ${getStatusColor(connectionStatus)}`}>
          {connectionStatus.toUpperCase()}
        </div>
        <div className="text-gray-300">Uptime: {formatTime(uptime)}</div>
      </div>

      {/* History log - only show when expanded */}
      <div className="mt-1 text-gray-400 max-h-20 overflow-y-auto text-[10px]">
        {connectionHistory.map((entry, i) => (
          <div key={i} className="flex space-x-2">
            <span className={getStatusColor(entry.status)}>●</span>
            <span>{entry.timestamp}</span>
            <span>({formatTime(entry.uptime)})</span>
            <span className="capitalize">{entry.status}</span>
          </div>
        ))}
      </div>
    </div>
  );
};

export default ConnectionStatus;
