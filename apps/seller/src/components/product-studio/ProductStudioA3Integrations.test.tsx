/* @vitest-environment jsdom */
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ProductStudio } from './ProductStudio';
import { ProductStudioVisualWorkspace } from './ProductStudioVisualWorkspace';
import { ProductStudioFormWorkspace } from './ProductStudioFormWorkspace';
import {
  ProductStudioProvider,
  useProductStudio,
  type ProductStudioDraft,
} from '../../contexts/ProductStudioContext';

const mockColors = [
  { id: 'col-black', nameRu: 'Чёрный', hex: '#000000' },
  { id: 'col-white', nameRu: 'Белый', hex: '#ffffff' },
  { id: 'col-red', nameRu: 'Красный', hex: '#ff0000' },
];

const mockCategorySchemas: Record<string, any> = {
  'cat-clothing': {
    id: 'sch-clothing',
    categoryId: 'cat-clothing',
    dimensionType: 'COLOR_AND_SIZE',
    allowedSizeSystems: [{ id: 'sys-int', name: 'INT', isDefault: true }],
  },
  'cat-color-only': {
    id: 'sch-color-only',
    categoryId: 'cat-color-only',
    dimensionType: 'COLOR_ONLY',
    allowedSizeSystems: [],
  },
  'cat-size-only': {
    id: 'sch-size-only',
    categoryId: 'cat-size-only',
    dimensionType: 'ONLY_SIZE',
    allowedSizeSystems: [{ id: 'sys-int', name: 'INT', isDefault: true }],
  },
  'cat-single': {
    id: 'sch-single',
    categoryId: 'cat-single',
    dimensionType: 'SINGLE_VARIANT',
    allowedSizeSystems: [],
  },
};

const mockSizeValues: Record<string, any[]> = {
  'sys-int': [
    { id: 'sz-s', sizeSystemId: 'sys-int', value: 'S' },
    { id: 'sz-m', sizeSystemId: 'sys-int', value: 'M' },
  ],
};

vi.mock('@zamk/api-client/src/seller', async (importOriginal: any) => {
  const actual = await importOriginal();
  return {
    ...actual,
    getSellerCategories: vi.fn().mockImplementation(() =>
      Promise.resolve([
        { id: 'cat-clothing', name: 'Одежда' },
        { id: 'cat-color-only', name: 'Косметика' },
        { id: 'cat-size-only', name: 'Обувь' },
        { id: 'cat-single', name: 'Книга' },
      ])
    ),
    getSellerColors: vi.fn().mockImplementation(() => Promise.resolve(mockColors)),
    getSellerCategorySchema: vi.fn().mockImplementation((catId: string) =>
      Promise.resolve(mockCategorySchemas[catId] || null)
    ),
    getSellerSizeValues: vi.fn().mockImplementation((sysId: string) =>
      Promise.resolve(mockSizeValues[sysId] || [])
    ),
  };
});

// Setup jsdom URL blob mocks
let objectUrlCounter = 0;
const createdUrls: string[] = [];
const revokedUrls: string[] = [];

