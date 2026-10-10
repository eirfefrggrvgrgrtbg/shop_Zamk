/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import {
  ProductStudioProvider,
  useProductStudio,
  type ProductStudioDraft,
} from '../../contexts/ProductStudioContext';
import { ProductStudioHeader } from './ProductStudioHeader';
import { createCanonicalProductStudioImage } from './productStudioMediaHelper';

afterEach(() => {
  cleanup();
});

beforeEach(() => {
  vi.clearAllMocks();
});

const completeHoodieDraft: ProductStudioDraft = {
  id: 'prod-hoodie-real-1',
  title: 'Худи черное оверсайз',
  description: 'Качественное худи из плотного футера с начесом',
  categoryId: 'cat-hoodies',
  categoryName: 'Худи',
  status: 'draft',
  priceCents: 490000,
  currency: 'RUB',
  material: 'Хлопок',
  materialComposition: [{ materialName: 'Хлопок', percentage: 100 }],
  images: [
    createCanonicalProductStudioImage({ imageId: 'img-1', url: 'https://example.com/1.jpg', cropWidth: 1, cropHeight: 1, isMain: true }),
    createCanonicalProductStudioImage({ imageId: 'img-2', url: 'https://example.com/2.jpg', cropWidth: 1, cropHeight: 1 }),
    createCanonicalProductStudioImage({ imageId: 'img-3', url: 'https://example.com/3.jpg', cropWidth: 1, cropHeight: 1 }),
  ],
  colors: [{ id: 'col-black', name: 'Черный', hex: '#000000' }],
  variants: [
    {
      id: 'var-1',
      colorId: 'col-black',
      colorName: 'Черный',
      sizeValueId: 'sz-m',
      size: 'M',
      sellerSku: 'HOODIE-BLK-M',
      priceCents: 490000,
      isActive: true,
    },
  ],
};

