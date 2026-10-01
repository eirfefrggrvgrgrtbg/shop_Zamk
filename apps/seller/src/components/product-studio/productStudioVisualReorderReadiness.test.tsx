/* @vitest-environment jsdom */
import { vi, describe, it, expect } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ProductStudioVisualWorkspace } from './ProductStudioVisualWorkspace';
import {
  ProductStudioProvider,
  useProductStudio,
} from '../../contexts/ProductStudioContext';
import { getMediaReadinessWarning } from './productStudioMediaHelper';

const { mockColors } = vi.hoisted(() => ({
  mockColors: [
    { id: 'col-black', nameRu: 'Чёрный', code: 'black', hex: '#000000' },
    { id: 'col-white', nameRu: 'Белый', code: 'white', hex: '#ffffff' },
    { id: 'col-gray', nameRu: 'Серый', code: 'gray', hex: '#808080' },
  ],
}));

vi.mock('@zamk/api-client/src/seller', async (importOriginal: any) => {
  const actual = await importOriginal();
  return {
    ...actual,
    getSellerColors: vi.fn().mockResolvedValue(mockColors),
    getSellerCategorySchema: vi.fn().mockResolvedValue({
      id: 'cat-1',
      dimensionType: 'COLOR_AND_SIZE',
      attributes: [],
      sizeChartFields: [],
    }),
    getSellerSizeValues: vi.fn().mockResolvedValue([]),
  };
});

