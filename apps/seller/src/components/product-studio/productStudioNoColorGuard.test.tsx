/* @vitest-environment jsdom */
import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ProductStudioProvider, useProductStudio } from '../../contexts/ProductStudioContext';
import { ProductStudioVisualWorkspace } from './ProductStudioVisualWorkspace';
import {
  deriveProductStudioMediaMode,
  createCanonicalProductStudioImage,
  reconcileMediaOnColorRemoval,
} from './productStudioMediaHelper';
import {
  getProductStudioMediaSaveBlockReason,
  isProductStudioSaveEligible,
} from './productStudioSaveProduct';
import type { ProductStudioDraft, ProductStudioImage } from '../../contexts/ProductStudioContext';

function StudioStateInspector({ onState }: { onState: (state: any) => void }) {
  const ctx = useProductStudio();
  onState(ctx);
  return <div data-testid="state-inspector" />;
}

describe('SELLER MEDIA.2B1 — No-Color Guard & Zero-Image State (Focused Tests A-F)', () => {
  const sampleColors = [
    { id: 'col-black', code: 'BLK', nameRu: 'Черный', hex: '#000000' },
    { id: 'col-white', code: 'WHT', nameRu: 'Белый', hex: '#ffffff' },
  ];

  // A. No colors => GENERAL available
  it('A. No colors => GENERAL available', () => {
    const draft: Partial<ProductStudioDraft> = {
      title: 'Сумка',
      priceCents: 300000,
      mediaMode: 'GENERAL',
      colors: [],
      images: [
        createCanonicalProductStudioImage({ imageId: 'img-1', url: 'https://img/1', colorId: null, isMain: true }),
      ],
    };

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={draft}>
          <ProductStudioVisualWorkspace />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    const generalBtn = screen.getByTestId('media-mode-general-btn');
    expect(generalBtn).toBeTruthy();
    expect(generalBtn.hasAttribute('disabled')).toBe(false);
    expect(generalBtn.textContent).toContain('Общая галерея');
  });

  // B. No colors => COLORWAY cannot be selected
  it('B. No colors => COLORWAY cannot be selected', () => {
    let capturedCtx: any = null;
    const draft: Partial<ProductStudioDraft> = {
      title: 'Сумка',
      priceCents: 300000,
      mediaMode: 'GENERAL',
      colors: [],
      images: [
        createCanonicalProductStudioImage({ imageId: 'img-1', url: 'https://img/1', colorId: null, isMain: true }),
      ],
    };

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={draft}>
          <ProductStudioVisualWorkspace />
          <StudioStateInspector onState={(ctx) => (capturedCtx = ctx)} />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    const colorwayBtn = screen.getByTestId('media-mode-colorway-btn');
    expect(colorwayBtn).toBeTruthy();
    expect(colorwayBtn.hasAttribute('disabled')).toBe(true);
    expect(colorwayBtn.getAttribute('title')).toBe(
      'Добавьте цвет товара, чтобы использовать фотографии по цветам.'
    );

    // Clicking does not switch mode
    fireEvent.click(colorwayBtn);
    expect(capturedCtx.draft.mediaMode).toBe('GENERAL');
  });

  // C. No colors + generic photos => draft save allowed
  it('C. No colors + generic photos => draft save allowed', () => {
    const draft: ProductStudioDraft = {
      id: 'prod-no-colors',
      title: 'Сумка кожаная',
      description: 'Описание сумки',
      priceCents: 300000,
      mediaMode: 'GENERAL',
      colors: [],
      images: [
        createCanonicalProductStudioImage({ imageId: 'img-1', url: 'https://img/1', colorId: null, isMain: true }),
        createCanonicalProductStudioImage({ imageId: 'img-2', url: 'https://img/2', colorId: null }),
      ],
      variants: [{ id: 'v1', sizeValueId: 's1', sellerSku: 'SKU-BAG' }],
    };

    expect(getProductStudioMediaSaveBlockReason(draft)).toBeNull();
    expect(isProductStudioSaveEligible(draft)).toBe(true);
  });

  // D. 0 images + colors => default GENERAL
  it('D. 0 images + colors => default GENERAL', () => {
    const mode = deriveProductStudioMediaMode([]);
    expect(mode).toBe('GENERAL');

    let capturedCtx: any = null;
    const draft: Partial<ProductStudioDraft> = {
      title: 'Футболка',
      priceCents: 150000,
      colors: sampleColors,
      images: [],
    };

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={draft}>
          <ProductStudioVisualWorkspace />
          <StudioStateInspector onState={(ctx) => (capturedCtx = ctx)} />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    expect(capturedCtx.draft.mediaMode).toBe('GENERAL');
    expect(getProductStudioMediaSaveBlockReason(capturedCtx.draft)).toBeNull();
    expect(isProductStudioSaveEligible(capturedCtx.draft)).toBe(true);
  });

  // E. 0 images + colors => Seller may explicitly select COLORWAY
  it('E. 0 images + colors => Seller may explicitly select COLORWAY', async () => {
    let capturedCtx: any = null;
    const draft: Partial<ProductStudioDraft> = {
      title: 'Футболка',
      priceCents: 150000,
      colors: sampleColors,
      images: [],
    };

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="create" initialDraft={draft}>
          <ProductStudioVisualWorkspace />
          <StudioStateInspector onState={(ctx) => (capturedCtx = ctx)} />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    const colorwayBtn = screen.getByTestId('media-mode-colorway-btn');
    expect(colorwayBtn.hasAttribute('disabled')).toBe(false);

    // Since images is empty, clicking directly switches to COLORWAY without modal
    fireEvent.click(colorwayBtn);

    await waitFor(() => {
      expect(capturedCtx.draft.mediaMode).toBe('COLORWAY');
      expect(screen.queryByTestId('confirm-to-colorway-modal')).toBeNull();
    });

    // Draft save is still allowed with 0 images in COLORWAY
    expect(getProductStudioMediaSaveBlockReason(capturedCtx.draft)).toBeNull();
    expect(isProductStudioSaveEligible(capturedCtx.draft)).toBe(true);
  });

  // F. COLORWAY -> remove last color => affected photos UNASSIGNED => save blocked
  it('F. COLORWAY -> remove last color => affected photos UNASSIGNED => save blocked', () => {
    const initialImages: ProductStudioImage[] = [
      createCanonicalProductStudioImage({ imageId: 'img-1', url: 'https://img/1', colorId: 'col-black', isMain: true }),
      createCanonicalProductStudioImage({ imageId: 'img-2', url: 'https://img/2', colorId: 'col-black' }),
    ];

    // Remove all colors (activeColorIds = empty set)
    const activeColorIds = new Set<string>();
    const reconciled = reconcileMediaOnColorRemoval(initialImages, activeColorIds, 'COLORWAY');

    expect(reconciled[0].colorId).toBeNull();
    expect(reconciled[0].isUnassigned).toBe(true);
    expect(reconciled[1].colorId).toBeNull();
    expect(reconciled[1].isUnassigned).toBe(true);

    const draftWithRemovedColors: ProductStudioDraft = {
      id: 'prod-colorway',
      title: 'Товар без цветов',
      description: 'Описание',
      mediaMode: 'COLORWAY',
      colors: [],
      images: reconciled,
      variants: [],
    };

    const reason = getProductStudioMediaSaveBlockReason(draftWithRemovedColors);
    expect(reason).toBe('Распределите все фотографии по цветам перед сохранением');
    expect(isProductStudioSaveEligible(draftWithRemovedColors)).toBe(false);
  });
});
