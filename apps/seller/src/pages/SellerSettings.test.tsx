/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { SellerSettings } from './SellerSettings';
import type { SellerMe } from '@zamk/api-client/src/types';

const mockGetSellerMe = vi.fn();
const mockUpdateSellerMe = vi.fn();
const mockUploadSellerLogo = vi.fn();
const mockUploadSellerCover = vi.fn();

vi.mock('@zamk/api-client/src/seller', () => ({
  getSellerMe: () => mockGetSellerMe(),
  updateSellerMe: (req: any) => mockUpdateSellerMe(req),
  uploadSellerLogo: (file: any) => mockUploadSellerLogo(file),
  uploadSellerCover: (file: any) => mockUploadSellerCover(file),
}));

const sampleSellerMe: SellerMe = {
  user: {
    id: 'u-1',
    email: 'atelier@example.com',
    role: 'seller',
  } as any,
  sellerUser: {
    id: 'su-1',
    sellerId: 's-1',
    userId: 'u-1',
    role: 'owner',
  },
  seller: {
    id: 's-1',
    brandName: 'Atelier Monochrome',
    slug: 'atelier-monochrome',
    description: 'Концептуальный минимализм и архитектурный крой.',
    history: 'Марка создана в 2022 году как лаборатория формы и фактуры.',
    contactEmail: 'atelier@example.com',
    contactPhone: '+7 (999) 000-11-22',
    logoUrl: 'https://cdn.zamk.test/atelier-logo.png',
    coverUrl: 'https://cdn.zamk.test/atelier-cover.png',
    status: 'active',
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
};

const renderSettings = () =>
  render(
    <MemoryRouter initialEntries={['/settings']}>
      <SellerSettings />
    </MemoryRouter>,
  );

describe('SellerSettings storefront management (MARKETING.M1.10)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  it('1. Loads and renders public storefront fields and shop preview link', async () => {
    mockGetSellerMe.mockResolvedValue(sampleSellerMe);

    renderSettings();

    expect(screen.getByText('Загрузка профиля магазина...')).toBeDefined();

    expect(await screen.findByRole('heading', { level: 2, name: 'Публичная витрина' })).toBeDefined();

    expect(screen.getByDisplayValue('Atelier Monochrome')).toBeDefined();
    expect(screen.getByDisplayValue('atelier-monochrome')).toBeDefined();
    expect(screen.getByDisplayValue('Концептуальный минимализм и архитектурный крой.')).toBeDefined();
    expect(screen.getByDisplayValue('Марка создана в 2022 году как лаборатория формы и фактуры.')).toBeDefined();

    // Check storefront preview link
    const previewLink = screen.getByRole('link', { name: /Открыть витрину в Shop/i });
    expect(previewLink.getAttribute('href')).toContain('/brand/atelier-monochrome');
  });

  it('2. Edits storefront profile and saves changes via updateSellerMe', async () => {
    mockGetSellerMe.mockResolvedValue(sampleSellerMe);
    mockUpdateSellerMe.mockResolvedValue({
      ...sampleSellerMe,
      seller: {
        ...sampleSellerMe.seller,
        brandName: 'Atelier Monochrome Studio',
        description: 'Обновленное описание.',
        history: 'Обновленная история марки.',
      },
    });

    renderSettings();

    await screen.findByRole('heading', { level: 2, name: 'Публичная витрина' });

    const nameInput = screen.getByDisplayValue('Atelier Monochrome');
    fireEvent.change(nameInput, { target: { value: 'Atelier Monochrome Studio' } });

    const descInput = screen.getByDisplayValue('Концептуальный минимализм и архитектурный крой.');
    fireEvent.change(descInput, { target: { value: 'Обновленное описание.' } });

    const historyInput = screen.getByDisplayValue('Марка создана в 2022 году как лаборатория формы и фактуры.');
    fireEvent.change(historyInput, { target: { value: 'Обновленная история марки.' } });

    const saveBtn = screen.getByRole('button', { name: 'Сохранить профиль' });
    fireEvent.click(saveBtn);

    await waitFor(() => {
      expect(mockUpdateSellerMe).toHaveBeenCalledWith(
        expect.objectContaining({
          brandName: 'Atelier Monochrome Studio',
          description: 'Обновленное описание.',
          history: 'Обновленная история марки.',
          logoUrl: 'https://cdn.zamk.test/atelier-logo.png',
          coverUrl: 'https://cdn.zamk.test/atelier-cover.png',
        }),
      );
    });

    expect(await screen.findByText('Профиль и публичная витрина сохранены')).toBeDefined();
  });

  it('3. Handles duplicate slug conflict error gracefully', async () => {
    mockGetSellerMe.mockResolvedValue(sampleSellerMe);
    mockUpdateSellerMe.mockRejectedValue({ status: 409 });

    renderSettings();

    await screen.findByRole('heading', { level: 2, name: 'Публичная витрина' });

    const saveBtn = screen.getByRole('button', { name: 'Сохранить профиль' });
    fireEvent.click(saveBtn);

    expect(await screen.findByText('Этот адрес магазина (slug) уже занят. Выберите другой.')).toBeDefined();
  });

  it('4. Allows removing logo and cover from storefront', async () => {
    mockGetSellerMe.mockResolvedValue(sampleSellerMe);
    mockUpdateSellerMe.mockResolvedValue(sampleSellerMe);

    renderSettings();

    await screen.findByRole('heading', { level: 2, name: 'Публичная витрина' });

    const removeLogoBtn = screen.getByTitle('Удалить логотип');
    fireEvent.click(removeLogoBtn);

    const removeCoverBtn = screen.getByTitle('Удалить обложку');
    fireEvent.click(removeCoverBtn);

    const saveBtn = screen.getByRole('button', { name: 'Сохранить профиль' });
    fireEvent.click(saveBtn);

    await waitFor(() => {
      expect(mockUpdateSellerMe).toHaveBeenCalledWith(
        expect.objectContaining({
          logoUrl: '',
          coverUrl: '',
        }),
      );
    });
  });
});
