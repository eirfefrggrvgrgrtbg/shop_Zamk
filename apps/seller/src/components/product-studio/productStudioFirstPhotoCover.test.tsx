/* @vitest-environment jsdom */
import { vi, describe, it, expect } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ProductStudio } from './ProductStudio';
import { ProductStudioVisualWorkspace } from './ProductStudioVisualWorkspace';
import {
  ProductStudioProvider,
  useProductStudio,
  ProductStudioDraft,
} from '../../contexts/ProductStudioContext';

const { mockColors } = vi.hoisted(() => ({
  mockColors: [
    { id: 'col-black', nameRu: 'Чёрный', code: 'black', hex: '#000000' },
    { id: 'col-white', nameRu: 'Белый', code: 'white', hex: '#ffffff' },
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

function createMockDataTransfer(fromIndex?: number) {
  let stored = fromIndex !== undefined ? String(fromIndex) : '';
  return {
    setData: (_format: string, val: string) => {
      stored = val;
    },
    getData: (_format?: string) => stored,
    effectAllowed: 'move',
    dropEffect: 'move',
  };
}

function dragVisualThumbnail(fromIndex: number, toIndex: number) {
  const fromEl = screen.getByTestId(`thumbnail-item-${fromIndex}`);
  const toEl = screen.getByTestId(`thumbnail-item-${toIndex}`);
  const dt = createMockDataTransfer(fromIndex);
  fireEvent.dragStart(fromEl, { dataTransfer: dt });
  fireEvent.dragOver(toEl, { dataTransfer: dt });
  fireEvent.drop(toEl, { dataTransfer: dt });
  fireEvent.dragEnd(fromEl);
}

function getThumbnailImgSrc(index: number): string | null {
  const thumb = screen.getByTestId(`pdp-thumbnail-${index}`);
  return thumb.querySelector('img')?.getAttribute('src') ?? null;
}

describe('SELLER MEDIA.2C3 — First Photo Is Cover', () => {
  describe('GENERAL Mode Tests (Cases A & B)', () => {
    it('Case A: GENERAL [A, B, C] -> slider first is A, thumbnail[0] is A, A isMain=true, B/C isMain=false', () => {
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
              mediaMode: 'GENERAL',
              images: [
                {
                  uiKey: 'img-A',
                  source: { kind: 'canonical', imageId: 'img-A', url: 'https://example.com/A.jpg' },
                  isMain: true,
                  sortOrder: 0,
                  colorId: null,
                },
                {
                  uiKey: 'img-B',
                  source: { kind: 'canonical', imageId: 'img-B', url: 'https://example.com/B.jpg' },
                  isMain: false,
                  sortOrder: 1,
                  colorId: null,
                },
                {
                  uiKey: 'img-C',
                  source: { kind: 'canonical', imageId: 'img-C', url: 'https://example.com/C.jpg' },
                  isMain: false,
                  sortOrder: 2,
                  colorId: null,
                },
              ],
            }}
          >
            <ContextWatcher />
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // Slider first image is A
      const mainImage = screen.getByTestId('main-product-image');
      expect(mainImage.getAttribute('src')).toContain('A.jpg');

      // Thumbnail[0] is A
      expect(getThumbnailImgSrc(0)).toContain('A.jpg');

      // A has isMain = true, B and C have isMain = false
      const images = studioCtx!.draft.images!;
      expect(images[0].uiKey).toBe('img-A');
      expect(images[0].isMain).toBe(true);
      expect(images[1].uiKey).toBe('img-B');
      expect(images[1].isMain).toBe(false);
      expect(images[2].uiKey).toBe('img-C');
      expect(images[2].isMain).toBe(false);

      // Case K check: exactly one isMain
      expect(images.filter((img) => img.isMain)).toHaveLength(1);
    });

    it('Case B: GENERAL reorder C to first -> slider [C, A, B], thumbnail [C, A, B], C isMain=true, exactly one isMain', async () => {
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
              mediaMode: 'GENERAL',
              images: [
                {
                  uiKey: 'img-A',
                  source: { kind: 'canonical', imageId: 'img-A', url: 'https://example.com/A.jpg' },
                  isMain: true,
                  sortOrder: 0,
                  colorId: null,
                },
                {
                  uiKey: 'img-B',
                  source: { kind: 'canonical', imageId: 'img-B', url: 'https://example.com/B.jpg' },
                  isMain: false,
                  sortOrder: 1,
                  colorId: null,
                },
                {
                  uiKey: 'img-C',
                  source: { kind: 'canonical', imageId: 'img-C', url: 'https://example.com/C.jpg' },
                  isMain: false,
                  sortOrder: 2,
                  colorId: null,
                },
              ],
            }}
          >
            <ContextWatcher />
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // Drag thumbnail 2 (C) to index 0
      dragVisualThumbnail(2, 0);

      await waitFor(() => {
        const images = studioCtx!.draft.images!;
        expect(images.map((img) => img.uiKey)).toEqual(['img-C', 'img-A', 'img-B']);
      });

      // Thumbnail order: [C, A, B]
      expect(getThumbnailImgSrc(0)).toContain('C.jpg');
      expect(getThumbnailImgSrc(1)).toContain('A.jpg');
      expect(getThumbnailImgSrc(2)).toContain('B.jpg');

      // Slider order: first slide is C
      const mainImage = screen.getByTestId('main-product-image');
      expect(mainImage.getAttribute('src')).toContain('C.jpg');

      // C has isMain = true, A and B have isMain = false
      const images = studioCtx!.draft.images!;
      expect(images[0].uiKey).toBe('img-C');
      expect(images[0].isMain).toBe(true);
      expect(images[1].uiKey).toBe('img-A');
      expect(images[1].isMain).toBe(false);
      expect(images[2].uiKey).toBe('img-B');
      expect(images[2].isMain).toBe(false);

      // Exactly one isMain exists
      expect(images.filter((img) => img.isMain)).toHaveLength(1);
    });
  });

  describe('COLORWAY Mode Tests (Cases C, D, E, F, G, H, I)', () => {
    function setupColorwayDraft(): ProductStudioDraft {
      return {
        title: 'Test Product',
        description: 'Test Description',
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
            source: { kind: 'canonical', imageId: 'b1', url: 'https://example.com/b1.jpg' },
          },
          {
            uiKey: 'b2',
            colorId: 'col-black',
            isMain: false,
            sortOrder: 1,
            source: { kind: 'canonical', imageId: 'b2', url: 'https://example.com/b2.jpg' },
          },
          {
            uiKey: 'b3',
            colorId: 'col-black',
            isMain: false,
            sortOrder: 2,
            source: { kind: 'canonical', imageId: 'b3', url: 'https://example.com/b3.jpg' },
          },
          {
            uiKey: 'w1',
            colorId: 'col-white',
            isMain: false,
            sortOrder: 3,
            source: { kind: 'canonical', imageId: 'w1', url: 'https://example.com/w1.jpg' },
          },
          {
            uiKey: 'w2',
            colorId: 'col-white',
            isMain: false,
            sortOrder: 4,
            source: { kind: 'canonical', imageId: 'w2', url: 'https://example.com/w2.jpg' },
          },
        ],
      };
    }

    it('Case C: COLORWAY Black [B1, B2, B3] -> Black slider matches thumbnails, first slide is B1, thumbnail[0] is B1', () => {
      let studioCtx: ReturnType<typeof useProductStudio> | null = null;
      function ContextWatcher() {
        studioCtx = useProductStudio();
        return null;
      }

      render(
        <MemoryRouter>
          <ProductStudioProvider entryMode="create" initialDraft={setupColorwayDraft()} canonicalColors={mockColors}>
            <ContextWatcher />
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // Black thumbnails
      expect(getThumbnailImgSrc(0)).toContain('b1.jpg');
      expect(getThumbnailImgSrc(1)).toContain('b2.jpg');
      expect(getThumbnailImgSrc(2)).toContain('b3.jpg');

      // First slide is B1
      expect(screen.getByTestId('main-product-image').getAttribute('src')).toContain('b1.jpg');

      // Exactly one isMain
      expect(studioCtx!.draft.images!.filter((img) => img.isMain)).toHaveLength(1);
    });

    it('Case D & E: COLORWAY reorder Black B3 first -> Black slider and thumbnails [B3, B1, B2], B3 becomes global isMain', async () => {
      let studioCtx: ReturnType<typeof useProductStudio> | null = null;
      function ContextWatcher() {
        studioCtx = useProductStudio();
        return null;
      }

      render(
        <MemoryRouter>
          <ProductStudioProvider entryMode="create" initialDraft={setupColorwayDraft()} canonicalColors={mockColors}>
            <ContextWatcher />
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // Drag Black B3 (index 2) to before B1 (index 0)
      dragVisualThumbnail(2, 0);

      await waitFor(() => {
        const blackImgs = studioCtx!.draft.images!.filter((img) => img.colorId === 'col-black');
        expect(blackImgs.map((img) => img.uiKey)).toEqual(['b3', 'b1', 'b2']);
      });

      // Black slider: first slide is B3
      expect(screen.getByTestId('main-product-image').getAttribute('src')).toContain('b3.jpg');

      // Black thumbnails: [B3, B1, B2]
      expect(getThumbnailImgSrc(0)).toContain('b3.jpg');
      expect(getThumbnailImgSrc(1)).toContain('b1.jpg');
      expect(getThumbnailImgSrc(2)).toContain('b2.jpg');

      // Case E: B3 becomes global isMain = true automatically; other images have isMain = false
      const allImages = studioCtx!.draft.images!;
      const b3 = allImages.find((img) => img.uiKey === 'b3');
      const b1 = allImages.find((img) => img.uiKey === 'b1');
      const b2 = allImages.find((img) => img.uiKey === 'b2');
      const w1 = allImages.find((img) => img.uiKey === 'w1');
      const w2 = allImages.find((img) => img.uiKey === 'w2');

      expect(b3?.isMain).toBe(true);
      expect(b1?.isMain).toBe(false);
      expect(b2?.isMain).toBe(false);
      expect(w1?.isMain).toBe(false);
      expect(w2?.isMain).toBe(false);

      // Exactly one isMain exists globally
      expect(allImages.filter((img) => img.isMain)).toHaveLength(1);
    });

    it('Case F: COLORWAY reorder White -> White first is W2, Black first (B3) remains global isMain', async () => {
      let studioCtx: ReturnType<typeof useProductStudio> | null = null;
      function ContextWatcher() {
        studioCtx = useProductStudio();
        return null;
      }

      // Initial draft where Black has B3 as first
      const draft = setupColorwayDraft();
      draft.images = [
        {
          uiKey: 'b3',
          colorId: 'col-black',
          isMain: true,
          sortOrder: 0,
          source: { kind: 'canonical', imageId: 'b3', url: 'https://example.com/b3.jpg' },
        },
        {
          uiKey: 'b1',
          colorId: 'col-black',
          isMain: false,
          sortOrder: 1,
          source: { kind: 'canonical', imageId: 'b1', url: 'https://example.com/b1.jpg' },
        },
        {
          uiKey: 'b2',
          colorId: 'col-black',
          isMain: false,
          sortOrder: 2,
          source: { kind: 'canonical', imageId: 'b2', url: 'https://example.com/b2.jpg' },
        },
        {
          uiKey: 'w1',
          colorId: 'col-white',
          isMain: false,
          sortOrder: 3,
          source: { kind: 'canonical', imageId: 'w1', url: 'https://example.com/w1.jpg' },
        },
        {
          uiKey: 'w2',
          colorId: 'col-white',
          isMain: false,
          sortOrder: 4,
          source: { kind: 'canonical', imageId: 'w2', url: 'https://example.com/w2.jpg' },
        },
      ];

      render(
        <MemoryRouter>
          <ProductStudioProvider entryMode="create" initialDraft={draft} canonicalColors={mockColors}>
            <ContextWatcher />
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // Switch to White gallery
      fireEvent.click(screen.getByTestId('color-swatch-col-white'));

      await waitFor(() => {
        expect(getThumbnailImgSrc(0)).toContain('w1.jpg');
      });

      // White thumbnails: [W1, W2]. Reorder W2 (index 1) to index 0
      dragVisualThumbnail(1, 0);

      await waitFor(() => {
        const whiteImgs = studioCtx!.draft.images!.filter((img) => img.colorId === 'col-white');
        expect(whiteImgs.map((img) => img.uiKey)).toEqual(['w2', 'w1']);
      });

      // White thumbnails are now [W2, W1]
      expect(getThumbnailImgSrc(0)).toContain('w2.jpg');
      expect(getThumbnailImgSrc(1)).toContain('w1.jpg');

      // Black's first image (B3) remains global isMain = true!
      // White's first image (W2) does NOT steal global isMain!
      const allImages = studioCtx!.draft.images!;
      const b3 = allImages.find((img) => img.uiKey === 'b3');
      const w2 = allImages.find((img) => img.uiKey === 'w2');

      expect(b3?.isMain).toBe(true);
      expect(w2?.isMain).toBe(false);

      // Exactly one isMain exists
      expect(allImages.filter((img) => img.isMain)).toHaveLength(1);
    });

    it('Case G: Delete first image (B3) -> next Black image (B1) becomes first slide and global isMain', async () => {
      let studioCtx: ReturnType<typeof useProductStudio> | null = null;
      function ContextWatcher() {
        studioCtx = useProductStudio();
        return null;
      }

      const draft = setupColorwayDraft();
      // Black: [B3 (isMain), B1, B2]
      draft.images = [
        {
          uiKey: 'b3',
          colorId: 'col-black',
          isMain: true,
          sortOrder: 0,
          source: { kind: 'canonical', imageId: 'b3', url: 'https://example.com/b3.jpg' },
        },
        {
          uiKey: 'b1',
          colorId: 'col-black',
          isMain: false,
          sortOrder: 1,
          source: { kind: 'canonical', imageId: 'b1', url: 'https://example.com/b1.jpg' },
        },
        {
          uiKey: 'b2',
          colorId: 'col-black',
          isMain: false,
          sortOrder: 2,
          source: { kind: 'canonical', imageId: 'b2', url: 'https://example.com/b2.jpg' },
        },
        {
          uiKey: 'w1',
          colorId: 'col-white',
          isMain: false,
          sortOrder: 3,
          source: { kind: 'canonical', imageId: 'w1', url: 'https://example.com/w1.jpg' },
        },
      ];

      render(
        <MemoryRouter>
          <ProductStudioProvider entryMode="create" initialDraft={draft} canonicalColors={mockColors}>
            <ContextWatcher />
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // Delete B3 (index 0)
      const deleteBtn0 = screen.getByTestId('thumbnail-delete-btn-0');
      fireEvent.click(deleteBtn0);

      await waitFor(() => {
        const blackImgs = studioCtx!.draft.images!.filter((img) => img.colorId === 'col-black');
        expect(blackImgs.map((img) => img.uiKey)).toEqual(['b1', 'b2']);
      });

      // Next Black image (B1) becomes first slide and global isMain = true
      const allImages = studioCtx!.draft.images!;
      const b1 = allImages.find((img) => img.uiKey === 'b1');
      const b2 = allImages.find((img) => img.uiKey === 'b2');

      expect(b1?.isMain).toBe(true);
      expect(b2?.isMain).toBe(false);

      // Slider first slide is now B1
      expect(screen.getByTestId('main-product-image').getAttribute('src')).toContain('b1.jpg');

      // Exactly one isMain exists
      expect(allImages.filter((img) => img.isMain)).toHaveLength(1);
    });

    it('Case H: Delete non-first image (B2) -> B1 remains first slide and isMain = true', async () => {
      let studioCtx: ReturnType<typeof useProductStudio> | null = null;
      function ContextWatcher() {
        studioCtx = useProductStudio();
        return null;
      }

      const draft = setupColorwayDraft();
      // Black: [B1 (isMain), B2]
      draft.images = [
        {
          uiKey: 'b1',
          colorId: 'col-black',
          isMain: true,
          sortOrder: 0,
          source: { kind: 'canonical', imageId: 'b1', url: 'https://example.com/b1.jpg' },
        },
        {
          uiKey: 'b2',
          colorId: 'col-black',
          isMain: false,
          sortOrder: 1,
          source: { kind: 'canonical', imageId: 'b2', url: 'https://example.com/b2.jpg' },
        },
      ];

      render(
        <MemoryRouter>
          <ProductStudioProvider entryMode="create" initialDraft={draft} canonicalColors={mockColors}>
            <ContextWatcher />
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // Delete B2 (index 1)
      const deleteBtn1 = screen.getByTestId('thumbnail-delete-btn-1');
      fireEvent.click(deleteBtn1);

      await waitFor(() => {
        const blackImgs = studioCtx!.draft.images!.filter((img) => img.colorId === 'col-black');
        expect(blackImgs.map((img) => img.uiKey)).toEqual(['b1']);
      });

      // B1 remains first slide and isMain = true
      const allImages = studioCtx!.draft.images!;
      expect(allImages[0].uiKey).toBe('b1');
      expect(allImages[0].isMain).toBe(true);

      // Slider first slide is B1
      expect(screen.getByTestId('main-product-image').getAttribute('src')).toContain('b1.jpg');

      // Exactly one isMain exists
      expect(allImages.filter((img) => img.isMain)).toHaveLength(1);
    });

    it('Case I: Upload new photo B_new -> appends to end, does NOT become cover, B1 remains cover', async () => {
      let studioCtx: ReturnType<typeof useProductStudio> | null = null;
      function ContextWatcher() {
        studioCtx = useProductStudio();
        return null;
      }

      const draft = setupColorwayDraft();
      // Black: [B1 (isMain), B3]
      draft.images = [
        {
          uiKey: 'b1',
          colorId: 'col-black',
          isMain: true,
          sortOrder: 0,
          source: { kind: 'canonical', imageId: 'b1', url: 'https://example.com/b1.jpg' },
        },
        {
          uiKey: 'b3',
          colorId: 'col-black',
          isMain: false,
          sortOrder: 1,
          source: { kind: 'canonical', imageId: 'b3', url: 'https://example.com/b3.jpg' },
        },
      ];

      render(
        <MemoryRouter>
          <ProductStudioProvider entryMode="create" initialDraft={draft} canonicalColors={mockColors}>
            <ContextWatcher />
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // Upload new photo B_new
      const newFile = new File(['dummy_content'], 'b_new.jpg', { type: 'image/jpeg' });
      (newFile as any).__dimensions = { width: 900, height: 1200 };

      const input = screen.getByTestId('thumbnail-add-photo-input');
      fireEvent.change(input, { target: { files: [newFile] } });

      await waitFor(() => {
        const blackImgs = studioCtx!.draft.images!.filter((img) => img.colorId === 'col-black');
        expect(blackImgs).toHaveLength(3);
      });

      const blackImgs = studioCtx!.draft.images!.filter((img) => img.colorId === 'col-black');
      // Appended to end: [B1, B3, B_new]
      expect(blackImgs[0].uiKey).toBe('b1');
      expect(blackImgs[1].uiKey).toBe('b3');
      expect(blackImgs[2].source.kind).toBe('local');

      // B1 remains cover, B_new does NOT become cover
      expect(blackImgs[0].isMain).toBe(true);
      expect(blackImgs[1].isMain).toBe(false);
      expect(blackImgs[2].isMain).toBe(false);

      // Exactly one isMain exists
      expect(studioCtx!.draft.images!.filter((img) => img.isMain)).toHaveLength(1);
    });
  });

  describe('Workspace Parity & Invariants (Cases J, K, L, M)', () => {
    it('Case J: Visual <-> Form Workspace preserve exact order', async () => {
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
              mediaMode: 'GENERAL',
              images: [
                {
                  uiKey: 'img-A',
                  source: { kind: 'canonical', imageId: 'img-A', url: 'https://example.com/A.jpg' },
                  isMain: true,
                  sortOrder: 0,
                  colorId: null,
                },
                {
                  uiKey: 'img-B',
                  source: { kind: 'canonical', imageId: 'img-B', url: 'https://example.com/B.jpg' },
                  isMain: false,
                  sortOrder: 1,
                  colorId: null,
                },
                {
                  uiKey: 'img-C',
                  source: { kind: 'canonical', imageId: 'img-C', url: 'https://example.com/C.jpg' },
                  isMain: false,
                  sortOrder: 2,
                  colorId: null,
                },
              ],
            }}
          >
            <ContextWatcher />
            <ProductStudio />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // Reorder in Visual Workspace: drag C (index 2) to index 0 -> [C, A, B]
      dragVisualThumbnail(2, 0);

      await waitFor(() => {
        const images = studioCtx!.draft.images!;
        expect(images.map((img) => img.uiKey)).toEqual(['img-C', 'img-A', 'img-B']);
      });

      // Switch to Form Workspace
      fireEvent.click(screen.getByTestId('studio-view-toggle-form'));
      // Open Media tab in Form Workspace
      fireEvent.click(screen.getByTestId('studio-section-btn-media'));

      // Form cards match exact order: C, A, B
      expect(screen.getByTestId('form-media-card-0')).toBeTruthy();
      expect(screen.getByTestId('form-media-card-1')).toBeTruthy();
      expect(screen.getByTestId('form-media-card-2')).toBeTruthy();

      expect(studioCtx!.draft.images!.map((img) => img.uiKey)).toEqual(['img-C', 'img-A', 'img-B']);

      // Switch back to Visual Workspace
      fireEvent.click(screen.getByTestId('studio-view-toggle-visual'));

      // Order preserved in Visual Workspace
      expect(getThumbnailImgSrc(0)).toContain('C.jpg');
      expect(getThumbnailImgSrc(1)).toContain('A.jpg');
      expect(getThumbnailImgSrc(2)).toContain('B.jpg');
      expect(studioCtx!.draft.images!.map((img) => img.uiKey)).toEqual(['img-C', 'img-A', 'img-B']);
    });

    it('Case L: Manual action "Сделать главной" removed from DOM in both Visual and Form Workspaces', () => {
      render(
        <MemoryRouter>
          <ProductStudioProvider
            entryMode="create"
            initialDraft={{
              mediaMode: 'GENERAL',
              images: [
                {
                  uiKey: 'img-A',
                  source: { kind: 'canonical', imageId: 'img-A', url: 'https://example.com/A.jpg' },
                  isMain: true,
                  sortOrder: 0,
                  colorId: null,
                },
                {
                  uiKey: 'img-B',
                  source: { kind: 'canonical', imageId: 'img-B', url: 'https://example.com/B.jpg' },
                  isMain: false,
                  sortOrder: 1,
                  colorId: null,
                },
              ],
            }}
          >
            <ProductStudio />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // In Visual Workspace: assert no "Сделать главной" button exists
      expect(screen.queryByTestId('thumbnail-make-cover-0')).toBeNull();
      expect(screen.queryByTestId('thumbnail-make-cover-1')).toBeNull();
      expect(screen.queryByText(/Сделать главной/i)).toBeNull();

      // Switch to Form Workspace
      fireEvent.click(screen.getByTestId('studio-view-toggle-form'));
      fireEvent.click(screen.getByTestId('studio-section-btn-media'));

      // In Form Workspace: assert no "Сделать главной" button exists
      expect(screen.queryByTestId('form-media-make-cover-0')).toBeNull();
      expect(screen.queryByTestId('form-media-make-cover-1')).toBeNull();
      expect(screen.queryByText(/Сделать главной/i)).toBeNull();
    });

    it('Case M: "Обложка" badge appears only on the gallery owning global isMain and moves automatically after reorder', async () => {
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
                  source: { kind: 'canonical', imageId: 'b1', url: 'https://example.com/b1.jpg' },
                },
                {
                  uiKey: 'b2',
                  colorId: 'col-black',
                  isMain: false,
                  sortOrder: 1,
                  source: { kind: 'canonical', imageId: 'b2', url: 'https://example.com/b2.jpg' },
                },
                {
                  uiKey: 'w1',
                  colorId: 'col-white',
                  isMain: false,
                  sortOrder: 2,
                  source: { kind: 'canonical', imageId: 'w1', url: 'https://example.com/w1.jpg' },
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

      // On Black gallery (first gallery): thumbnail 0 has "Обложка" badge, thumbnail 1 does not
      expect(screen.getByTestId('thumbnail-main-badge-0')).toBeTruthy();
      expect(screen.getByTestId('thumbnail-main-badge-0').textContent).toContain('Обложка');
      expect(screen.queryByTestId('thumbnail-main-badge-1')).toBeNull();

      // Reorder Black: drag B2 (index 1) to B1 (index 0)
      dragVisualThumbnail(1, 0);

      await waitFor(() => {
        const blackImgs = studioCtx!.draft.images!.filter((img) => img.colorId === 'col-black');
        expect(blackImgs.map((img) => img.uiKey)).toEqual(['b2', 'b1']);
      });

      // Now thumbnail 0 (which is now B2) has the "Обложка" badge
      expect(screen.getByTestId('thumbnail-main-badge-0')).toBeTruthy();
      expect(screen.getByTestId('thumbnail-main-badge-0').textContent).toContain('Обложка');
      expect(screen.queryByTestId('thumbnail-main-badge-1')).toBeNull();

      // Switch to White gallery
      fireEvent.click(screen.getByTestId('color-swatch-col-white'));

      await waitFor(() => {
        expect(getThumbnailImgSrc(0)).toContain('w1.jpg');
      });

      // On White gallery: NO "Обложка" badge appears because Black owns global isMain
      expect(screen.queryByTestId('thumbnail-main-badge-0')).toBeNull();
    });
  });
});
