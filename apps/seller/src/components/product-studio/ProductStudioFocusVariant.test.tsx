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
import { render, screen, cleanup } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ProductStudioProvider, type ProductStudioDraft } from '../../contexts/ProductStudioContext';
import { ProductStudio } from './ProductStudio';

const draft: Partial<ProductStudioDraft> = {
  id: 'prod-1',
  title: 'Футболка',
  description: 'Хлопок',
  categoryId: 'cat-clothing',
  dimensionType: 'COLOR_AND_SIZE',
  colors: [{ id: 'col-black', name: 'Чёрный', hex: '#000000' }],
  variants: [
    { id: 'var-m', colorId: 'col-black', colorName: 'Чёрный', sizeValueId: 'sz-m', size: 'M', isActive: true, totalStock: 3, reservedStock: 1 },
    { id: 'var-l', colorId: 'col-black', colorName: 'Чёрный', sizeValueId: 'sz-l', size: 'L', isActive: false, totalStock: 0, reservedStock: 0 },
  ],
};

const renderStudio = (focusVariantId?: string, focusVariantStored?: any) =>
  render(
    <MemoryRouter>
      <ProductStudioProvider entryMode="edit" initialDraft={draft}>
        <ProductStudio focusVariantId={focusVariantId} focusVariantStored={focusVariantStored} />
      </ProductStudioProvider>
    </MemoryRouter>
  );

describe('Product Studio — focus on a variant from the product list', () => {
  afterEach(() => cleanup());

  it('opens the variants section of the form and describes exactly the requested variant', () => {
    renderStudio('var-l');
    expect(screen.getByTestId('studio-section-panel-variants')).toBeTruthy();
    const banner = screen.getByTestId('studio-focused-variant');
    expect(banner.textContent).toContain('Чёрный / L');
    expect(banner.textContent).toContain('выключен');
    expect(banner.textContent).not.toContain('Чёрный / M');
  });

  it('shows read-only warehouse numbers and says the warehouse owns the stock', () => {
    renderStudio('var-m');
    const stock = screen.getByTestId('studio-focused-variant-stock');
    expect(stock.textContent).toContain('3 шт.');
    expect(stock.textContent).toContain('в резерве: 1 шт.');
    expect(stock.textContent).toContain('Остаток меняет только склад ZAMK');
    // the banner offers no stock input
    expect(screen.getByTestId('studio-focused-variant').querySelectorAll('input').length).toBe(0);
  });

  it('an unknown variant id opens no other variant', () => {
    renderStudio('var-of-another-product');
    const banner = screen.getByTestId('studio-focused-variant');
    expect(banner.textContent).toContain('не найден');
    expect(banner.textContent).not.toContain('Чёрный / M');
  });

  it('without a focus parameter the Studio opens as before', () => {
    renderStudio();
    expect(screen.queryByTestId('studio-focused-variant')).toBeNull();
    expect(screen.getByTestId('studio-visual-workspace')).toBeTruthy();
  });

  it('a switched-off variant that is not in the draft is described from its stored copy, only for its exact id', () => {
    const stored = { id: 'var-off', colorId: 'col-black', colorName: 'Чёрный', sizeValueId: 'sz-xl', size: 'XL', isActive: false, totalStock: 4, reservedStock: 0 };
    renderStudio('var-off', stored);
    const banner = screen.getByTestId('studio-focused-variant');
    expect(banner.textContent).toContain('Чёрный / XL');
    expect(banner.textContent).toContain('выключен');
    expect(screen.getByTestId('studio-focused-variant-stock').textContent).toContain('4 шт.');
    cleanup();
    renderStudio('another-id', stored);
    expect(screen.getByTestId('studio-focused-variant').textContent).toContain('не найден');
  });
});
