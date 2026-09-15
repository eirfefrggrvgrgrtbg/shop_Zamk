import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { AdminFreeScanner } from './AdminFreeScanner';
import { resolvePhysicalUnit, processFoundUnit, ResolvedPhysicalUnit } from '@zamk/api-client/src/admin';
import * as AdminAuthContext from '../contexts/AdminAuthContext';

vi.mock('@zamk/api-client/src/admin', () => ({
  resolvePhysicalUnit: vi.fn(),
  processFoundUnit: vi.fn(),
  finalizeSupplyReceivingSession: vi.fn(),
}));

vi.mock('../utils/audio', () => ({
  playBeepSound: vi.fn(),
}));

const mockExpectedUnit: ResolvedPhysicalUnit = {
  inventoryUnitId: 'unit-exp-123',
  unitCode: 'ZMU-BR8XJV54XCMX48ZZ',
  unitStatus: 'expected',
  recommendedAction: 'additional_receiving',
  product: {
    title: 'Awesome Jacket',
  },
  variant: {
    color: 'Black',
    size: 'L',
    sellerSku: 'SKU-JACKET-BLK-L',
    barcode: 'BAR-JACKET-123',
  },
  origin: {
    supplyId: 'sup-1',
    supplyNumber: 'SUP-001',
    supplyStatus: 'completed_with_discrepancies',
    supplyItemId: 'sup-item-1',
    boxNumber: 'BOX-01',
    sellerName: 'Fashion Seller',
  },
  receivingState: {},
};

const mockWarehouseUnit: ResolvedPhysicalUnit = {
  ...mockExpectedUnit,
  unitCode: 'ZMU-WAREHOUSE-001',
  unitStatus: 'warehouse',
  recommendedAction: 'already_in_warehouse',
};

const mockDamagedUnit: ResolvedPhysicalUnit = {
  ...mockExpectedUnit,
  unitCode: 'ZMU-DAMAGED-001',
  unitStatus: 'damaged',
  recommendedAction: 'already_damaged',
};

const mockShippedUnit: ResolvedPhysicalUnit = {
  ...mockExpectedUnit,
  unitCode: 'ZMU-SHIPPED-001',
  unitStatus: 'shipped',
  recommendedAction: 'already_shipped',
};

const mockWrittenOffUnit: ResolvedPhysicalUnit = {
  ...mockExpectedUnit,
  unitCode: 'ZMU-WRITTEN-OFF-001',
  unitStatus: 'written_off',
  recommendedAction: 'written_off',
};

const defaultAuthMock = {
  hasPermission: (p: string) => p === 'inventory.receipt' || p === 'inventory.read',
  user: null,
  isAuthenticated: true,
  isLoading: false,
  hasAnyPermission: () => true,
  staff: null,
  login: vi.fn(),
  logout: vi.fn(),
};