beforeEach(() => {
  objectUrlCounter = 0;
  createdUrls.length = 0;
  revokedUrls.length = 0;

  globalThis.URL.createObjectURL = vi.fn((_file: any) => {
    objectUrlCounter++;
    const url = `blob:http://localhost/test-blob-${objectUrlCounter}`;
    createdUrls.push(url);
    return url;
  });

  globalThis.URL.revokeObjectURL = vi.fn((url: string) => {
    revokedUrls.push(url);
  });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('ProductStudio PS.R4A.5A3 Integrations', () => {
  describe('1. Media Session Persistence Across View Toggles', () => {
    it('uploaded photo URL survives Visual -> Form -> Visual toggle without revoking', async () => {
      let studioCtx: ReturnType<typeof useProductStudio> | null = null;
      function ContextWatcher() {
        studioCtx = useProductStudio();
        return null;
      }

      render(
        <MemoryRouter>
          <ProductStudioProvider entryMode="create">
            <ContextWatcher />
            <ProductStudio />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // Start in Visual view
      expect(screen.getByTestId('studio-visual-workspace')).toBeTruthy();

      // Find file input and upload photo
      const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement;
      expect(fileInput).toBeTruthy();

      const testFile = new File(['image-bits'], 'test-photo.jpg', { type: 'image/jpeg' });
      fireEvent.change(fileInput, { target: { files: [testFile] } });

      await waitFor(() => {
        expect(studioCtx?.draft?.images).toHaveLength(1);
      });

      const uploadedBlobUrl = studioCtx!.draft!.images![0].url;
      expect(uploadedBlobUrl).toContain('blob:');
      expect(createdUrls).toContain(uploadedBlobUrl);
      expect(revokedUrls).not.toContain(uploadedBlobUrl);

      // Switch to Form view
      const formTab = screen.getByTestId('studio-view-toggle-form');
      fireEvent.click(formTab);

      await waitFor(() => {
        expect(screen.getByTestId('studio-form-workspace')).toBeTruthy();
        expect(screen.queryByTestId('studio-visual-workspace')).toBeNull();
      });

      // The URL MUST NOT have been revoked upon unmounting VisualWorkspace
      expect(revokedUrls).not.toContain(uploadedBlobUrl);

      // Switch back to Visual view
      const visualTab = screen.getByTestId('studio-view-toggle-visual');
      fireEvent.click(visualTab);

      await waitFor(() => {
        expect(screen.getByTestId('studio-visual-workspace')).toBeTruthy();
      });

      // URL is still not revoked and photo is rendered
      expect(revokedUrls).not.toContain(uploadedBlobUrl);
      expect(studioCtx!.draft!.images![0].url).toBe(uploadedBlobUrl);
      const mainImg = screen.getByTestId('main-product-image') as HTMLImageElement;
      expect(mainImg.src).toBe(uploadedBlobUrl);
    });

    it('removing a photo revokes its registered URL', async () => {
      let studioCtx: ReturnType<typeof useProductStudio> | null = null;
      function ContextWatcher() {
        studioCtx = useProductStudio();
        return null;
      }

      render(
        <MemoryRouter>
          <ProductStudioProvider entryMode="create">
            <ContextWatcher />
            <ProductStudio />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement;
      const testFile = new File(['image-bits-2'], 'photo-2.jpg', { type: 'image/jpeg' });
      fireEvent.change(fileInput, { target: { files: [testFile] } });

      await waitFor(() => {
        expect(studioCtx?.draft?.images).toHaveLength(1);
      });

      const uploadedUrl = studioCtx!.draft!.images![0].url;

      // Remove the image
      studioCtx!.updateDraft({ images: [] });

      await waitFor(() => {
        expect(revokedUrls).toContain(uploadedUrl);
      });
    });

    it('unmounting entire ProductStudioProvider revokes remaining object URLs', async () => {
      let studioCtx: ReturnType<typeof useProductStudio> | null = null;
      function ContextWatcher() {
        studioCtx = useProductStudio();
        return null;
      }

      const { unmount } = render(
        <MemoryRouter>
          <ProductStudioProvider entryMode="create">
            <ContextWatcher />
            <ProductStudio />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement;
      const testFile = new File(['image-bits-3'], 'photo-3.jpg', { type: 'image/jpeg' });
      fireEvent.change(fileInput, { target: { files: [testFile] } });

      await waitFor(() => {
        expect(studioCtx?.draft?.images).toHaveLength(1);
      });

      const uploadedUrl = studioCtx!.draft!.images![0].url;
      expect(revokedUrls).not.toContain(uploadedUrl);

      // Unmount entire provider
      unmount();

      expect(revokedUrls).toContain(uploadedUrl);
    });
  });

  describe('2. Dedicated Photo ↔ Color Binding Modal', () => {
    it('button is disabled with hint when draft has 0 colors, enabled when colors exist', async () => {
      let studioCtx: ReturnType<typeof useProductStudio> | null = null;
      function ContextWatcher() {
        studioCtx = useProductStudio();
        return null;
      }

      const draftNoColors: Partial<ProductStudioDraft> = {
        images: [{ url: 'https://cdn/1.jpg', isMain: true }],
        colors: [],
      };

      render(
        <MemoryRouter>
          <ProductStudioProvider entryMode="create" initialDraft={draftNoColors}>
            <ContextWatcher />
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      const bindBtn = screen.getByTestId('bind-photos-to-colors-btn') as HTMLButtonElement;
      expect(bindBtn.disabled).toBe(true);
      expect(bindBtn.getAttribute('title')).toContain('Сначала добавьте цвета');

      // Now add colors via updateDraft
      studioCtx!.updateDraft({
        colors: [{ id: 'col-black', name: 'Чёрный', hex: '#000000' }],
      });

      await waitFor(() => {
        expect(bindBtn.disabled).toBe(false);
      });
      expect(bindBtn.getAttribute('title')).toBe('Привязать фото к цветам');
    });

    it('canceling binding modal does not mutate draft color assignments', async () => {
      let studioCtx: ReturnType<typeof useProductStudio> | null = null;
      function ContextWatcher() {
        studioCtx = useProductStudio();
        return null;
      }

      const initialDraft: Partial<ProductStudioDraft> = {
        images: [
          { url: 'https://cdn/1.jpg', isMain: true, colorId: null },
          { url: 'https://cdn/2.jpg', isMain: false, colorId: null },
        ],
        colors: [
          { id: 'col-black', name: 'Чёрный', hex: '#000000' },
          { id: 'col-white', name: 'Белый', hex: '#ffffff' },
        ],
      };

      render(
        <MemoryRouter>
          <ProductStudioProvider entryMode="create" initialDraft={initialDraft}>
            <ContextWatcher />
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      const openBtn = screen.getByTestId('bind-photos-to-colors-btn');
      fireEvent.click(openBtn);

      const modal = screen.getByTestId('photo-color-binding-modal');
      expect(modal).toBeTruthy();

      // Change selection in row 0
      const select0 = within(modal).getByTestId('photo-color-select-0') as HTMLSelectElement;
      fireEvent.change(select0, { target: { value: 'col-black' } });

      // Click Cancel
      const cancelBtn = within(modal).getByTestId('photo-color-modal-cancel');
      fireEvent.click(cancelBtn);

      await waitFor(() => {
        expect(screen.queryByTestId('photo-color-binding-modal')).toBeNull();
      });

      // Draft images remain unchanged
      expect(studioCtx!.draft!.images![0].colorId).toBeNull();
      expect(screen.queryByTestId('thumbnail-color-dot-0')).toBeNull();
    });

    it('committing binding modal updates colorId atomically and renders thumbnail color dots', async () => {
      let studioCtx: ReturnType<typeof useProductStudio> | null = null;
      function ContextWatcher() {
        studioCtx = useProductStudio();
        return null;
      }

      const initialDraft: Partial<ProductStudioDraft> = {
        images: [
          { url: 'https://cdn/1.jpg', isMain: true, colorId: null },
          { url: 'https://cdn/2.jpg', isMain: false, colorId: null },
        ],
        colors: [
          { id: 'col-black', name: 'Чёрный', hex: '#000000' },
          { id: 'col-white', name: 'Белый', hex: '#ffffff' },
        ],
      };

      render(
        <MemoryRouter>
          <ProductStudioProvider entryMode="create" initialDraft={initialDraft}>
            <ContextWatcher />
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      const openBtn = screen.getByTestId('bind-photos-to-colors-btn');
      fireEvent.click(openBtn);

      const modal = screen.getByTestId('photo-color-binding-modal');
      const select0 = within(modal).getByTestId('photo-color-select-0') as HTMLSelectElement;
      const select1 = within(modal).getByTestId('photo-color-select-1') as HTMLSelectElement;

      // Assign photo 0 to black, photo 1 to white
      fireEvent.change(select0, { target: { value: 'col-black' } });
      fireEvent.change(select1, { target: { value: 'col-white' } });

      // Click submit
      const submitBtn = within(modal).getByTestId('photo-color-modal-submit');
      fireEvent.click(submitBtn);

      await waitFor(() => {
        expect(screen.queryByTestId('photo-color-binding-modal')).toBeNull();
      });

      expect(studioCtx!.draft!.images![0].colorId).toBe('col-black');
      expect(studioCtx!.draft!.images![1].colorId).toBe('col-white');

      // Visual thumbnail indicators
      const dot0 = screen.getAllByTestId('thumbnail-color-dot-0')[0];
      const dot1 = screen.getAllByTestId('thumbnail-color-dot-1')[0];
      expect(dot0).toBeTruthy();
      expect(dot1).toBeTruthy();
      expect(dot0.getAttribute('title')).toBe('Цвет: Чёрный');
      expect(dot1.getAttribute('title')).toBe('Цвет: Белый');
    });
  });

  describe('3. Validation States and Dynamic Variant Readiness', () => {
    it('clean draft shows calm neutral state: title, price, media, color, size have no red error states or helpers', async () => {
      render(
        <MemoryRouter>
          <ProductStudioProvider entryMode="create">
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // Title: neutral, helper not visible on clean draft
      expect(screen.queryByTestId('title-required-helper')).toBeNull();

      // Price: neutral, helper not visible on clean draft
      expect(screen.queryByTestId('price-required-helper')).toBeNull();

      // Empty media slot: neutral dashed border, no red border, helper not visible
      const emptySlot = screen.getByTestId('presentation-empty-media-slot');
      expect(screen.queryByTestId('media-required-helper')).toBeNull();
      expect(emptySlot.className).toContain('border-ash/30');
      expect(emptySlot.className).not.toContain('border-red');

      // Color and size: NOT visible before category schema exists
      expect(screen.queryByTestId('color-required-helper')).toBeNull();
      expect(screen.queryByTestId('size-required-helper')).toBeNull();
    });

    it('dynamic schema COLOR_AND_SIZE with attention shows color and size helpers, clearing live as each is satisfied', async () => {
      let studioCtx: ReturnType<typeof useProductStudio> | null = null;
      function ContextWatcher() {
        studioCtx = useProductStudio();
        return null;
      }

      render(
        <MemoryRouter>
          <ProductStudioProvider entryMode="create">
            <ContextWatcher />
            <ProductStudioVisualWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // Set category with COLOR_AND_SIZE
      studioCtx!.updateDraft({
        categoryId: 'cat-clothing',
        categoryName: 'Одежда',
      });

      await waitFor(() => {
        expect(studioCtx?.categorySchema?.dimensionType).toBe('COLOR_AND_SIZE');
      });

      // When attention is requested, both color and size show attention helpers
      studioCtx!.setShowReadinessAttention(true);

      await waitFor(() => {
        expect(screen.getByTestId('color-required-helper').textContent).toBe('Выберите цвет');
        expect(screen.getByTestId('size-required-helper').textContent).toBe('Выберите размер');
      });

      // Add color -> color attention clears
      studioCtx!.updateDraft({
        colors: [{ id: 'col-black', name: 'Чёрный', hex: '#000000' }],
      });

      await waitFor(() => {
        expect(screen.queryByTestId('color-required-helper')).toBeNull();
        // Size still needs attention
        expect(screen.getByTestId('size-required-helper')).toBeTruthy();
      });

      // Add variant with size -> size attention clears
      studioCtx!.updateDraft({
        variants: [
          {
            id: 'v-1',
            colorId: 'col-black',
            colorName: 'Чёрный',
            sizeValueId: 'sz-m',
            size: 'M',
            priceCents: 1000,
          },
        ],
      });

      await waitFor(() => {
        expect(screen.queryByTestId('size-required-helper')).toBeNull();
      });
    });

    it('dynamic schema SINGLE_VARIANT keeps color and size non-red', async () => {
      let studioCtx: ReturnType<typeof useProductStudio> | null = null;
      function ContextWatcher() {
        studioCtx = useProductStudio();
        return null;
      }

      render(
        <MemoryRouter>
          <ProductStudioProvider entryMode="create">
            <ContextWatcher />
            <ProductStudio />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      studioCtx!.updateDraft({
        categoryId: 'cat-single',
        categoryName: 'Книга',
      });

      await waitFor(() => {
        expect(studioCtx?.categorySchema?.dimensionType).toBe('SINGLE_VARIANT');
      });

      expect(screen.queryByTestId('color-required-helper')).toBeNull();
      expect(screen.queryByTestId('size-required-helper')).toBeNull();
    });

    it('form workspace reflects calm neutral state on clean draft, and attention helpers when attention is active', async () => {
      let studioCtx: ReturnType<typeof useProductStudio> | null = null;
      function ContextWatcher() {
        studioCtx = useProductStudio();
        return null;
      }

      render(
        <MemoryRouter>
          <ProductStudioProvider entryMode="create">
            <ContextWatcher />
            <ProductStudioFormWorkspace />
          </ProductStudioProvider>
        </MemoryRouter>
      );

      // Clean draft: helpers NOT visible
      expect(screen.queryByText('Укажите название')).toBeNull();
      expect(screen.queryByText('Выберите категорию')).toBeNull();

      // When attention is active
      studioCtx!.setShowReadinessAttention(true);

      await waitFor(() => {
        expect(screen.getByText('Укажите название')).toBeTruthy();
        expect(screen.getByText('Выберите категорию')).toBeTruthy();
      });
    });
  });
});
