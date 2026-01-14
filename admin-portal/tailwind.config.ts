import type { Config } from "tailwindcss";

export default {
  content: ["./app/**/*.{ts,tsx}", "./components/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        bg: "#fff8e6",
        panel: "#fff3d1",
        panel2: "#ffe7b0",
        text: "#1f2937",
        muted: "#6b7280",
        brand: "#f59e0b",
        brand2: "#7f1d1d",
        danger: "#ef4444"
      },
      boxShadow: {
        soft: "0 10px 30px rgba(0,0,0,0.35)"
      }
    }
  },
  plugins: []
} satisfies Config;
