import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./src/pages/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/components/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/app/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        background: "var(--bg)",
        foreground: "var(--text)",
        primary: "var(--primary)",
        accent: "var(--accent)",
        success: "var(--success)",
        danger: "var(--danger)",
        muted: "var(--text-muted)",
      },
      boxShadow: {
        'glass-glow': '0 0 20px rgba(87, 125, 232, 0.25)',
        'glass-inner': 'inset 0 1px 0 rgba(255, 255, 255, 0.1)',
      }
    },
  },
  plugins: [],
};
export default config;
