/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./src/**/*.{js,jsx,ts,tsx}",
  ],
  presets: [
    require('@cloistr/tailwind-config'),
  ],
  theme: {
    extend: {
      colors: {
        // Brand purple for text. The preset's cloistr-primary is the fill
        // colour (#7c3aed), which is 3.05:1 on the dark background and fails
        // WCAG AA; @cloistr/ui defines this lighter text value for dark mode
        // (identical in light mode). The preset has no utility for it yet.
        'cloistr-primary-text': 'var(--cloistr-primary-text)',
      },
    },
  },
  plugins: [
    require('@tailwindcss/forms'),
  ],
}