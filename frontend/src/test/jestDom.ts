// jest-dom's matchers for every file, whether or not it imports them. Listed
// in vitest.config.ts by path rather than by package name: a setup file named
// by package is served from node_modules as it lies, and jest-dom then reaches
// aria-query as CommonJS, which a browser cannot import. Imported from here,
// it goes through Vite's bundled dependencies like the rest of the page.
import '@testing-library/jest-dom/vitest'
