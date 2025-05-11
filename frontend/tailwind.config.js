/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  theme: {
    extend: {
      colors: {
        black: "#000000",
        white: "#ffffff",
        red: {
          500: "#ef4444",
        },
        gray: {
          500: "#6b7280",
          600: "#4b5563",
          800: "#1f2937",
        },
      },
    },
  },
  safelist: [
    "bg-black",
    "bg-red-500",
    "bg-opacity-70",
    "bg-opacity-80",
    "text-white",
  ],
  plugins: [],
};
