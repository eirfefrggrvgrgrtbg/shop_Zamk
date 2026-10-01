/* @vitest-environment jsdom */
import { vi, describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ProductStudioVisualWorkspace } from './ProductStudioVisualWorkspace';
import { ProductStudioFormWorkspace } from './ProductStudioFormWorkspace';
import {
  ProductStudioProvider,
  useProductStudio,
} from '../../contexts/ProductStudioContext';
import {
  resolveInitialMediaColorId,
  reorderProductStudioImages,
} from './productStudioMediaHelper';
import type { SellerColor } from '@zamk/api-client';

const mockColors: SellerColor[] = [
  { id: 'col-black', nameRu: 'Чёрный', code: 'black', hex: '#000000' },
  { id: 'col-white', nameRu: 'Белый', code: 'white', hex: '#ffffff' },
  { id: 'col-red', nameRu: 'Красный', code: 'red', hex: '#ff0000' },
];

vi.mock('@zamk/api-client/src/seller', async (importOriginal: any) => {
  const actual = await importOriginal();
  return {
    ...actual,
    getSellerColors: vi.fn().mockImplementation(() => Promise.resolve(mockColors)),
    getSellerCategories: vi.fn().mockImplementation(() => Promise.resolve([])),
    getSellerCategorySchema: vi.fn().mockImplementation(() => Promise.resolve(null)),
    getSellerSizeValues: vi.fn().mockImplementation(() => Promise.resolve([])),
  };
});

let objectUrlCounter = 0;
beforeEach(() => {
  objectUrlCounter = 0;
  globalThis.URL.createObjectURL = vi.fn((_file: any) => {
    objectUrlCounter++;
    return `blob:http://localhost/test-blob-${objectUrlCounter}`;
  });
  globalThis.URL.revokeObjectURL = vi.fn();
});

