import noUnsanitized from 'eslint-plugin-no-unsanitized'

export default [
  {
    ignores: ['dist/', 'node_modules/'],
  },
  {
    files: ['src/**/*.js'],
    plugins: {
      'no-unsanitized': noUnsanitized,
    },
    rules: {
      // Block innerHTML/outerHTML assignments with dynamic content (XSS vectors)
      'no-unsanitized/property': 'error',
      // Block document.write, insertAdjacentHTML with dynamic content
      'no-unsanitized/method': 'error',
    },
  },
]
