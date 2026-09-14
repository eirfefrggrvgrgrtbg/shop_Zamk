import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { AdminFreeScanner } from './AdminFreeScanner';
import { processFoundUnit } from '@zamk/api-client/src/admin';

vi.mock('@zamk/api-client/src/admin', () => ({
  processFoundUnit: vi.fn(),
  finalizeSupplyReceivingSession: vi.fn(),
}));

vi.mock('../utils/audio', () => ({
  playBeepSound: vi.fn(),
}));

describe('AdminFreeScanner routing and prefill contract', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

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

describe('AdminFreeScanner Layout-Independent Normalization (SCN.1C)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('H & K: submits canonical normalized ZMU when scanned in Russian keyboard layout via Enter', async () => {
    vi.mocked(processFoundUnit).mockResolvedValueOnce({
      unitCode: 'ZMU-BR8XJV54XCMX48ZZ',
      unitStatus: 'warehouse',
      recommendedNextAction: 'already_in_warehouse',
      supplyNumber: 'SUP-001',
      sessionExpected: 1,
      sessionScanned: 1,
      sessionRemaining: 0,
    } as any);

    render(
      <MemoryRouter initialEntries={['/warehouse/free-scan']}>
        <AdminFreeScanner />
      </MemoryRouter>
    );

    const input = screen.getByPlaceholderText(/Отсканируйте ZMU/i) as HTMLInputElement;
    // Physical scanner transmits Russian characters when RU layout is active on macOS
    fireEvent.change(input, { target: { value: 'ЯЬГ-ИК8ЧОМ54ЧСЬЧ48ЯЯ' } });
    // Hardware scanner sends Enter key / form submit automatically
    fireEvent.submit(input.closest('form')!);

    await waitFor(() => {
      expect(processFoundUnit).toHaveBeenCalledWith({
        unitCode: 'ZMU-BR8XJV54XCMX48ZZ',
        condition: 'ok',
      });
    });
  });

  it('L: clicking submit button uses the exact same normalized value', async () => {
    vi.mocked(processFoundUnit).mockResolvedValueOnce({
      unitCode: 'ZMU-BR8XJV54XCMX48ZZ',
      unitStatus: 'warehouse',
      recommendedNextAction: 'already_in_warehouse',
      supplyNumber: 'SUP-001',
      sessionExpected: 1,
      sessionScanned: 1,
      sessionRemaining: 0,
    } as any);

    render(
      <MemoryRouter initialEntries={['/warehouse/free-scan']}>
        <AdminFreeScanner />
      </MemoryRouter>
    );

    const input = screen.getByPlaceholderText(/Отсканируйте ZMU/i) as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'ЯЬГ-ИК8ЧОМ54ЧСЬЧ48ЯЯ' } });

    const submitBtn = screen.getByRole('button', { name: /Принять/i });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(processFoundUnit).toHaveBeenCalledWith({
        unitCode: 'ZMU-BR8XJV54XCMX48ZZ',
        condition: 'ok',
      });
    });
  });

  it('M: leaves English layout input unchanged', async () => {
    vi.mocked(processFoundUnit).mockResolvedValueOnce({
      unitCode: 'ZMU-BR8XJV54XCMX48ZZ',
      unitStatus: 'warehouse',
      recommendedNextAction: 'already_in_warehouse',
      supplyNumber: 'SUP-001',
      sessionExpected: 1,
      sessionScanned: 1,
      sessionRemaining: 0,
    } as any);

    render(
      <MemoryRouter initialEntries={['/warehouse/free-scan']}>
        <AdminFreeScanner />
      </MemoryRouter>
    );

    const input = screen.getByPlaceholderText(/Отсканируйте ZMU/i) as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'ZMU-BR8XJV54XCMX48ZZ' } });
    fireEvent.submit(input.closest('form')!);

    await waitFor(() => {
      expect(processFoundUnit).toHaveBeenCalledWith({
        unitCode: 'ZMU-BR8XJV54XCMX48ZZ',
        condition: 'ok',
      });
    });
  });

  it('N: unknown normalized code reaches backend and triggers not-found behavior', async () => {
    vi.mocked(processFoundUnit).mockRejectedValueOnce({
      code: 'UNIT_NOT_FOUND',
      message: 'Физическая единица с таким кодом не найдена.',
      status: 404,
    });

    render(
      <MemoryRouter initialEntries={['/warehouse/free-scan']}>
        <AdminFreeScanner />
      </MemoryRouter>
    );

    const input = screen.getByPlaceholderText(/Отсканируйте ZMU/i) as HTMLInputElement;
    // Russian input for unknown unit ZMU-UNKNOWN99999 -> ЯЬГ-ГТЛТЩЦТ99999
    fireEvent.change(input, { target: { value: 'ЯЬГ-ГТЛТЩЦТ99999' } });
    fireEvent.submit(input.closest('form')!);

    await waitFor(() => {
      expect(processFoundUnit).toHaveBeenCalledWith({
        unitCode: 'ZMU-UNKNOWN99999',
        condition: 'ok',
      });
    });

    await waitFor(() => {
      expect(screen.getByText(/Физическая единица с таким кодом не найдена/i)).toBeDefined();
    });
  });
});
