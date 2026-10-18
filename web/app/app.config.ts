export default defineAppConfig({
  ui: {
    colors: {
      primary: 'blue',
      neutral: 'slate',
    },
  },
  toaster: {
    position: 'bottom-right',
    duration: 6000,
    max: 4,
    expand: true,
  },
})
