/** @vitest-environment jsdom */
import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { ProductStudioProvider } from '../../contexts/ProductStudioContext';
import { ProductStudioHeader } from './ProductStudioHeader';

afterEach(() => {
  cleanup();
});

describe('ProductStudioHeader — Top Identity Block Layout Normalization', () => {
  it('renders Row 1 (title + status badges), Row 2 (category), Row 3 (brand) in clean vertical hierarchy', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider
          entryMode="edit"
          initialDraft={{
            id: 'prod-1',
            title: 'Классический хлопковый тренч',
            status: 'draft',
            categoryId: 'cat-coats',
            categoryName: 'Верхняя одежда / Тренчи',
            brandId: 'b-1',
            brandName: 'Atelier Monochrome',
          }}
        >
          <ProductStudioHeader />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    // Row 1: Title and badges
    const titleEl = screen.getByTestId('studio-product-title');
    expect(titleEl.textContent).toBe('Классический хлопковый тренч');
    expect(titleEl.className).toContain('truncate');
    expect(titleEl.className).toContain('min-w-0');

    const entryBadge = screen.getByTestId('studio-entry-badge');
    expect(entryBadge.textContent).toBe('Черновик');

    const readinessBtn = screen.getByTestId('studio-readiness-summary');
    expect(readinessBtn).toBeTruthy();

    // Verify Row 1 container structure: title and badge container are siblings in Row 1
    const row1Container = titleEl.parentElement;
    expect(row1Container).not.toBeNull();
    expect(row1Container?.className).toContain('flex');
    expect(row1Container?.className).toContain('items-center');
    expect(row1Container?.className).toContain('gap-2.5');
    expect(row1Container?.className).toContain('min-w-0');
    expect(row1Container?.contains(entryBadge)).toBe(true);
    expect(row1Container?.contains(readinessBtn)).toBe(true);

    // Row 2: Category chip on its own explicit row
    const catBtn = screen.getByTestId('studio-header-category-btn');
    expect(catBtn.textContent).toContain('Категория · Верхняя одежда / Тренчи');
    const row2Container = catBtn.parentElement;
    expect(row2Container).not.toBeNull();
    expect(row2Container).not.toBe(row1Container);
    expect(row2Container?.className).toContain('flex');
    expect(row2Container?.className).toContain('items-center');
    expect(row2Container?.className).toContain('min-w-0');

    // Row 3: Brand line on its own explicit row below category
    const brandEl = screen.getByTestId('studio-header-subtitle');
    expect(brandEl.textContent).toBe('Бренд: Atelier Monochrome');
    // Ensure brand is not hidden on any screen breakpoint
    expect(brandEl.className).not.toContain('hidden');
    expect(brandEl.className).toContain('truncate');
    const row3Container = brandEl.parentElement;
    expect(row3Container).not.toBeNull();
    expect(row3Container).not.toBe(row2Container);
    expect(row3Container).not.toBe(row1Container);

    // Identity block container encapsulates all 3 rows vertically
    const identityBlock = row1Container?.parentElement;
    expect(identityBlock?.className).toContain('flex-col');
    expect(identityBlock?.className).toContain('gap-1');
    expect(identityBlock?.className).toContain('min-w-0');
    expect(identityBlock?.children.length).toBe(3);
  });

  it('safely truncates long product titles with ellipsis without overflowing container', () => {
    const longTitle = 'Очень длинное пальто из кашемира с объемным поясом и декоративной строчкой ручной работы из новой коллекции 2026';
    render(
      <MemoryRouter>
        <ProductStudioProvider
          entryMode="edit"
          initialDraft={{
            id: 'prod-2',
            title: longTitle,
            status: 'draft',
            categoryId: 'cat-1',
            categoryName: 'Пальто',
            brandName: 'ZAMK Studio',
          }}
        >
          <ProductStudioHeader />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    const titleEl = screen.getByTestId('studio-product-title');
    expect(titleEl.getAttribute('title')).toBe(longTitle);
    expect(titleEl.className).toContain('truncate');
    expect(titleEl.className).toContain('min-w-0');
    expect(titleEl.className).toMatch(/max-w-\[/);

    // Badges remain adjacent and intact in Row 1
    const entryBadge = screen.getByTestId('studio-entry-badge');
    expect(entryBadge).toBeTruthy();
    const readinessBtn = screen.getByTestId('studio-readiness-summary');
    expect(readinessBtn).toBeTruthy();
  });

  it('omits Row 3 cleanly when brand is absent, preserving Row 1 and Row 2', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider
          entryMode="create"
          initialDraft={{
            title: 'Новая футболка',
            status: 'draft',
            categoryId: '',
          }}
        >
          <ProductStudioHeader />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    expect(screen.queryByTestId('studio-header-subtitle')).toBeNull();

    const titleEl = screen.getByTestId('studio-product-title');
    expect(titleEl.textContent).toBe('Новая футболка');

    const catBtn = screen.getByTestId('studio-header-category-btn');
    expect(catBtn.textContent).toContain('Категория * · Не выбрана');

    const identityBlock = titleEl.parentElement?.parentElement;
    expect(identityBlock?.className).toContain('flex-col');
    expect(identityBlock?.children.length).toBe(2);
  });

  it('ensures category button has truncation protection for long category paths', () => {
    render(
      <MemoryRouter>
        <ProductStudioProvider
          entryMode="edit"
          initialDraft={{
            id: 'prod-3',
            title: 'Кожаная куртка',
            categoryId: 'cat-deep',
            categoryName: 'Одежда / Женская одежда / Верхняя одежда / Куртки и парки / Кожаные куртки',
          }}
        >
          <ProductStudioHeader />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    const catBtn = screen.getByTestId('studio-header-category-btn');
    expect(catBtn.className).toContain('truncate');
    expect(catBtn.className).toContain('max-w-full');
    const labelSpan = catBtn.querySelector('span');
    expect(labelSpan?.className).toContain('truncate');
  });
});
