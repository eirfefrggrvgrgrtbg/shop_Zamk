import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { AdminFreeScanner } from './AdminFreeScanner';

vi.mock('@zamk/api-client/src/admin', () => ({
  processFoundUnit: vi.fn(),
  finalizeSupplyReceivingSession: vi.fn(),
}));

describe('AdminFreeScanner routing and prefill contract', () => {
  it('initializes unit code from ?q= parameter', () => {
    render(
      <MemoryRouter initialEntries={['/warehouse/free-scan?q=ZMU-TEST12345']}>
        <AdminFreeScanner />
      </MemoryRouter>
    );

    const input = screen.getByPlaceholderText(/Отсканируйте ZMU/i) as HTMLInputElement;
    expect(input.value).toBe('ZMU-TEST12345');
  });

  it('initializes unit code from ?code= parameter', () => {
    render(
      <MemoryRouter initialEntries={['/warehouse/free-scan?code=ZMU-CODE67890']}>
        <AdminFreeScanner />
      </MemoryRouter>
    );

    const input = screen.getByPlaceholderText(/Отсканируйте ZMU/i) as HTMLInputElement;
    expect(input.value).toBe('ZMU-CODE67890');
  });

  it('initializes empty input when no query parameter is provided', () => {
    render(
      <MemoryRouter initialEntries={['/warehouse/free-scan']}>
        <AdminFreeScanner />
      </MemoryRouter>
    );

    const input = screen.getByPlaceholderText(/Отсканируйте ZMU/i) as HTMLInputElement;
    expect(input.value).toBe('');
  });
});
