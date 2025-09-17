/* eslint-disable @typescript-eslint/no-var-requires */
module.exports = {
  root: true,
  parser: "@typescript-eslint/parser",
  parserOptions: { ecmaVersion: 2022, sourceType: "module" },
  env: { browser: true, node: true, es2022: true },
  extends: [
    "eslint:recommended",
    "plugin:@typescript-eslint/recommended",
    "plugin:jsx-a11y/recommended",
    "plugin:import/recommended",
    "plugin:import/typescript",
    "next/core-web-vitals",
    "prettier",
  ],
  plugins: ["@typescript-eslint", "unused-imports", "import"],
  settings: {
    react: { version: "detect" },
    "import/resolver": { typescript: true, node: true },
  },
  rules: {
    "no-console": ["warn", { allow: ["warn", "error"] }],
    "unused-imports/no-unused-imports": "error",
    "@typescript-eslint/no-unused-vars": "off",
    "import/order": [
      "warn",
      {
        "newlines-between": "always",
        groups: [["builtin", "external"], ["internal"], ["parent", "sibling", "index"]],
        alphabetize: { order: "asc", caseInsensitive: true },
      },
    ],
    "react/react-in-jsx-scope": "off",
  },
}
