/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        background: 'var(--color-bg)',
        surface: 'var(--color-bg-card)',
        surfaceHover: 'var(--color-bg-card)', // Can adjust if needed
        primary: 'var(--color-primary)',
        primaryHover: 'var(--color-primary-hover)',
        primaryLight: 'var(--color-primary-light)',
        textPrimary: 'var(--color-text-primary)',
        textSecondary: 'var(--color-text-secondary)',
        textMuted: 'var(--color-text-muted)',
        border: 'var(--color-border)',
        borderLight: 'var(--color-border-light)',
        success: 'var(--color-success)',
        successSoft: 'var(--color-success-soft)',
        warning: 'var(--color-warning)',
        warningSoft: 'var(--color-warning-soft)',
        danger: 'var(--color-danger)',
        dangerSoft: 'var(--color-danger-soft)',
        info: 'var(--color-info)',
        infoSoft: 'var(--color-info-soft)',
      },
      borderRadius: {
        sm: 'var(--radius-sm)',
        md: 'var(--radius-md)',
        lg: 'var(--radius-lg)',
        xl: 'var(--radius-xl)',
      },
      boxShadow: {
        sm: 'var(--shadow-sm)',
        md: 'var(--shadow-md)',
        lg: 'var(--shadow-lg)',
        card: 'var(--shadow-card)',
      },
      animation: {
        'fade-in': 'fade-in 0.3s ease-out',
        'slide-up': 'slide-up 0.4s ease-out',
        'spin-slow': 'spin 3s linear infinite',
      },
      keyframes: {
        'fade-in': {
          '0%': { opacity: '0', transform: 'translateY(6px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        'slide-up': {
          '0%': { opacity: '0', transform: 'translateY(12px)', filter: 'blur(2px)' },
          '100%': { opacity: '1', transform: 'translateY(0)', filter: 'blur(0)' },
        }
      }
    },
  },
  plugins: [],
}