describe('AdminFreeScanner routing and prefill contract', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(AdminAuthContext, 'useAdminAuth').mockReturnValue(defaultAuthMock as any);
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

describe('WH.1B Free Scanner Non-Mutating Resolve and Explicit Action', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(AdminAuthContext, 'useAdminAuth').mockReturnValue(defaultAuthMock as any);
  });

  it('A & B: scanning ZMU performs resolve/read only and does NOT call processFoundUnit', async () => {
    vi.mocked(resolvePhysicalUnit).mockResolvedValueOnce(mockExpectedUnit);

    render(
      <MemoryRouter initialEntries={['/warehouse/free-scan']}>
        <AdminFreeScanner />
      </MemoryRouter>
    );

    const input = screen.getByPlaceholderText(/Отсканируйте ZMU/i) as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'ZMU-BR8XJV54XCMX48ZZ' } });
    fireEvent.submit(input.closest('form')!);

    await waitFor(() => {
      expect(resolvePhysicalUnit).toHaveBeenCalledWith('ZMU-BR8XJV54XCMX48ZZ');
    });

    // CRITICAL: processFoundUnit must NOT have been called during scan
    expect(processFoundUnit).not.toHaveBeenCalled();

    // Result card must be displayed
    expect(await screen.findByText('Awesome Jacket')).toBeDefined();
    expect(screen.getByText('ZMU-BR8XJV54XCMX48ZZ')).toBeDefined();
    expect(screen.getByText('SKU-JACKET-BLK-L')).toBeDefined();
    expect(screen.getAllByText('SUP-001').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('Fashion Seller')).toBeDefined();
    expect(screen.getByTestId('unit-status-badge').textContent).toContain('Ожидается на приёмке');
  });

  it('C & E: user with inventory.read only can resolve, but "Допринять на склад" button is disabled', async () => {
    vi.spyOn(AdminAuthContext, 'useAdminAuth').mockReturnValue({
      hasPermission: (p: string) => p === 'inventory.read',
      user: null,
      isAuthenticated: true,
      isLoading: false,
      hasAnyPermission: () => false,
      staff: null,
      login: vi.fn(),
      logout: vi.fn(),
    } as any);

    vi.mocked(resolvePhysicalUnit).mockResolvedValueOnce(mockExpectedUnit);

    render(
      <MemoryRouter initialEntries={['/warehouse/free-scan']}>
        <AdminFreeScanner />
      </MemoryRouter>
    );

    const input = screen.getByPlaceholderText(/Отсканируйте ZMU/i) as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'ZMU-BR8XJV54XCMX48ZZ' } });
    fireEvent.submit(input.closest('form')!);

    await waitFor(() => {
      expect(resolvePhysicalUnit).toHaveBeenCalledWith('ZMU-BR8XJV54XCMX48ZZ');
    });

    const receiveBtn = (await screen.findByRole('button', { name: /Допринять на склад/i })) as HTMLButtonElement;
    expect(receiveBtn).toBeDefined();
    expect(receiveBtn.disabled).toBe(true);
    expect(screen.getByText(/Требуются права inventory.receipt/i)).toBeDefined();

    // No mutation
    expect(processFoundUnit).not.toHaveBeenCalled();
  });

  it('F & G: user with inventory.receipt can click "Допринять на склад", calling processFoundUnit and refreshing state', async () => {
    vi.mocked(resolvePhysicalUnit)
      .mockResolvedValueOnce(mockExpectedUnit)
      .mockResolvedValueOnce(mockWarehouseUnit);

    vi.mocked(processFoundUnit).mockResolvedValueOnce({
      unitCode: mockExpectedUnit.unitCode,
      inventoryUnitId: mockExpectedUnit.inventoryUnitId,
      supplyId: mockExpectedUnit.origin.supplyId,
      supplyNumber: mockExpectedUnit.origin.supplyNumber,
      unitStatus: 'warehouse',
      recommendedNextAction: 'already_in_warehouse',
      sessionExpected: 1,
      sessionScanned: 1,
      sessionOk: 1,
      sessionDamaged: 0,
      sessionRemaining: 0,
    });

    render(
      <MemoryRouter initialEntries={['/warehouse/free-scan']}>
        <AdminFreeScanner />
      </MemoryRouter>
    );

    const input = screen.getByPlaceholderText(/Отсканируйте ZMU/i) as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'ZMU-BR8XJV54XCMX48ZZ' } });
    fireEvent.submit(input.closest('form')!);

    const receiveBtn = (await screen.findByRole('button', { name: /Допринять на склад/i })) as HTMLButtonElement;
    expect(receiveBtn.disabled).toBe(false);

    fireEvent.click(receiveBtn);

    await waitFor(() => {
      expect(processFoundUnit).toHaveBeenCalledWith({
        unitCode: 'ZMU-BR8XJV54XCMX48ZZ',
        condition: 'ok',
      });
    });

    // Success feedback is displayed
    expect(await screen.findByText(/Единица успешно принята на склад/i)).toBeDefined();

    // Refreshed view shows "Уже на складе" and no longer has "Допринять на склад"
    await waitFor(() => {
      expect(screen.getByTestId('unit-status-badge').textContent).toContain('Уже на складе');
    });
    expect(screen.queryByRole('button', { name: /Допринять на склад/i })).toBeNull();
  });

  it('H: scanning warehouse unit displays information only and does NOT show receiving action', async () => {
    vi.mocked(resolvePhysicalUnit).mockResolvedValueOnce(mockWarehouseUnit);

    render(
      <MemoryRouter initialEntries={['/warehouse/free-scan']}>
        <AdminFreeScanner />
      </MemoryRouter>
    );

    const input = screen.getByPlaceholderText(/Отсканируйте ZMU/i) as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'ZMU-WAREHOUSE-001' } });
    fireEvent.submit(input.closest('form')!);

    await waitFor(() => {
      expect(resolvePhysicalUnit).toHaveBeenCalledWith('ZMU-WAREHOUSE-001');
    });

    const badge = await screen.findByTestId('unit-status-badge');
    expect(badge.textContent).toContain('Уже на складе');
    expect(screen.getByText(/Этот товар уже оприходован и находится на складе/i)).toBeDefined();
    expect(screen.queryByRole('button', { name: /Допринять на склад/i })).toBeNull();
    expect(processFoundUnit).not.toHaveBeenCalled();
  });

  it('I1: damaged state shows information only without receiving action', async () => {
    vi.mocked(resolvePhysicalUnit).mockResolvedValueOnce(mockDamagedUnit);

    render(
      <MemoryRouter initialEntries={['/warehouse/free-scan']}>
        <AdminFreeScanner />
      </MemoryRouter>
    );

    const input = screen.getByPlaceholderText(/Отсканируйте ZMU/i) as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'ZMU-DAMAGED-001' } });
    fireEvent.submit(input.closest('form')!);

    const badge = await screen.findByTestId('unit-status-badge');
    expect(badge.textContent).toMatch(/Повреждено \/ Брак/i);
    expect(screen.queryByRole('button', { name: /Допринять на склад/i })).toBeNull();
    expect(processFoundUnit).not.toHaveBeenCalled();
  });

  it('I2: shipped state shows information only without receiving action', async () => {
    vi.mocked(resolvePhysicalUnit).mockResolvedValueOnce(mockShippedUnit);

    render(
      <MemoryRouter initialEntries={['/warehouse/free-scan']}>
        <AdminFreeScanner />
      </MemoryRouter>
    );

    const input = screen.getByPlaceholderText(/Отсканируйте ZMU/i) as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'ZMU-SHIPPED-001' } });
    fireEvent.submit(input.closest('form')!);

    const badge = await screen.findByTestId('unit-status-badge');
    expect(badge.textContent).toContain('Отгружено');
    expect(screen.queryByRole('button', { name: /Допринять на склад/i })).toBeNull();
    expect(processFoundUnit).not.toHaveBeenCalled();
  });

  it('I3: written_off state shows information only without receiving action', async () => {
    vi.mocked(resolvePhysicalUnit).mockResolvedValueOnce(mockWrittenOffUnit);

    render(
      <MemoryRouter initialEntries={['/warehouse/free-scan']}>
        <AdminFreeScanner />
      </MemoryRouter>
    );

    const input = screen.getByPlaceholderText(/Отсканируйте ZMU/i) as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'ZMU-WRITTEN-OFF-001' } });
    fireEvent.submit(input.closest('form')!);

    const badge = await screen.findByTestId('unit-status-badge');
    expect(badge.textContent).toContain('Списано');
    expect(screen.queryByRole('button', { name: /Допринять на склад/i })).toBeNull();
    expect(processFoundUnit).not.toHaveBeenCalled();
  });

  it('J: unknown ZMU shows error and does not call processFoundUnit', async () => {
    vi.mocked(resolvePhysicalUnit).mockRejectedValueOnce({
      code: 'UNIT_NOT_FOUND',
      status: 404,
      message: 'Физическая единица с таким кодом не найдена.',
    });

    render(
      <MemoryRouter initialEntries={['/warehouse/free-scan']}>
        <AdminFreeScanner />
      </MemoryRouter>
    );

    const input = screen.getByPlaceholderText(/Отсканируйте ZMU/i) as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'ZMU-UNKNOWN999' } });
    fireEvent.submit(input.closest('form')!);

    await waitFor(() => {
      expect(resolvePhysicalUnit).toHaveBeenCalledWith('ZMU-UNKNOWN999');
    });

    expect(await screen.findByText(/Физическая единица с таким кодом не найдена/i)).toBeDefined();
    expect(processFoundUnit).not.toHaveBeenCalled();
    expect(screen.queryByRole('button', { name: /Допринять на склад/i })).toBeNull();
  });
});

