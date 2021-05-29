module.exports = {
  env: {
    browser: true,
    es2017: true,
  },
  extends: [
    'plugin:react/recommended',
    'standard',
  ],
  parser: '@typescript-eslint/parser',
  parserOptions: {
    ecmaFeatures: {
      jsx: true,
    },
    sourceType: 'module',
  },
  plugins: [
    'react',
    '@typescript-eslint',
  ],
  rules: {
    'no-undef': 'off',
    'no-use-before-define': 'off',
    'object-curly-spacing': ['warn', 'never'],
    // Require NO spaces before the function parentheses.
    'space-before-function-paren': ['warn', 'never'],
    camelcase: 'off',
    'no-unused-vars': 'off',
    'block-spacing': 'off',
    'comma-dangle': ['warn', {
      arrays: 'always-multiline',
      objects: 'always-multiline',
      imports: 'always-multiline',
      functions: 'ignore',
    }],
  },
}