describe('SELLER MEDIA.2C2 — Real Reorder Interaction + Correct Colorway Readiness Copy', () => {
  describe('Visual Thumbnail Reorder (Tests A, B, C)', () => {
    it('A. actual Visual thumbnail reorder interaction works via UI drag events', async () => {
      let studioCtx: ReturnType<typeof useProductStudio> | null = null;
      function ContextWatcher() {
        studioCtx = useProductStudio();
        return null;
      }

      render(
        <MemoryRouter>
          <ProductStudioProvider
            entryMode="create"
            initialDraft={{
              mediaMode: 'COLORWAY',
              colors: [
                { id: 'col-black', name: 'Чёрный', hex: '#000000' },
                { id: 'col-white', name: 'Белый', hex: '#ffffff' },
              ],
              variants: [
                { colorId: 'col-black', colorName: 'Чёрный', size: 'S', isActive: true },
                { colorId: 'col-white', colorName: 'Белый', size: 'M', isActive: true },
              ],
              images: [
                {
                  uiKey: 'b1',
                  colorId: 'col-black',
                  isMain: true,
                  sortOrder: 0,
                  source: { kind: 'local', clientMediaId: 'b1', file: new File([], 'b1.jpg'), previewUrl: 'blob:http://b1' },
                },
                {
                  uiKey: 'w1',
                  colorId: 'col-white',
                  isMain: false,
                  sortOrder: 1,
                  source: { kind: 'local', clientMediaId: 'w1', file: new File([], 'w1.jpg'), previewUrl: 'blob:http://w1' },
                },
                {
                  uiKey: 'b2',
                  colorId: 'col-black',
                  isMain: false,
                  sortOrder: 2,
                  source: { kind: 'local', clientMediaId: 'b2', file: new File([], 'b2.jpg'), previewUrl: 'blob:http://b2' },
                },
                {
                  uiKey: 'w2',
                  colorId: 'col-white',
                  isMain: false,
                  sortOrder: 3,
                  source: { kind: 'local', clientMediaId: 'w2', file: new File([], 'w2.jpg'), previewUrl: 'blob:http://w2' },
                },
                {
                  uiKey: 'b3',
                  colorId: 'col-black',
                  isMain: false,
                  sortOrder: 4,
                  source: { kind: 'local', clientMediaId: 'b3', file: new File([], 'b3.jpg'), previewUrl: 'blob:http://b3' },
                },
              ],
            }}
            canonicalColors={mockColors}
          >
            <ContextWatcher />
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // Black is selected initially (b1, b2, b3)
      const thumb0 = screen.getByTestId('thumbnail-item-0');
      const thumb2 = screen.getByTestId('thumbnail-item-2');
      expect(thumb0).toBeTruthy();
      expect(thumb2).toBeTruthy();

      // Drag b3 (index 2 in visible Black) to before b1 (index 0)
      fireEvent.dragStart(thumb2, {
        dataTransfer: {
          setData: vi.fn(),
          getData: () => '2',
          effectAllowed: 'move',
        },
      });
      fireEvent.dragOver(thumb0, {
        dataTransfer: {
          dropEffect: 'move',
        },
      });
      fireEvent.drop(thumb0, {
        dataTransfer: {
          getData: () => '2',
        },
      });
      fireEvent.dragEnd(thumb2);

      // Black visible order changed: b3, b1, b2
      await waitFor(() => {
        const blackImages = studioCtx!.draft.images!.filter((img) => img.colorId === 'col-black');
        expect(blackImages.map((img) => img.uiKey)).toEqual(['b3', 'b1', 'b2']);
      });
    });

    it('B. reorder Black does not modify White order', async () => {
      let studioCtx: ReturnType<typeof useProductStudio> | null = null;
      function ContextWatcher() {
        studioCtx = useProductStudio();
        return null;
      }

      render(
        <MemoryRouter>
          <ProductStudioProvider
            entryMode="create"
            initialDraft={{
              mediaMode: 'COLORWAY',
              colors: [
                { id: 'col-black', name: 'Чёрный', hex: '#000000' },
                { id: 'col-white', name: 'Белый', hex: '#ffffff' },
              ],
              variants: [
                { colorId: 'col-black', colorName: 'Чёрный', size: 'S', isActive: true },
                { colorId: 'col-white', colorName: 'Белый', size: 'M', isActive: true },
              ],
              images: [
                {
                  uiKey: 'b1',
                  colorId: 'col-black',
                  isMain: true,
                  sortOrder: 0,
                  source: { kind: 'local', clientMediaId: 'b1', file: new File([], 'b1.jpg'), previewUrl: 'blob:http://b1' },
                },
                {
                  uiKey: 'w1',
                  colorId: 'col-white',
                  isMain: false,
                  sortOrder: 1,
                  source: { kind: 'local', clientMediaId: 'w1', file: new File([], 'w1.jpg'), previewUrl: 'blob:http://w1' },
                },
                {
                  uiKey: 'b2',
                  colorId: 'col-black',
                  isMain: false,
                  sortOrder: 2,
                  source: { kind: 'local', clientMediaId: 'b2', file: new File([], 'b2.jpg'), previewUrl: 'blob:http://b2' },
                },
                {
                  uiKey: 'w2',
                  colorId: 'col-white',
                  isMain: false,
                  sortOrder: 3,
                  source: { kind: 'local', clientMediaId: 'w2', file: new File([], 'w2.jpg'), previewUrl: 'blob:http://w2' },
                },
                {
                  uiKey: 'b3',
                  colorId: 'col-black',
                  isMain: false,
                  sortOrder: 4,
                  source: { kind: 'local', clientMediaId: 'b3', file: new File([], 'b3.jpg'), previewUrl: 'blob:http://b3' },
                },
              ],
            }}
            canonicalColors={mockColors}
          >
            <ContextWatcher />
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // Drag b3 to index 0
      const thumb0 = screen.getByTestId('thumbnail-item-0');
      const thumb2 = screen.getByTestId('thumbnail-item-2');
      fireEvent.dragStart(thumb2, { dataTransfer: { setData: vi.fn(), getData: () => '2' } });
      fireEvent.dragOver(thumb0, { dataTransfer: {} });
      fireEvent.drop(thumb0, { dataTransfer: { getData: () => '2' } });
      fireEvent.dragEnd(thumb2);

      // Switch to White tab
      fireEvent.click(screen.getByTestId('colorway-tab-col-white'));

      // White thumbnails order must be w1, w2
      await waitFor(() => {
        const whiteImages = studioCtx!.draft.images!.filter((img) => img.colorId === 'col-white');
        expect(whiteImages.map((img) => img.uiKey)).toEqual(['w1', 'w2']);
      });

      // Switch back to Black tab
      fireEvent.click(screen.getByTestId('colorway-tab-col-black'));

      // Black order remains b3, b1, b2
      await waitFor(() => {
        const blackImages = studioCtx!.draft.images!.filter((img) => img.colorId === 'col-black');
        expect(blackImages.map((img) => img.uiKey)).toEqual(['b3', 'b1', 'b2']);
      });
    });

    it('C. reordering b3 to first position promotes b3 to global isMain automatically', async () => {
      let studioCtx: ReturnType<typeof useProductStudio> | null = null;
      function ContextWatcher() {
        studioCtx = useProductStudio();
        return null;
      }

      render(
        <MemoryRouter>
          <ProductStudioProvider
            entryMode="create"
            initialDraft={{
              mediaMode: 'COLORWAY',
              colors: [
                { id: 'col-black', name: 'Чёрный', hex: '#000000' },
                { id: 'col-white', name: 'Белый', hex: '#ffffff' },
              ],
              variants: [
                { colorId: 'col-black', colorName: 'Чёрный', size: 'S', isActive: true },
              ],
              images: [
                {
                  uiKey: 'b1',
                  colorId: 'col-black',
                  isMain: true,
                  sortOrder: 0,
                  source: { kind: 'local', clientMediaId: 'b1', file: new File([], 'b1.jpg'), previewUrl: 'blob:http://b1' },
                },
                {
                  uiKey: 'b2',
                  colorId: 'col-black',
                  isMain: false,
                  sortOrder: 1,
                  source: { kind: 'local', clientMediaId: 'b2', file: new File([], 'b2.jpg'), previewUrl: 'blob:http://b2' },
                },
                {
                  uiKey: 'b3',
                  colorId: 'col-black',
                  isMain: false,
                  sortOrder: 2,
                  source: { kind: 'local', clientMediaId: 'b3', file: new File([], 'b3.jpg'), previewUrl: 'blob:http://b3' },
                },
              ],
            }}
            canonicalColors={mockColors}
          >
            <ContextWatcher />
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // Drag b3 to index 0
      const thumb0 = screen.getByTestId('thumbnail-item-0');
      const thumb2 = screen.getByTestId('thumbnail-item-2');
      fireEvent.dragStart(thumb2, { dataTransfer: { setData: vi.fn(), getData: () => '2' } });
      fireEvent.dragOver(thumb0, { dataTransfer: {} });
      fireEvent.drop(thumb0, { dataTransfer: { getData: () => '2' } });
      fireEvent.dragEnd(thumb2);

      await waitFor(() => {
        const b1Img = studioCtx!.draft.images!.find((img) => img.uiKey === 'b1');
        const b3Img = studioCtx!.draft.images!.find((img) => img.uiKey === 'b3');
        expect(b3Img?.isMain).toBe(true);
        expect(b1Img?.isMain).toBe(false);
      });
    });
  });

  describe('Readiness Copy Canonical Presentation (Tests D through I)', () => {
    it('D. total 4 + Gray 0 => DOES NOT show "Нужно минимум 3 фото", DOES show "Добавьте фото для цвета «Серый»"', async () => {
      render(
        <MemoryRouter>
          <ProductStudioProvider
            entryMode="create"
            initialDraft={{
              mediaMode: 'COLORWAY',
              colors: [
                { id: 'col-black', name: 'Чёрный', hex: '#000000' },
                { id: 'col-white', name: 'Белый', hex: '#ffffff' },
                { id: 'col-gray', name: 'Серый', hex: '#808080' },
              ],
              variants: [
                { colorId: 'col-black', colorName: 'Чёрный', size: 'S', isActive: true },
                { colorId: 'col-white', colorName: 'Белый', size: 'M', isActive: true },
                { colorId: 'col-gray', colorName: 'Серый', size: 'L', isActive: true },
              ],
              images: [
                { uiKey: 'b1', colorId: 'col-black', isMain: true, sortOrder: 0, source: { kind: 'local', clientMediaId: 'b1', file: new File([], 'b1.jpg'), previewUrl: 'blob:http://b1' } },
                { uiKey: 'b2', colorId: 'col-black', isMain: false, sortOrder: 1, source: { kind: 'local', clientMediaId: 'b2', file: new File([], 'b2.jpg'), previewUrl: 'blob:http://b2' } },
                { uiKey: 'b3', colorId: 'col-black', isMain: false, sortOrder: 2, source: { kind: 'local', clientMediaId: 'b3', file: new File([], 'b3.jpg'), previewUrl: 'blob:http://b3' } },
                { uiKey: 'w1', colorId: 'col-white', isMain: false, sortOrder: 3, source: { kind: 'local', clientMediaId: 'w1', file: new File([], 'w1.jpg'), previewUrl: 'blob:http://w1' } },
              ],
            }}
            canonicalColors={mockColors}
          >
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // Select Gray tab (which has 0 photos)
      fireEvent.click(screen.getByTestId('colorway-tab-col-gray'));

      // Progress text shows 4 of 3 ready
      expect(screen.getByTestId('media-progress-badge').textContent).toBe('Фото 4 из 3 · готово');

      // Helper does NOT show "Нужно минимум 3 фото"
      const helper = screen.getByTestId('media-required-helper');
      expect(helper.textContent).not.toContain('Нужно минимум 3 фото');
      expect(helper.textContent).toBe('Добавьте фото для цвета «Серый»');
    });

    it('E. total 2 => shows "Нужно минимум 3 фото"', async () => {
      // 1. Canonical readiness function check (Case A: Black 1, White 1)
      const draft = {
        mediaMode: 'COLORWAY' as const,
        colors: [
          { id: 'col-black', name: 'Чёрный', hex: '#000000' },
          { id: 'col-white', name: 'Белый', hex: '#ffffff' },
        ],
        variants: [
          { colorId: 'col-black', colorName: 'Чёрный', size: 'S', isActive: true },
          { colorId: 'col-white', colorName: 'Белый', size: 'M', isActive: true },
        ],
        images: [
          { uiKey: 'b1', colorId: 'col-black', isMain: true, sortOrder: 0 },
          { uiKey: 'w1', colorId: 'col-white', isMain: false, sortOrder: 1 },
        ],
      };

      const warning = getMediaReadinessWarning(draft);
      expect(warning).toBe('Нужно минимум 3 фото');

      // 2. Visual Workspace UI test: total 2 (Black 2, Gray 0) with Gray selected
      render(
        <MemoryRouter>
          <ProductStudioProvider
            entryMode="create"
            initialDraft={{
              mediaMode: 'COLORWAY',
              colors: [
                { id: 'col-black', name: 'Чёрный', hex: '#000000' },
                { id: 'col-gray', name: 'Серый', hex: '#808080' },
              ],
              variants: [
                { colorId: 'col-black', colorName: 'Чёрный', size: 'S', isActive: true },
                { colorId: 'col-gray', colorName: 'Серый', size: 'M', isActive: true },
              ],
              images: [
                { uiKey: 'b1', colorId: 'col-black', isMain: true, sortOrder: 0, source: { kind: 'local', clientMediaId: 'b1', file: new File([], 'b1.jpg'), previewUrl: 'blob:http://b1' } },
                { uiKey: 'b2', colorId: 'col-black', isMain: false, sortOrder: 1, source: { kind: 'local', clientMediaId: 'b2', file: new File([], 'b2.jpg'), previewUrl: 'blob:http://b2' } },
              ],
            }}
            canonicalColors={mockColors}
          >
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // Select Gray tab (0 photos)
      fireEvent.click(screen.getByTestId('colorway-tab-col-gray'));

      // In total < 3, Case A takes precedence: shows "Нужно минимум 3 фото"
      const helper = screen.getByTestId('media-required-helper');
      expect(helper.textContent).toBe('Нужно минимум 3 фото');

      // Progress text shows 2 of 3 (NOT "готово")
      const progressBadge = screen.getByTestId('media-progress-badge');
      expect(progressBadge.textContent).toBe('Фото 2 из 3');
      expect(progressBadge.textContent).not.toContain('готово');
    });

    it('F. total >=3 + two missing colors => message identifies missing colors', () => {
      const draft = {
        mediaMode: 'COLORWAY' as const,
        colors: [
          { id: 'col-black', name: 'Чёрный', hex: '#000000' },
          { id: 'col-white', name: 'Белый', hex: '#ffffff' },
          { id: 'col-gray', name: 'Серый', hex: '#808080' },
        ],
        variants: [
          { colorId: 'col-black', colorName: 'Чёрный', size: 'S', isActive: true },
          { colorId: 'col-white', colorName: 'Белый', size: 'M', isActive: true },
          { colorId: 'col-gray', colorName: 'Серый', size: 'L', isActive: true },
        ],
        images: [
          { uiKey: 'b1', colorId: 'col-black', isMain: true, sortOrder: 0 },
          { uiKey: 'b2', colorId: 'col-black', isMain: false, sortOrder: 1 },
          { uiKey: 'b3', colorId: 'col-black', isMain: false, sortOrder: 2 },
        ],
      };

      const warning = getMediaReadinessWarning(draft);
      expect(warning).toBe('Добавьте фото для цветов: Белый, Серый');
    });

    it('G. all color coverage satisfied => no readiness warning', () => {
      const draft = {
        mediaMode: 'COLORWAY' as const,
        colors: [
          { id: 'col-black', name: 'Чёрный', hex: '#000000' },
          { id: 'col-white', name: 'Белый', hex: '#ffffff' },
        ],
        variants: [
          { colorId: 'col-black', colorName: 'Чёрный', size: 'S', isActive: true },
          { colorId: 'col-white', colorName: 'Белый', size: 'M', isActive: true },
        ],
        images: [
          { uiKey: 'b1', colorId: 'col-black', isMain: true, sortOrder: 0 },
          { uiKey: 'b2', colorId: 'col-black', isMain: false, sortOrder: 1 },
          { uiKey: 'w1', colorId: 'col-white', isMain: false, sortOrder: 2 },
        ],
      };

      const warning = getMediaReadinessWarning(draft);
      expect(warning).toBeNull();
    });

    it('H. GENERAL total <3 => still shows minimum 3 warning', () => {
      const draft = {
        mediaMode: 'GENERAL' as const,
        images: [
          { uiKey: 'g1', colorId: null, isMain: true, sortOrder: 0 },
          { uiKey: 'g2', colorId: null, isMain: false, sortOrder: 1 },
        ],
      };

      const warning = getMediaReadinessWarning(draft);
      expect(warning).toBe('Нужно минимум 3 фото');
    });

    it('I. GENERAL ignores color coverage', () => {
      const draft = {
        mediaMode: 'GENERAL' as const,
        colors: [
          { id: 'col-black', name: 'Чёрный', hex: '#000000' },
          { id: 'col-white', name: 'Белый', hex: '#ffffff' },
        ],
        variants: [
          { colorId: 'col-black', colorName: 'Чёрный', size: 'S', isActive: true },
          { colorId: 'col-white', colorName: 'Белый', size: 'M', isActive: true },
        ],
        images: [
          { uiKey: 'g1', colorId: null, isMain: true, sortOrder: 0 },
          { uiKey: 'g2', colorId: null, isMain: false, sortOrder: 1 },
          { uiKey: 'g3', colorId: null, isMain: false, sortOrder: 2 },
        ],
      };

      const warning = getMediaReadinessWarning(draft);
      expect(warning).toBeNull();
    });
  });
});