describe('AdminFreeScanner Layout-Independent Normalization (SCN.1C)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(AdminAuthContext, 'useAdminAuth').mockReturnValue(defaultAuthMock as any);
  });

  it('L1: submits canonical normalized ZMU when scanned in Russian keyboard layout via Enter', async () => {
    vi.mocked(resolvePhysicalUnit).mockResolvedValueOnce(mockWarehouseUnit);

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
      expect(resolvePhysicalUnit).toHaveBeenCalledWith('ZMU-BR8XJV54XCMX48ZZ');
    });
    expect(processFoundUnit).not.toHaveBeenCalled();
  });

  it('L2: clicking submit button uses the exact same normalized value', async () => {
    vi.mocked(resolvePhysicalUnit).mockResolvedValueOnce(mockWarehouseUnit);

    render(
      <MemoryRouter initialEntries={['/warehouse/free-scan']}>
        <AdminFreeScanner />
      </MemoryRouter>
    );

    const input = screen.getByPlaceholderText(/Отсканируйте ZMU/i) as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'ЯЬГ-ИК8ЧОМ54ЧСЬЧ48ЯЯ' } });

    const submitBtn = screen.getByRole('button', { name: /Найти/i });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(resolvePhysicalUnit).toHaveBeenCalledWith('ZMU-BR8XJV54XCMX48ZZ');
    });
    expect(processFoundUnit).not.toHaveBeenCalled();
  });

  it('L3: leaves English layout input unchanged', async () => {
    vi.mocked(resolvePhysicalUnit).mockResolvedValueOnce(mockWarehouseUnit);

    render(
      <MemoryRouter initialEntries={['/warehouse/free-scan']}>
        <AdminFreeScanner />
      </MemoryRouter>
    );

    const input = screen.getByPlaceholderText(/Отсканируйте ZMU/i) as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'ZMU-BR8XJV54XCMX48ZZ' } });
    fireEvent.submit(input.closest('form')!);

    await waitFor(() => {
      expect(resolvePhysicalUnit).toHaveBeenCalledWith('ZMU-BR8XJV54XCMX48ZZ');
    });
    expect(processFoundUnit).not.toHaveBeenCalled();
  });
});
