import tseslint from 'typescript-eslint'
import reactCompiler from 'eslint-plugin-react-compiler'

export default tseslint.config(
  {
    ignores: ['.next/**', 'node_modules/**'],
  },
  ...tseslint.configs.recommended,
  {
    plugins: {
      'react-compiler': reactCompiler,
    },
    rules: {
      'react-compiler/react-compiler': 'error',
      '@typescript-eslint/no-explicit-any': 'off',
      '@typescript-eslint/no-unused-vars': [
        'warn',
        {
          argsIgnorePattern: '^_',
          varsIgnorePattern: '^_',
          caughtErrorsIgnorePattern: '^_',
        },
      ],
    },
  }
)
