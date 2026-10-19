/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react';
import { ShareProductButton } from './ShareProductButton';

const mockShowToast = vi.fn();
vi.mock('../../contexts/ToastContext', () => ({
  useToast: () => ({ showToast: mockShowToast }),
}));

function setNav(props: Record<string, unknown>) {
  for (const [k, v] of Object.entries(props)) {
    Object.defineProperty(navigator, k, { value: v, configurable: true });
  }
}

describe('ShareProductButton', () => {
  beforeEach(() => {
    mockShowToast.mockClear();
    setNav({ share: undefined, clipboard: undefined });
  });
  afterEach(cleanup);

  it('has an accessible name', () => {
    render(<ShareProductButton title="Платье" />);
    expect(screen.getByRole('button', { name: 'Поделиться товаром' })).toBeTruthy();
  });

  it('uses Web Share API when available', async () => {
    const share = vi.fn().mockResolvedValue(undefined);
    setNav({ share });
    render(<ShareProductButton title="Платье" />);
    fireEvent.click(screen.getByRole('button'));
    await waitFor(() => expect(share).toHaveBeenCalledWith({ title: 'Платье', url: window.location.href }));
    expect(mockShowToast).not.toHaveBeenCalled();
  });

  it('ignores share cancellation silently', async () => {
    const share = vi.fn().mockRejectedValue(new DOMException('cancel', 'AbortError'));
    const writeText = vi.fn();
    setNav({ share, clipboard: { writeText } });
    render(<ShareProductButton title="Платье" />);
    fireEvent.click(screen.getByRole('button'));
    await waitFor(() => expect(share).toHaveBeenCalled());
    expect(writeText).not.toHaveBeenCalled();
    expect(mockShowToast).not.toHaveBeenCalled();
  });

  it('copies link and shows toast without Web Share', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    setNav({ clipboard: { writeText } });
    render(<ShareProductButton title="Платье" />);
    fireEvent.click(screen.getByRole('button'));
    await waitFor(() => expect(mockShowToast).toHaveBeenCalledWith('Ссылка скопирована'));
    expect(writeText).toHaveBeenCalledWith(window.location.href);
  });

  it('shows error toast when copy fails', async () => {
    setNav({ clipboard: { writeText: vi.fn().mockRejectedValue(new Error('denied')) } });
    render(<ShareProductButton title="Платье" />);
    fireEvent.click(screen.getByRole('button'));
    await waitFor(() =>
      expect(mockShowToast).toHaveBeenCalledWith('Не удалось скопировать ссылку', 'error'),
    );
  });
});