describe('SELLER MEDIA.2C1 — Selection & Reorder Hardening', () => {
  describe('A. main image White + no explicit media selection => White selected initially', () => {
    it('selects White initially when White is global main and seller made no explicit selection', () => {
      const colors = [
        { id: 'col-black', name: 'Чёрный', hex: '#000000' },
        { id: 'col-white', name: 'Белый', hex: '#ffffff' },
      ];
      const images: any[] = [
        { uiKey: 'img-b1', colorId: 'col-black', isMain: false, sortOrder: 0 },
        { uiKey: 'img-w1', colorId: 'col-white', isMain: true, sortOrder: 1 },
      ];

      const resolved = resolveInitialMediaColorId({
        selectedMediaColorId: null,
        selectedPreviewColorId: null,
        images,
        colors,
        mediaMode: 'COLORWAY',
      });

      expect(resolved).toBe('col-white');

      // Also verify in Visual Workspace component
      render(
        <MemoryRouter>
          <ProductStudioProvider
            entryMode="edit"
            initialDraft={{
              id: 'prod-init-test',
              title: 'Блузка',
              mediaMode: 'COLORWAY',
              colors,
              variants: [
                { colorId: 'col-black', colorName: 'Чёрный', size: 'S', isActive: true },
                { colorId: 'col-white', colorName: 'Белый', size: 'M', isActive: true },
              ],
              images: images.map((img) => ({
                ...img,
                source: { kind: 'local', clientMediaId: img.uiKey, file: new File([], 'f.jpg'), previewUrl: 'blob:http://test/' + img.uiKey },
              })),
            }}
            canonicalColors={mockColors}
          >
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      const whiteTab = screen.getByTestId('colorway-tab-col-white');
      expect(whiteTab.getAttribute('aria-selected')).toBe('true');
    });
  });

  describe('B. selected product color Black overrides main White => Black selected', () => {
    it('uses selected preview color when present, overriding main photo color', () => {
      const colors = [
        { id: 'col-black', name: 'Чёрный', hex: '#000000' },
        { id: 'col-white', name: 'Белый', hex: '#ffffff' },
      ];
      const images: any[] = [
        { uiKey: 'img-b1', colorId: 'col-black', isMain: false, sortOrder: 0 },
        { uiKey: 'img-w1', colorId: 'col-white', isMain: true, sortOrder: 1 },
      ];

      const resolved = resolveInitialMediaColorId({
        selectedMediaColorId: null,
        selectedPreviewColorId: 'col-black',
        images,
        colors,
        mediaMode: 'COLORWAY',
      });

      expect(resolved).toBe('col-black');
    });
  });

  describe('C. explicit valid media selection preserved', () => {
    it('preserves explicit selectedMediaColorId even if preview color or main image differs', () => {
      const colors = [
        { id: 'col-black', name: 'Чёрный', hex: '#000000' },
        { id: 'col-white', name: 'Белый', hex: '#ffffff' },
      ];
      const images: any[] = [
        { uiKey: 'img-b1', colorId: 'col-black', isMain: true, sortOrder: 0 },
        { uiKey: 'img-w1', colorId: 'col-white', isMain: false, sortOrder: 1 },
      ];

      const resolved = resolveInitialMediaColorId({
        selectedMediaColorId: 'col-white',
        selectedPreviewColorId: 'col-black',
        images,
        colors,
        mediaMode: 'COLORWAY',
      });

      expect(resolved).toBe('col-white');
    });
  });

  describe('D & E. Visual <-> Form Selection Continuity', () => {
    it('D. preserves White selection when switching from Visual to Form', async () => {
      function Switcher() {
        const { viewMode, setViewMode } = useProductStudio();
        return (
          <div>
            <button data-testid="toggle-view" onClick={() => setViewMode(viewMode === 'visual' ? 'form' : 'visual')}>
              Toggle
            </button>
            {viewMode === 'visual' ? <ProductStudioVisualWorkspace /> : <ProductStudioFormWorkspace />}
          </div>
        );
      }

      const colors = [
        { id: 'col-black', name: 'Чёрный', hex: '#000000' },
        { id: 'col-white', name: 'Белый', hex: '#ffffff' },
      ];

      render(
        <MemoryRouter>
          <ProductStudioProvider
            entryMode="create"
            initialDraft={{
              mediaMode: 'COLORWAY',
              colors,
              variants: [
                { colorId: 'col-black', colorName: 'Чёрный', size: 'S', isActive: true },
                { colorId: 'col-white', colorName: 'Белый', size: 'M', isActive: true },
              ],
              images: [
                {
                  uiKey: 'img-b1',
                  colorId: 'col-black',
                  isMain: true,
                  sortOrder: 0,
                  source: { kind: 'local', clientMediaId: 'img-b1', file: new File([], 'b.jpg'), previewUrl: 'blob:http://b' },
                },
                {
                  uiKey: 'img-w1',
                  colorId: 'col-white',
                  isMain: false,
                  sortOrder: 1,
                  source: { kind: 'local', clientMediaId: 'img-w1', file: new File([], 'w.jpg'), previewUrl: 'blob:http://w' },
                },
              ],
            }}
            canonicalColors={mockColors}
          >
            <Switcher />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // In Visual workspace, click White tab
      const visualWhiteTab = screen.getByTestId('colorway-tab-col-white');
      fireEvent.click(visualWhiteTab);
      expect(visualWhiteTab.getAttribute('aria-selected')).toBe('true');

      // Switch to Form workspace
      fireEvent.click(screen.getByTestId('toggle-view'));
      fireEvent.click(screen.getByTestId('studio-section-btn-media'));

      // In Form workspace, White tab must be selected!
      await waitFor(() => {
        const formWhiteTab = screen.getByTestId('form-colorway-tab-col-white');
        expect(formWhiteTab.className).toContain('bg-gray-900');
      });
    });

    it('E. preserves Black selection when switching from Form to Visual', async () => {
      function Switcher() {
        const { viewMode, setViewMode } = useProductStudio();
        return (
          <div>
            <button data-testid="toggle-view" onClick={() => setViewMode(viewMode === 'visual' ? 'form' : 'visual')}>
              Toggle
            </button>
            {viewMode === 'visual' ? <ProductStudioVisualWorkspace /> : <ProductStudioFormWorkspace />}
          </div>
        );
      }

      const colors = [
        { id: 'col-black', name: 'Чёрный', hex: '#000000' },
        { id: 'col-white', name: 'Белый', hex: '#ffffff' },
      ];

      render(
        <MemoryRouter>
          <ProductStudioProvider
            entryMode="create"
            initialDraft={{
              mediaMode: 'COLORWAY',
              colors,
              variants: [
                { colorId: 'col-black', colorName: 'Чёрный', size: 'S', isActive: true },
                { colorId: 'col-white', colorName: 'Белый', size: 'M', isActive: true },
              ],
              images: [
                {
                  uiKey: 'img-w1',
                  colorId: 'col-white',
                  isMain: true,
                  sortOrder: 0,
                  source: { kind: 'local', clientMediaId: 'img-w1', file: new File([], 'w.jpg'), previewUrl: 'blob:http://w' },
                },
                {
                  uiKey: 'img-b1',
                  colorId: 'col-black',
                  isMain: false,
                  sortOrder: 1,
                  source: { kind: 'local', clientMediaId: 'img-b1', file: new File([], 'b.jpg'), previewUrl: 'blob:http://b' },
                },
              ],
            }}
            canonicalColors={mockColors}
          >
            <Switcher />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // Start in Form workspace
      fireEvent.click(screen.getByTestId('toggle-view'));
      fireEvent.click(screen.getByTestId('studio-section-btn-media'));

      // Click Black tab in Form workspace
      await waitFor(() => {
        expect(screen.getByTestId('form-colorway-tab-col-black')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('form-colorway-tab-col-black'));

      // Switch back to Visual workspace
      fireEvent.click(screen.getByTestId('toggle-view'));

      // In Visual workspace, Black tab must be selected!
      const visualBlackTab = screen.getByTestId('colorway-tab-col-black');
      expect(visualBlackTab.getAttribute('aria-selected')).toBe('true');
    });
  });

  describe('F, G, H. Reorder within Filtered Color Gallery', () => {
    const interleavedDraftImages: any[] = [
      { uiKey: 'Black-1', colorId: 'col-black', isMain: false, sortOrder: 0 },
      { uiKey: 'White-1', colorId: 'col-white', isMain: true, sortOrder: 1 },
      { uiKey: 'Black-2', colorId: 'col-black', isMain: false, sortOrder: 2 },
      { uiKey: 'White-2', colorId: 'col-white', isMain: false, sortOrder: 3 },
    ];

    it('F. reorders Black in interleaved global image array without affecting White order', () => {
      const reordered = reorderProductStudioImages(
        interleavedDraftImages,
        0, // move Black-1 from index 0 of Black gallery
        1, // to index 1 of Black gallery
        'col-black',
        'COLORWAY'
      );

      // Expected final order: Black-2, White-1, Black-1, White-2
      expect(reordered.map((img) => img.uiKey)).toEqual([
        'Black-2',
        'White-1',
        'Black-1',
        'White-2',
      ]);
      expect(reordered.map((img) => img.sortOrder)).toEqual([0, 1, 2, 3]);

      // White images remain in same relative order
      const whiteKeys = reordered.filter((img) => img.colorId === 'col-white').map((img) => img.uiKey);
      expect(whiteKeys).toEqual(['White-1', 'White-2']);

      // No image changed color
      expect(reordered[0].colorId).toBe('col-black');
      expect(reordered[1].colorId).toBe('col-white');
      expect(reordered[2].colorId).toBe('col-black');
      expect(reordered[3].colorId).toBe('col-white');
    });

    it('G. reorders White afterwards, leaving Black relative order untouched', () => {
      const step1 = reorderProductStudioImages(
        interleavedDraftImages,
        0,
        1,
        'col-black',
        'COLORWAY'
      );
      // step1 is [Black-2, White-1, Black-1, White-2]

      // Now switch to White and swap White-1 / White-2
      const step2 = reorderProductStudioImages(
        step1,
        0, // move White-1 from index 0 of White gallery
        1, // to index 1 of White gallery
        'col-white',
        'COLORWAY'
      );

      // Expected: Black-2, White-2, Black-1, White-1
      expect(step2.map((img) => img.uiKey)).toEqual([
        'Black-2',
        'White-2',
        'Black-1',
        'White-1',
      ]);
      expect(step2.map((img) => img.sortOrder)).toEqual([0, 1, 2, 3]);

      // Black relative order must remain untouched: Black-2 before Black-1
      const blackKeys = step2.filter((img) => img.colorId === 'col-black').map((img) => img.uiKey);
      expect(blackKeys).toEqual(['Black-2', 'Black-1']);
    });

    it('H. reordering first gallery updates global isMain to new first photo; secondary gallery reorder preserves it', () => {
      // Reordering Black (first canonical gallery): Black-2 becomes first photo, so Black-2 becomes global isMain
      const step1 = reorderProductStudioImages(
        interleavedDraftImages,
        0,
        1,
        'col-black',
        'COLORWAY'
      );

      const main1 = step1.find((img) => img.isMain);
      expect(main1?.uiKey).toBe('Black-2');

      // Reordering White (secondary gallery): White-2 becomes first White photo, but Black-2 remains global isMain
      const step2 = reorderProductStudioImages(
        step1,
        0,
        1,
        'col-white',
        'COLORWAY'
      );

      const main2 = step2.find((img) => img.isMain);
      expect(main2?.uiKey).toBe('Black-2');
    });
  });

  describe('I & J. Delete Regression', () => {
    it('I. deleting last Black photo keeps Black tab with count 0 and shows empty state', async () => {
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
              ],
            }}
            canonicalColors={mockColors}
          >
            <ContextWatcher />
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // Verify Black has 1 photo
      expect(screen.getByTestId('colorway-tab-col-black').textContent).toContain('· 1');

      // Delete the Black photo
      const deleteBtn = screen.getByTestId('thumbnail-delete-btn-0');
      fireEvent.click(deleteBtn);

      // Tab remains and count becomes 0
      await waitFor(() => {
        expect(screen.getByTestId('colorway-tab-col-black').textContent).toContain('· 0');
      });

      // Empty state appears for Black
      const emptySlot = screen.getByTestId('presentation-empty-media-slot');
      expect(emptySlot).toBeTruthy();
      expect(emptySlot.textContent).toContain('Для цвета «Чёрный» пока нет фотографий');
      expect(screen.getByTestId('empty-stage-add-btn')).toBeTruthy();

      // White gallery untouched in draft
      expect(studioCtx!.draft.images).toHaveLength(1);
      expect(studioCtx!.draft.images![0].uiKey).toBe('w1');
      expect(studioCtx!.draft.images![0].colorId).toBe('col-white');
    });

    it('J. deleting Black photo in Form workspace does not affect White photos', async () => {
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
                  uiKey: 'w2',
                  colorId: 'col-white',
                  isMain: false,
                  sortOrder: 2,
                  source: { kind: 'local', clientMediaId: 'w2', file: new File([], 'w2.jpg'), previewUrl: 'blob:http://w2' },
                },
              ],
            }}
            canonicalColors={mockColors}
          >
            <ContextWatcher />
            <ProductStudioFormWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      fireEvent.click(screen.getByTestId('studio-section-btn-media'));

      // Black tab is active (count 1)
      await waitFor(() => {
        expect(screen.getByTestId('form-colorway-tab-col-black').textContent).toContain('· 1');
      });

      // Delete the only Black photo via Form delete button
      const deleteBtn = screen.getByTestId('form-media-delete-btn-0');
      fireEvent.click(deleteBtn);

      await waitFor(() => {
        expect(screen.getByTestId('form-colorway-tab-col-black').textContent).toContain('· 0');
      });

      // White tab still has 2 photos
      expect(screen.getByTestId('form-colorway-tab-col-white').textContent).toContain('· 2');

      // Draft images has both white photos intact
      expect(studioCtx!.draft.images).toHaveLength(2);
      expect(studioCtx!.draft.images!.every((img) => img.colorId === 'col-white')).toBe(true);
    });
  });
});
