/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        sidebar: '#1e293b',
      },
      keyframes: {
        'slide-in': {
          '0%': { transform: 'translateX(100%)', opacity: '0' },
          '100%': { transform: 'translateX(0)', opacity: '1' },
        },
        pulse_section: {
          '0%, 100%': { boxShadow: '0 0 0 0 rgba(59, 130, 246, 0)' },
          '50%': { boxShadow: '0 0 0 4px rgba(59, 130, 246, 0.15)' },
        },
      },
      animation: {
        'slide-in': 'slide-in 0.3s ease-out',
        'pulse-section': 'pulse_section 1s ease-in-out 2',
      },
    },
  },
  plugins: [],
};
