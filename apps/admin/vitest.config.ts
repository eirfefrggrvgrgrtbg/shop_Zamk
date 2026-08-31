import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    include: [
      'src/components/search/**/*.test.{ts,tsx}',
      'src/pages/AdminSearchContextHandoff.test.tsx',
      'src/components/EntityTimeline.test.tsx',
      'src/api/adminTimeline.test.ts',
    ],
  },
});
