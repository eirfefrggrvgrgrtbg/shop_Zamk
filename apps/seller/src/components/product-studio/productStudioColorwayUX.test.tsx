/* @vitest-environment jsdom */
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ProductStudioVisualWorkspace } from './ProductStudioVisualWorkspace';
import { ProductStudioFormWorkspace } from './ProductStudioFormWorkspace';
import {
  ProductStudioProvider,
  useProductStudio,
  type ProductStudioDraft,
} from '../../contexts/ProductStudioContext';
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

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('SELLER MEDIA.2C — Color Gallery Authoring UX', () => {
  describe('1. GENERAL Mode', () => {
    it('shows simple common gallery without color tabs and uploads with colorId = null', async () => {
      let studioCtx: ReturnType<typeof useProductStudio> | null = null;
      function ContextWatcher() {
        studioCtx = useProductStudio();
        return null;
      }

      const initialDraft: Partial<ProductStudioDraft> = {
        title: 'Тестовый товар',
        mediaMode: 'GENERAL',
        colors: [
          { id: 'col-black', name: 'Чёрный', hex: '#000000' },
        ],
        variants: [
          { colorId: 'col-black', colorName: 'Чёрный', colorHex: '#000000', size: 'M', isActive: true },
        ],
        images: [
          {
            uiKey: 'img-1',
            isMain: true,
            sortOrder: 0,
            colorId: null,
            isUnassigned: false,
            source: { kind: 'local', clientMediaId: 'cm-1', file: new File([], 'img1.jpg'), previewUrl: 'blob:http://localhost/1' },
          },
        ],
      };

      render(
        <MemoryRouter>
          <ProductStudioProvider
            entryMode="create"
            initialDraft={initialDraft}
            canonicalColors={mockColors}
          >
            <ContextWatcher />
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // In GENERAL mode, colorway tabs bar must not be rendered
      expect(screen.queryByTestId('colorway-tabs-bar')).toBeNull();

      // Uploading a photo assigns colorId: null and isUnassigned: false
      const fileInput = screen.getByTestId('thumbnail-add-photo-input') as HTMLInputElement;
      expect(fileInput).toBeTruthy();

      const testFile = new File(['bits'], 'new-general.jpg', { type: 'image/jpeg' });
      fireEvent.change(fileInput, { target: { files: [testFile] } });

      await waitFor(() => {
        expect(studioCtx?.draft?.images).toHaveLength(2);
      });

      const newImg = studioCtx!.draft!.images![1];
      expect(newImg.colorId).toBeNull();
      expect(newImg.isUnassigned).toBe(false);
    });
  });

  describe('2. COLORWAY Mode Tabs Bar and Gallery Filtering', () => {
    it('renders color tabs with photo counts and filters visible stage/thumbnails to active color', async () => {
      const initialDraft: Partial<ProductStudioDraft> = {
        title: 'Колорвей платье',
        mediaMode: 'COLORWAY',
        colors: [
          { id: 'col-black', name: 'Чёрный', hex: '#000000' },
          { id: 'col-white', name: 'Белый', hex: '#ffffff' },
          { id: 'col-red', name: 'Красный', hex: '#ff0000' },
        ],
        variants: [
          { colorId: 'col-black', colorName: 'Чёрный', colorHex: '#000000', size: 'S', isActive: true },
          { colorId: 'col-white', colorName: 'Белый', colorHex: '#ffffff', size: 'M', isActive: true },
          { colorId: 'col-red', colorName: 'Красный', colorHex: '#ff0000', size: 'L', isActive: true },
        ],
        images: [
          {
            uiKey: 'img-black-1',
            colorId: 'col-black',
            isMain: true,
            sortOrder: 0,
            isUnassigned: false,
            source: { kind: 'local', clientMediaId: 'b1', file: new File([], 'b1.jpg'), previewUrl: 'blob:http://localhost/b1' },
          },
          {
            uiKey: 'img-black-2',
            colorId: 'col-black',
            isMain: false,
            sortOrder: 1,
            isUnassigned: false,
            source: { kind: 'local', clientMediaId: 'b2', file: new File([], 'b2.jpg'), previewUrl: 'blob:http://localhost/b2' },
          },
          {
            uiKey: 'img-white-1',
            colorId: 'col-white',
            isMain: false,
            sortOrder: 2,
            isUnassigned: false,
            source: { kind: 'local', clientMediaId: 'w1', file: new File([], 'w1.jpg'), previewUrl: 'blob:http://localhost/w1' },
          },
        ],
      };

      render(
        <MemoryRouter>
          <ProductStudioProvider
            entryMode="create"
            initialDraft={initialDraft}
            canonicalColors={mockColors}
          >
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('colorway-tabs-bar')).toBeTruthy();
      });

      // Check tabs and photo count labels
      const blackTab = screen.getByTestId('colorway-tab-col-black');
      const whiteTab = screen.getByTestId('colorway-tab-col-white');
      const redTab = screen.getByTestId('colorway-tab-col-red');

      expect(blackTab.textContent).toContain('Чёрный');
      expect(blackTab.textContent).toContain('· 2');

      expect(whiteTab.textContent).toContain('Белый');
      expect(whiteTab.textContent).toContain('· 1');

      expect(redTab.textContent).toContain('Красный');
      expect(redTab.textContent).toContain('· 0');

      // Initially Black is active (first color)
      expect(blackTab.getAttribute('aria-selected')).toBe('true');

      // 2 thumbnails for black
      expect(screen.getByTestId('pdp-thumbnail-0')).toBeTruthy();
      expect(screen.getByTestId('pdp-thumbnail-1')).toBeTruthy();
      expect(screen.queryByTestId('pdp-thumbnail-2')).toBeNull();

      // Switch to White tab
      fireEvent.click(whiteTab);
      expect(whiteTab.getAttribute('aria-selected')).toBe('true');

      // 1 thumbnail for white
      expect(screen.getByTestId('pdp-thumbnail-0')).toBeTruthy();
      expect(screen.queryByTestId('pdp-thumbnail-1')).toBeNull();

      // Switch to Red tab (0 photos)
      fireEvent.click(redTab);
      expect(redTab.getAttribute('aria-selected')).toBe('true');
      expect(screen.getByText(/Для цвета «Красный» пока нет фотографий/i)).toBeTruthy();
      expect(screen.getByTestId('empty-stage-add-btn')).toBeTruthy();
    });
  });

  describe('3. Direct Upload into Active Color', () => {
    it('immediately assigns colorId of active tab to uploaded photo without modal', async () => {
      let studioCtx: ReturnType<typeof useProductStudio> | null = null;
      function ContextWatcher() {
        studioCtx = useProductStudio();
        return null;
      }

      const initialDraft: Partial<ProductStudioDraft> = {
        title: 'Колорвей платье',
        mediaMode: 'COLORWAY',
        colors: [
          { id: 'col-black', name: 'Чёрный', hex: '#000000' },
          { id: 'col-white', name: 'Белый', hex: '#ffffff' },
        ],
        variants: [
          { colorId: 'col-black', colorName: 'Чёрный', colorHex: '#000000', size: 'S', isActive: true },
          { colorId: 'col-white', colorName: 'Белый', colorHex: '#ffffff', size: 'M', isActive: true },
        ],
        images: [
          {
            uiKey: 'img-black-1',
            colorId: 'col-black',
            isMain: true,
            sortOrder: 0,
            isUnassigned: false,
            source: { kind: 'local', clientMediaId: 'b1', file: new File([], 'b1.jpg'), previewUrl: 'blob:http://localhost/b1' },
          },
        ],
      };

      render(
        <MemoryRouter>
          <ProductStudioProvider
            entryMode="create"
            initialDraft={initialDraft}
            canonicalColors={mockColors}
          >
            <ContextWatcher />
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('colorway-tab-col-white')).toBeTruthy();
      });

      // Select White tab
      const whiteTab = screen.getByTestId('colorway-tab-col-white');
      fireEvent.click(whiteTab);

      // Find file input and upload
      const fileInput = screen.getByTestId('empty-stage-photo-input') as HTMLInputElement;
      const testFile = new File(['white-bits'], 'white-photo.jpg', { type: 'image/jpeg' });
      fireEvent.change(fileInput, { target: { files: [testFile] } });

      await waitFor(() => {
        expect(studioCtx?.draft?.images).toHaveLength(2);
      });

      const uploadedImage = studioCtx!.draft!.images![1];
      expect(uploadedImage.colorId).toBe('col-white');
      expect(uploadedImage.isUnassigned).toBe(false);

      // Tab count for White updates to 1
      await waitFor(() => {
        expect(screen.getByTestId('colorway-tab-col-white').textContent).toContain('· 1');
      });
    });
  });

  describe('4. Unassigned State Tab and Action Button', () => {
    it('shows unassigned tab and action button when unassigned > 0, hides them when unassigned == 0', async () => {
      let studioCtx: ReturnType<typeof useProductStudio> | null = null;
      function ContextWatcher() {
        studioCtx = useProductStudio();
        return null;
      }

      const initialDraft: Partial<ProductStudioDraft> = {
        title: 'Платье',
        mediaMode: 'COLORWAY',
        colors: [
          { id: 'col-black', name: 'Чёрный', hex: '#000000' },
          { id: 'col-white', name: 'Белый', hex: '#ffffff' },
        ],
        variants: [
          { colorId: 'col-black', colorName: 'Чёрный', colorHex: '#000000', size: 'S', isActive: true },
          { colorId: 'col-white', colorName: 'Белый', colorHex: '#ffffff', size: 'M', isActive: true },
        ],
        images: [
          {
            uiKey: 'img-unassigned-1',
            colorId: null,
            isMain: true,
            sortOrder: 0,
            isUnassigned: true,
            source: { kind: 'local', clientMediaId: 'u1', file: new File([], 'u1.jpg'), previewUrl: 'blob:http://localhost/u1' },
          },
          {
            uiKey: 'img-black-1',
            colorId: 'col-black',
            isMain: false,
            sortOrder: 1,
            isUnassigned: false,
            source: { kind: 'local', clientMediaId: 'b1', file: new File([], 'b1.jpg'), previewUrl: 'blob:http://localhost/b1' },
          },
        ],
      };

      render(
        <MemoryRouter>
          <ProductStudioProvider
            entryMode="create"
            initialDraft={initialDraft}
            canonicalColors={mockColors}
          >
            <ContextWatcher />
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // Unassigned tab exists
      const unassignedTab = screen.getByTestId('colorway-tab-unassigned');
      expect(unassignedTab).toBeTruthy();
      expect(unassignedTab.textContent).toContain('Нераспределённые');
      expect(unassignedTab.textContent).toContain('· 1');

      // Bind button is rendered with count
      const bindBtn = screen.getByTestId('bind-photos-to-colors-btn');
      expect(bindBtn).toBeTruthy();
      expect(bindBtn.textContent).toContain('Распределить фотографии · 1');

      // Clicking bind button opens photo-color-modal
      fireEvent.click(bindBtn);
      expect(screen.getByTestId('photo-color-binding-modal')).toBeTruthy();

      // Update draft so unassigned image is assigned
      act(() => {
        studioCtx!.updateDraft({
          images: [
            {
              ...initialDraft.images![0],
              colorId: 'col-black',
              isUnassigned: false,
            },
            initialDraft.images![1],
          ],
        });
      });

      // Unassigned tab must NOT exist
      await waitFor(() => {
        expect(screen.queryByTestId('colorway-tab-unassigned')).toBeNull();
      });

      // Bind photos button must NOT exist in control row
      expect(screen.queryByTestId('bind-photos-to-colors-btn')).toBeNull();
    });
  });

  describe('5. Global isMain Cover Photo Preservation', () => {
    it('maintains exactly one global cover across colors when setting cover or deleting', async () => {
      let studioCtx: ReturnType<typeof useProductStudio> | null = null;
      function ContextWatcher() {
        studioCtx = useProductStudio();
        return null;
      }

      const initialDraft: Partial<ProductStudioDraft> = {
        title: 'Товар',
        mediaMode: 'COLORWAY',
        colors: [
          { id: 'col-black', name: 'Чёрный', hex: '#000000' },
          { id: 'col-white', name: 'Белый', hex: '#ffffff' },
        ],
        variants: [
          { colorId: 'col-black', colorName: 'Чёрный', colorHex: '#000000', size: 'S', isActive: true },
          { colorId: 'col-white', colorName: 'Белый', colorHex: '#ffffff', size: 'M', isActive: true },
        ],
        images: [
          {
            uiKey: 'img-b1',
            colorId: 'col-black',
            isMain: true,
            sortOrder: 0,
            isUnassigned: false,
            source: { kind: 'local', clientMediaId: 'b1', file: new File([], 'b1.jpg'), previewUrl: 'blob:http://localhost/b1' },
          },
          {
            uiKey: 'img-w1',
            colorId: 'col-white',
            isMain: false,
            sortOrder: 1,
            isUnassigned: false,
            source: { kind: 'local', clientMediaId: 'w1', file: new File([], 'w1.jpg'), previewUrl: 'blob:http://localhost/w1' },
          },
        ],
      };

      render(
        <MemoryRouter>
          <ProductStudioProvider
            entryMode="create"
            initialDraft={initialDraft}
            canonicalColors={mockColors}
          >
            <ContextWatcher />
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // Initially Black has the cover
      expect(screen.getByTestId('thumbnail-main-badge-0')).toBeTruthy();

      // Switch to White tab
      fireEvent.click(screen.getByTestId('colorway-tab-col-white'));

      // Manual make-cover button no longer exists under 2C3
      expect(screen.queryByTestId('thumbnail-make-cover-0')).toBeNull();

      // Switch back to Black tab and delete Black cover photo
      fireEvent.click(screen.getByTestId('colorway-tab-col-black'));
      const deleteBtn = screen.getByTestId('thumbnail-delete-btn-0');
      fireEvent.click(deleteBtn);

      // Now only White photo remains and automatically inherits global cover
      await waitFor(() => {
        expect(studioCtx?.draft?.images).toHaveLength(1);
        const remainingImg = studioCtx!.draft!.images![0];
        expect(remainingImg.colorId).toBe('col-white');
        expect(remainingImg.isMain).toBe(true);
      });
    });
  });

  describe('6. Single-Color Product', () => {
    it('renders single color tab and uploads directly into it', async () => {
      let studioCtx: ReturnType<typeof useProductStudio> | null = null;
      function ContextWatcher() {
        studioCtx = useProductStudio();
        return null;
      }

      const initialDraft: Partial<ProductStudioDraft> = {
        title: 'Монохром',
        mediaMode: 'COLORWAY',
        colors: [
          { id: 'col-black', name: 'Чёрный', hex: '#000000' },
        ],
        variants: [
          { colorId: 'col-black', colorName: 'Чёрный', colorHex: '#000000', size: 'OneSize', isActive: true },
        ],
        images: [],
      };

      render(
        <MemoryRouter>
          <ProductStudioProvider
            entryMode="create"
            initialDraft={initialDraft}
            canonicalColors={mockColors}
          >
            <ContextWatcher />
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // Single tab for Black
      expect(screen.getByTestId('colorway-tab-col-black')).toBeTruthy();
      expect(screen.queryByTestId('colorway-tab-col-white')).toBeNull();

      // Zero photo state
      expect(screen.getByText(/Для цвета «Чёрный» пока нет фотографий/i)).toBeTruthy();

      // Direct upload
      const fileInput = screen.getByTestId('empty-stage-photo-input') as HTMLInputElement;
      fireEvent.change(fileInput, {
        target: { files: [new File(['bits'], 'black.jpg', { type: 'image/jpeg' })] },
      });

      await waitFor(() => {
        expect(studioCtx?.draft?.images).toHaveLength(1);
        expect(studioCtx!.draft!.images![0].colorId).toBe('col-black');
        expect(studioCtx!.draft!.images![0].isMain).toBe(true);
      });
    });
  });

  describe('7. Form Workspace Colorway Alignment', () => {
    it('renders color tabs and uploads directly into active color in Form workspace', async () => {
      let studioCtx: ReturnType<typeof useProductStudio> | null = null;
      function ContextWatcher() {
        studioCtx = useProductStudio();
        return null;
      }

      const initialDraft: Partial<ProductStudioDraft> = {
        title: 'Форма товар',
        mediaMode: 'COLORWAY',
        colors: [
          { id: 'col-black', name: 'Чёрный', hex: '#000000' },
          { id: 'col-white', name: 'Белый', hex: '#ffffff' },
        ],
        variants: [
          { colorId: 'col-black', colorName: 'Чёрный', colorHex: '#000000', size: 'S', isActive: true },
          { colorId: 'col-white', colorName: 'Белый', colorHex: '#ffffff', size: 'M', isActive: true },
        ],
        images: [
          {
            uiKey: 'img-b1',
            colorId: 'col-black',
            isMain: true,
            sortOrder: 0,
            isUnassigned: false,
            source: { kind: 'local', clientMediaId: 'b1', file: new File([], 'b1.jpg'), previewUrl: 'blob:http://localhost/b1' },
          },
        ],
      };

      render(
        <MemoryRouter>
          <ProductStudioProvider
            entryMode="create"
            initialDraft={initialDraft}
            canonicalColors={mockColors}
          >
            <ContextWatcher />
            <ProductStudioFormWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // Switch to Media section
      fireEvent.click(screen.getByTestId('studio-section-btn-media'));

      // Form tabs bar
      await waitFor(() => {
        expect(screen.getByTestId('form-colorway-tabs-bar')).toBeTruthy();
      });
      expect(screen.getByTestId('form-colorway-tab-col-black').textContent).toContain('Чёрный');
      expect(screen.getByTestId('form-colorway-tab-col-black').textContent).toContain('· 1');
      expect(screen.getByTestId('form-colorway-tab-col-white').textContent).toContain('Белый');
      expect(screen.getByTestId('form-colorway-tab-col-white').textContent).toContain('· 0');

      // Bind button hidden since unassigned is 0
      expect(screen.queryByTestId('form-bind-photos-to-colors-btn')).toBeNull();

      // Click White tab
      fireEvent.click(screen.getByTestId('form-colorway-tab-col-white'));

      // Upload file in form workspace
      const fileInput = screen.getByTestId('form-add-media-input') as HTMLInputElement;
      fireEvent.change(fileInput, {
        target: { files: [new File(['bits'], 'form-white.jpg', { type: 'image/jpeg' })] },
      });

      await waitFor(() => {
        expect(studioCtx?.draft?.images).toHaveLength(2);
        const wImg = studioCtx?.draft?.images?.find(i => i.colorId === 'col-white');
        expect(wImg).toBeDefined();
        expect(wImg?.isUnassigned).toBe(false);
      });
    });
  });
});
