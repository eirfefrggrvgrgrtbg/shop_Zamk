/* @vitest-environment jsdom */
import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ProductStudioProvider } from '../../contexts/ProductStudioContext';
import { ProductStudioVisualWorkspace } from './ProductStudioVisualWorkspace';
import type { ProductStudioDraft } from '../../contexts/ProductStudioContext';

describe('SELLER MEDIA.2B — Visual Workspace Media Mode UI & Modals', () => {
  const sampleColors = [
    { id: 'col-black', code: 'BLK', nameRu: 'Черный', hex: '#000000' },
    { id: 'col-white', code: 'WHT', nameRu: 'Белый', hex: '#ffffff' },
  ];

  it('1. Renders media mode switcher with GENERAL active by default on generic draft', () => {
    const draft: Partial<ProductStudioDraft> = {
      title: 'Худи',
      priceCents: 500000,
      mediaMode: 'GENERAL',
      colors: sampleColors,
      images: [
        {
          uiKey: 'img-1',
          isMain: true,
          sortOrder: 0,
          colorId: null,
          source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' },
        },
      ],
      variants: [{ id: 'v1', colorId: 'col-black', sizeValueId: 's1', sellerSku: 'SKU1' }],
    };

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={draft}>
          <ProductStudioVisualWorkspace />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    const switcher = screen.getByTestId('media-mode-switcher');
    expect(switcher).toBeTruthy();

    const generalBtn = screen.getByTestId('media-mode-general-btn');
    const colorwayBtn = screen.getByTestId('media-mode-colorway-btn');
    expect(generalBtn.textContent).toContain('Общая галерея');
    expect(colorwayBtn.textContent).toContain('Фотографии по цветам');
  });

  it('2. Switching GENERAL -> COLORWAY opens confirmation modal and confirms transition', async () => {
    const draft: Partial<ProductStudioDraft> = {
      title: 'Худи',
      priceCents: 500000,
      mediaMode: 'GENERAL',
      colors: sampleColors,
      images: [
        {
          uiKey: 'img-1',
          isMain: true,
          sortOrder: 0,
          colorId: null,
          source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' },
        },
      ],
      variants: [{ id: 'v1', colorId: 'col-black', sizeValueId: 's1', sellerSku: 'SKU1' }],
    };

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={draft}>
          <ProductStudioVisualWorkspace />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('media-mode-colorway-btn'));

    // Modal appears
    const modal = screen.getByTestId('confirm-to-colorway-modal');
    expect(modal).toBeTruthy();
    expect(modal.textContent).toContain('Переключить на фотографии по цветам?');
    expect(modal.textContent).toContain('Фотографии нужно будет распределить по цветам.');

    // Confirm transition
    fireEvent.click(screen.getByTestId('confirm-to-colorway-submit'));

    await waitFor(() => {
      expect(screen.queryByTestId('confirm-to-colorway-modal')).toBeNull();
      // Unassigned banner appears because photo became unassigned
      expect(screen.getByTestId('unassigned-photos-banner')).toBeTruthy();
      expect(screen.getByTestId('thumbnail-unassigned-dot-0')).toBeTruthy();
    });
  });

  it('3. Switching COLORWAY -> GENERAL opens confirmation modal and confirms transition', async () => {
    const draft: Partial<ProductStudioDraft> = {
      title: 'Худи',
      priceCents: 500000,
      mediaMode: 'COLORWAY',
      colors: sampleColors,
      images: [
        {
          uiKey: 'img-1',
          isMain: true,
          sortOrder: 0,
          colorId: 'col-black',
          source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' },
        },
      ],
      variants: [{ id: 'v1', colorId: 'col-black', sizeValueId: 's1', sellerSku: 'SKU1' }],
    };

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={draft}>
          <ProductStudioVisualWorkspace />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    fireEvent.click(screen.getByTestId('media-mode-general-btn'));

    // Modal appears
    const modal = screen.getByTestId('confirm-to-general-modal');
    expect(modal).toBeTruthy();
    expect(modal.textContent).toContain('Переключить на общую галерею?');
    expect(modal.textContent).toContain('Фотографии разных цветов будут объединены в одну общую галерею.');

    // Confirm transition
    fireEvent.click(screen.getByTestId('confirm-to-general-submit'));

    await waitFor(() => {
      expect(screen.queryByTestId('confirm-to-general-modal')).toBeNull();
      // No unassigned banner in GENERAL
      expect(screen.queryByTestId('unassigned-photos-banner')).toBeNull();
      // Color dot should be absent in GENERAL
      expect(screen.queryByTestId('thumbnail-color-dot-0')).toBeNull();
    });
  });

  it('4. LEGACY_MIXED renders persistent banner with actions A and B', async () => {
    const draft: Partial<ProductStudioDraft> = {
      title: 'Mixed Product',
      priceCents: 500000,
      mediaMode: 'LEGACY_MIXED',
      colors: sampleColors,
      images: [
        {
          uiKey: 'img-1',
          isMain: true,
          sortOrder: 0,
          colorId: 'col-black',
          source: { kind: 'canonical', imageId: 'img-1', url: 'https://img/1' },
        },
        {
          uiKey: 'img-2',
          isMain: false,
          sortOrder: 1,
          colorId: null,
          source: { kind: 'canonical', imageId: 'img-2', url: 'https://img/2' },
        },
      ],
      variants: [{ id: 'v1', colorId: 'col-black', sizeValueId: 's1', sellerSku: 'SKU1' }],
    };

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={draft}>
          <ProductStudioVisualWorkspace />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    const banner = screen.getByTestId('legacy-mixed-banner');
    expect(banner).toBeTruthy();
    expect(banner.textContent).toContain('Фотографии товара нужно привести к одному режиму перед сохранением.');

    const resolveGenBtn = screen.getByTestId('resolve-to-general-btn');
    const resolveCwBtn = screen.getByTestId('resolve-to-colorway-btn');
    expect(resolveGenBtn.textContent).toContain('Объединить в общую галерею');
    expect(resolveCwBtn.textContent).toContain('Разложить по цветам');

    // Click Resolve to General
    fireEvent.click(resolveGenBtn);

    await waitFor(() => {
      expect(screen.queryByTestId('legacy-mixed-banner')).toBeNull();
      expect(screen.queryByTestId('unassigned-photos-banner')).toBeNull();
    });
  });
});
