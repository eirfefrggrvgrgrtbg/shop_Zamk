import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    include: [
      'src/components/search/**/*.test.{ts,tsx}',
      'src/pages/AdminSearchContextHandoff.test.tsx',
      'src/pages/AdminReturnRefundUI.test.tsx',
      'src/pages/AdminReturnReceiving.test.tsx',
      'src/pages/AdminReturns.test.ts',
      'src/pages/AdminModeration.test.tsx',
      'src/pages/AdminProductDetail.test.tsx',
      'src/components/EntityTimeline.test.tsx',
      'src/api/adminTimeline.test.ts',
      'src/components/inventory/**/*.test.{ts,tsx}',
      'src/pages/AdminGuidedPicking.test.tsx',
      'src/pages/AdminFreeScanner.test.tsx',
      'src/pages/AdminInventoryReconciliation.test.tsx',
    ],
  },
});
