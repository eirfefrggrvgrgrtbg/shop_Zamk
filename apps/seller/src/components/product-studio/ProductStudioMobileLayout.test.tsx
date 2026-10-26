/** @vitest-environment jsdom */
vi.mock('@zamk/api-client/src/seller', async (importOriginal: any) => {
  const actual = await importOriginal();
  return {
    ...actual,
    getSellerColors: vi.fn().mockResolvedValue([{ id: 'col-black', name: 'Чёрный', hexValue: '#000000' }]),
    getSellerSizeValues: vi.fn().mockResolvedValue([]),
  };
});

import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ProductStudioProvider, type ProductStudioDraft } from '../../contexts/ProductStudioContext';
import { ProductStudio } from './ProductStudio';

// SELLER.STUDIO.MOBILE.1. These are structure/class checks only: real widths are measured in the live Playwright run.

const draft: Partial<ProductStudioDraft> = {
  id: 'prod-1',
  title: 'Очень длинное название товара для проверки переноса и сокращения в шапке на узком экране',
  description: 'Хлопок',
  categoryId: 'cat-clothing',
  dimensionType: 'COLOR_AND_SIZE',
  colors: [{ id: 'col-black', name: 'Чёрный', hex: '#000000' }],
  hasPendingRevision: true,
  variants: [
    { id: 'var-m', colorId: 'col-black', colorName: 'Чёрный', sizeValueId: 'sz-m', size: 'M', isActive: true, totalStock: 3, reservedStock: 1 },
  ],
};

const renderStudio = (focusVariantId?: string) =>
  render(
    <MemoryRouter>
      <ProductStudioProvider entryMode="edit" initialDraft={draft}>
        <ProductStudio focusVariantId={focusVariantId} />
      </ProductStudioProvider>
    </MemoryRouter>
  );

describe('Product Studio — mobile layout structure', () => {
  afterEach(() => cleanup());

  it('header: back link, save status, submit button and view toggle all exist; the row wraps below lg', () => {
    renderStudio();
    const header = screen.getByTestId('product-studio-header');
    expect(screen.getByTestId('studio-back-to-products').getAttribute('href')).toBe('/products');
    expect(screen.getByTestId('studio-save-status')).toBeTruthy();
    expect(screen.getByTestId('studio-header-submit-moderation-btn')).toBeTruthy();
    expect(screen.getByTestId('studio-view-toggle-form')).toBeTruthy();
    expect(header.className).toContain('lg:sticky');
    expect(header.className).not.toMatch(/(^|\s)sticky(\s|$)/);
    expect(header.firstElementChild!.className).toContain('flex-wrap');
    expect(header.firstElementChild!.className).toContain('lg:flex-nowrap');
  });

  it('long title is truncated with the full text kept in the title attribute; badges may wrap', () => {
    renderStudio();
    const h1 = screen.getByTestId('studio-product-title');
    expect(h1.className).toContain('truncate');
    expect(h1.getAttribute('title')).toBe(draft.title);
    expect(screen.getByTestId('studio-pending-revision-badge').parentElement!.className).toContain('flex-wrap');
    expect(screen.getByTestId('studio-pending-revision-badge').parentElement!.className).not.toContain('shrink-0');
  });

  it('section tabs: a scrollable tablist, one selected tab, switching works and the panel follows', () => {
    renderStudio();
    fireEvent.click(screen.getByTestId('studio-view-toggle-form'));
    const nav = screen.getByTestId('product-studio-section-nav');
    expect(nav.getAttribute('role')).toBe('tablist');
    expect(nav.className).toContain('overflow-x-auto');
    const tabs = nav.querySelectorAll('[role=tab]');
    expect(tabs.length).toBe(6);
    expect(nav.querySelectorAll('[aria-selected=true]').length).toBe(1);
    fireEvent.click(screen.getByTestId('studio-section-btn-variants'));
    expect(screen.getByTestId('studio-section-btn-variants').getAttribute('aria-selected')).toBe('true');
    expect(screen.getByTestId('studio-section-panel-variants')).toBeTruthy();
  });

  it('matrix stays editable: cell toggles are real buttons and the colour column stays pinned while the matrix scrolls', () => {
    renderStudio();
    fireEvent.click(screen.getByTestId('studio-view-toggle-form'));
    fireEvent.click(screen.getByTestId('studio-section-btn-variants'));
    const cell = screen.getByTestId('form-matrix-cell-col-black-sz-m');
    expect(cell.tagName).toBe('BUTTON');
    const row = cell.closest('tr')!;
    expect(row.firstElementChild!.className).toContain('sticky');
    expect(screen.getByTestId('form-matrix-editor-section').querySelector('.overflow-x-auto')).toBeTruthy();
  });

  it('SKU workflow intact: the variant banner shows read-only warehouse numbers and no stock input', () => {
    renderStudio('var-m');
    const banner = screen.getByTestId('studio-focused-variant');
    expect(banner.textContent).toContain('Чёрный / M');
    expect(screen.getByTestId('studio-focused-variant-stock').textContent).toContain('3 шт.');
    expect(screen.getByTestId('studio-section-panel-variants').querySelectorAll('input[name*="stock" i]').length).toBe(0);
  });
});