describe('ProductStudio Submit to Moderation Eligibility', () => {
  it('1. Complete draft with no dirty changes: Save is disabled, Submit to Moderation is ENABLED', async () => {
    const mockSubmit = vi.fn().mockResolvedValue(undefined);

    render(
      <MemoryRouter>
        <ProductStudioProvider
          entryMode="edit"
          initialDraft={completeHoodieDraft}
          submitModerationFn={mockSubmit}
        >
          <ProductStudioHeader />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    const saveBtn = screen.getByTestId('studio-header-save-btn');
    const submitBtn = screen.getByTestId('studio-header-submit-moderation-btn');

    // Save disabled because there are no dirty changes
    expect(saveBtn.hasAttribute('disabled')).toBe(true);
    expect(saveBtn.getAttribute('title')).toBe('Нет несохраненных изменений');

    // Submit to moderation ENABLED
    expect(submitBtn.hasAttribute('disabled')).toBe(false);
    expect(submitBtn.textContent).toContain('Отправить на модерацию');

    // Clicking submit triggers the submission flow
    await act(async () => {
      fireEvent.click(submitBtn);
    });

    expect(mockSubmit).toHaveBeenCalledTimes(1);
    expect(mockSubmit).toHaveBeenCalledWith('prod-hoodie-real-1');

    // After success, status changes to pending_moderation -> button becomes disabled
    await waitFor(() => {
      expect(screen.getByTestId('studio-entry-badge').textContent).toBe('На модерации');
      expect(submitBtn.hasAttribute('disabled')).toBe(true);
    });
  });

  it('2. Incomplete card: Submit to Moderation is disabled', () => {
    // Incomplete: only 2 images instead of 3
    const incompleteDraft: ProductStudioDraft = {
      ...completeHoodieDraft,
      images: [
        createCanonicalProductStudioImage({ imageId: 'img-1', url: 'https://example.com/1.jpg', cropWidth: 1, cropHeight: 1, isMain: true }),
        createCanonicalProductStudioImage({ imageId: 'img-2', url: 'https://example.com/2.jpg', cropWidth: 1, cropHeight: 1 }),
      ],
    };

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={incompleteDraft}>
          <ProductStudioHeader />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    const submitBtn = screen.getByTestId('studio-header-submit-moderation-btn');
    expect(submitBtn.hasAttribute('disabled')).toBe(true);
    expect(submitBtn.getAttribute('title')).toBe('Заполните все обязательные поля для отправки на модерацию');
  });

  it('3. Submitting state: Submit is disabled during in-flight request and shows loading indicator', async () => {
    let resolveSubmit: () => void = () => {};
    const submitPromise = new Promise<void>((resolve) => {
      resolveSubmit = resolve;
    });
    const mockSubmit = vi.fn().mockImplementation(() => submitPromise);

    render(
      <MemoryRouter>
        <ProductStudioProvider
          entryMode="edit"
          initialDraft={completeHoodieDraft}
          submitModerationFn={mockSubmit}
        >
          <ProductStudioHeader />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    const submitBtn = screen.getByTestId('studio-header-submit-moderation-btn');
    expect(submitBtn.hasAttribute('disabled')).toBe(false);

    // Fire click
    act(() => {
      fireEvent.click(submitBtn);
    });

    // In flight: button is disabled, label shows "Отправка…"
    expect(submitBtn.hasAttribute('disabled')).toBe(true);
    expect(submitBtn.textContent).toContain('Отправка…');

    // Resolve in flight promise
    await act(async () => {
      resolveSubmit();
    });

    await waitFor(() => {
      expect(submitBtn.hasAttribute('disabled')).toBe(true);
    });
  });

  it('4. Non-eligible product status: Submit to Moderation is disabled for pending_moderation and published', () => {
    const pendingDraft: ProductStudioDraft = {
      ...completeHoodieDraft,
      status: 'pending_moderation',
    };

    const { unmount } = render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={pendingDraft}>
          <ProductStudioHeader />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    const submitBtn = screen.getByTestId('studio-header-submit-moderation-btn');
    expect(submitBtn.hasAttribute('disabled')).toBe(true);
    expect(submitBtn.getAttribute('title')).toBe('Товар уже находится на модерации');

    unmount();

    const publishedDraft: ProductStudioDraft = {
      ...completeHoodieDraft,
      status: 'published',
    };

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={publishedDraft}>
          <ProductStudioHeader />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    const submitBtnPublished = screen.getByTestId('studio-header-submit-moderation-btn');
    expect(submitBtnPublished.hasAttribute('disabled')).toBe(true);
    expect(submitBtnPublished.getAttribute('title')).toBe('Отправка на модерацию недоступна для этого статуса товара');
  });

  it('5. Dirty draft: Save is enabled, Submit to Moderation is disabled until changes are saved', async () => {
    function Mutator() {
      const { updateDraft } = useProductStudio();
      return (
        <button
          data-testid="test-mutate-btn"
          onClick={() => updateDraft({ title: 'Худи черное оверсайз модифицированное' })}
        >
          Mutate
        </button>
      );
    }

    render(
      <MemoryRouter>
        <ProductStudioProvider entryMode="edit" initialDraft={completeHoodieDraft}>
          <ProductStudioHeader />
          <Mutator />
        </ProductStudioProvider>
      </MemoryRouter>
    );

    const saveBtn = screen.getByTestId('studio-header-save-btn');
    const submitBtn = screen.getByTestId('studio-header-submit-moderation-btn');

    // Initially clean: Save disabled, Submit enabled
    expect(saveBtn.hasAttribute('disabled')).toBe(true);
    expect(submitBtn.hasAttribute('disabled')).toBe(false);

    // Mutate draft locally
    act(() => {
      fireEvent.click(screen.getByTestId('test-mutate-btn'));
    });

    // When dirty: Save enabled, Submit disabled
    expect(saveBtn.hasAttribute('disabled')).toBe(false);
    expect(submitBtn.hasAttribute('disabled')).toBe(true);
    expect(submitBtn.getAttribute('title')).toBe('Сохраните изменения перед отправкой на модерацию');
  });
});
