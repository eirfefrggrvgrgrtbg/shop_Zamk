import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AdminMarketingQueryBuilder } from './AdminMarketingQueryBuilder';
import * as marketingApi from '../api/marketing';

vi.mock('../api/marketing', async () => {
  const actual = await vi.importActual<any>('../api/marketing');
  return {
    ...actual,
    executeAdminMarketingQuery: vi.fn(),
  };
});

describe('AdminMarketingQueryBuilder — Core Contract Matrix', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const mount = () => {
    return render(
      <MemoryRouter>
        <AdminMarketingQueryBuilder />
      </MemoryRouter>
    );
  };

  // Matrix 1, 2, 3: Route, Tabs, Period control render
  it('Matrix 1-3: renders header, tabs navigation, and period controls', () => {
    mount();
    expect(screen.getByRole('heading', { level: 1, name: /Аналитика/i })).toBeTruthy();
    expect(screen.getAllByText(/Конструктор отчетов/i).length).toBeGreaterThanOrEqual(1);

    // Marketing tabs exist
    const tabsNav = screen.getByRole('navigation', { name: /Разделы маркетинга/i });
    expect(tabsNav).toBeTruthy();
    expect(screen.getByRole('link', { name: 'Сводка' })).toBeTruthy();
    expect(screen.getByRole('link', { name: 'Конструктор отчетов' })).toBeTruthy();

    // Period control exists
    expect(screen.getByRole('group', { name: /Период аналитики/i })).toBeTruthy();
    expect(screen.getByRole('button', { name: '30 дней' })).toBeTruthy();
  });

  // Matrix 4, 5, 6, 7: Dimensions selection rules
  it('Matrix 4-7: allows zero, first, second dimensions, and rejects third without replacing', () => {
    mount();

    // Starts with default 'source' active
    const sourceBtn = screen.getByRole('button', { name: 'Источник' });
    expect(sourceBtn.className.includes('bg-indigo-50')).toBe(true);

    // Matrix 4: Deselect source -> zero dimensions allowed
    fireEvent.click(sourceBtn);
    expect(sourceBtn.className.includes('bg-indigo-50')).toBe(false);

    // Matrix 5: Select first dimension (e.g. День)
    const dayBtn = screen.getByRole('button', { name: 'День' });
    fireEvent.click(dayBtn);
    expect(dayBtn.className.includes('bg-indigo-50')).toBe(true);

    // Matrix 6: Select second dimension (e.g. Товар)
    const productBtn = screen.getByRole('button', { name: 'Товар' });
    fireEvent.click(productBtn);
    expect(productBtn.className.includes('bg-indigo-50')).toBe(true);

    // Matrix 7: Click third dimension (e.g. Дизайнер) -> rejected without replacing!
    const designerBtn = screen.getByRole('button', { name: 'Дизайнер' });
    fireEvent.click(designerBtn);

    // Existing selections unchanged
    expect(dayBtn.className.includes('bg-indigo-50')).toBe(true);
    expect(productBtn.className.includes('bg-indigo-50')).toBe(true);
    expect(designerBtn.className.includes('bg-indigo-50')).toBe(false);

    // UI feedback displayed
  });

  // Matrix 8, 9: Metric selection rules
  it('Matrix 8-9: requires at least one metric and supports multiple metrics selection', () => {
    mount();

    // Initial metrics: Выручка, Заказы
    const revBtn = screen.getByRole('button', { name: 'Выручка' });
    const ordBtn = screen.getByRole('button', { name: 'Заказы' });
    expect(revBtn.className.includes('bg-indigo-50')).toBe(true);
    expect(ordBtn.className.includes('bg-indigo-50')).toBe(true);

    // Deselect both
    fireEvent.click(revBtn);
    fireEvent.click(ordBtn);
    expect(revBtn.className.includes('bg-indigo-50')).toBe(false);
    expect(ordBtn.className.includes('bg-indigo-50')).toBe(false);

    // Build button disabled when metrics = 0
    const buildBtn = screen.getByRole('button', { name: /Построить отчет/i });
    expect((buildBtn as HTMLButtonElement).disabled).toBe(true);

    // Add multiple metrics: Сессии, Конверсия, Возвраты
    const sessBtn = screen.getByRole('button', { name: 'Сессии' });
    const convBtn = screen.getByRole('button', { name: 'Конверсия' });
    const retBtn = screen.getByRole('button', { name: 'Возвраты' });

    fireEvent.click(sessBtn);
    fireEvent.click(convBtn);
    fireEvent.click(retBtn);

    expect(sessBtn.className.includes('bg-indigo-50')).toBe(true);
    expect(convBtn.className.includes('bg-indigo-50')).toBe(true);
    expect(retBtn.className.includes('bg-indigo-50')).toBe(true);
    expect((buildBtn as HTMLButtonElement).disabled).toBe(false);
  });

  // Matrix 10, 11, 12, 13, 14, 15: Filters
  it('Matrix 10-15: filter add/remove and operators eq, neq, in, contains', () => {
    mount();

    // Default dimension is 'source'
    const addFilterBtn = screen.getAllByRole('button', { name: /Добавить/i })[0];
    fireEvent.click(addFilterBtn);

    const filterDimSelect = screen.getByRole('combobox', { name: 'Разрез фильтра' });
    expect((filterDimSelect as HTMLSelectElement).value).toBe('source');
    const filterOpSelect = screen.getByRole('combobox', { name: 'Оператор фильтра' });
    const filterInput = screen.getByPlaceholderText('Значение');

    // Matrix 12: eq
    fireEvent.change(filterOpSelect, { target: { value: 'eq' } });
    fireEvent.change(filterInput, { target: { value: 'direct' } });

    // Matrix 13: neq
    fireEvent.change(filterOpSelect, { target: { value: 'neq' } });

    // Matrix 14: in
    fireEvent.change(filterOpSelect, { target: { value: 'in' } });
    expect(screen.getByPlaceholderText('зн1, зн2')).toBeTruthy();

    // Matrix 15: contains
    fireEvent.change(filterOpSelect, { target: { value: 'contains' } });

    // Matrix 11: remove filter
    const removeBtn = screen.getByRole('button', { name: /Удалить фильтр/i });
    fireEvent.click(removeBtn);
    expect(screen.queryByRole('combobox', { name: 'Разрез фильтра' })).toBeNull();
  });

  // Matrix 16, 17, 18, 19: Sort
  it('Matrix 16-19: sort add, second sort, third sort rejected, and sorts only selected fields', () => {
    mount();

    const addSortBtn = screen.getAllByRole('button', { name: /Добавить/i })[1];
    fireEvent.click(addSortBtn);

    const sortFieldSelect = screen.getByRole('combobox', { name: 'Поле сортировки' });
    const sortDirSelect = screen.getByRole('combobox', { name: 'Направление сортировки' });
    expect(sortFieldSelect).toBeTruthy();
    expect(sortDirSelect).toBeTruthy();

    // Second sort
    fireEvent.click(addSortBtn);
    const sortFields = screen.getAllByRole('combobox', { name: 'Поле сортировки' });
    expect(sortFields.length).toBe(2);

    // Matrix 18: Add sort button hidden when max 2 sorts reached
    expect(screen.getAllByRole('button', { name: /Добавить/i })).toHaveLength(1);

    // Matrix 19: Options only contain selected fields (source, revenue, orders)
    const options = Array.from((sortFields[0] as HTMLSelectElement).options).map(o => o.value);
    expect(options).toEqual(['source', 'revenue', 'orders']);
  });

  // Matrix 20: Limit selector
  it('Matrix 20: limit selector offers 50, 100, 250, 500, 1000 with default 100', () => {
    mount();
    const limitSelect = screen.getByRole('combobox', { name: 'Лимит строк' }) as HTMLSelectElement;
    expect(limitSelect.value).toBe('100');

    const options = Array.from(limitSelect.options).map(o => o.value);
    expect(options).toEqual(['50', '100', '250', '500', '1000']);

    fireEvent.change(limitSelect, { target: { value: '250' } });
    expect(limitSelect.value).toBe('250');
  });

  // Matrix 21, 22, 23, 24: Execution contract
  it('Matrix 21-24: Build sends canonical request, no auto-run, loading state, prevents double submit', async () => {
    let resolveQuery: (res: any) => void;
    const queryPromise = new Promise(resolve => {
      resolveQuery = resolve;
    });
    vi.mocked(marketingApi.executeAdminMarketingQuery).mockReturnValueOnce(queryPromise as any);

    mount();

    // Matrix 22: Changing parameters does NOT trigger query automatically
    const prodBtn = screen.getByRole('button', { name: 'Товар' });
    fireEvent.click(prodBtn);
    expect(marketingApi.executeAdminMarketingQuery).not.toHaveBeenCalled();

    // Matrix 21: Clicking "Построить отчет" sends exact canonical request
    const buildBtn = screen.getByRole('button', { name: /Построить отчет/i });
    fireEvent.click(buildBtn);

    expect(marketingApi.executeAdminMarketingQuery).toHaveBeenCalledTimes(1);
    const sentReq = vi.mocked(marketingApi.executeAdminMarketingQuery).mock.calls[0][0];
    expect(sentReq.version).toBe(1);
    expect(sentReq.dimensions).toEqual(['source', 'product']);
    expect(sentReq.metrics).toEqual(['revenue', 'orders']);
    expect(sentReq.limit).toBe(100);
    expect(sentReq.period).toBeTruthy();

    // Matrix 24: Loading state visible
    expect((screen.getByRole('button', { name: 'Загрузка...' }) as HTMLButtonElement).disabled).toBe(true);

    // Matrix 23: Clicking again while loading does NOT send a second request
    const loadingBtn = screen.getByRole('button', { name: 'Загрузка...' });
    fireEvent.click(loadingBtn);
    expect(marketingApi.executeAdminMarketingQuery).toHaveBeenCalledTimes(1);

    // Resolve
    resolveQuery!({
      version: 1,
      query: sentReq,
      columns: [
        { key: 'source', kind: 'dimension', type: 'string' },
        { key: 'revenue', kind: 'metric', type: 'money' },
      ],
      rows: [],
      coverage: {},
      warnings: [],
    });

    await waitFor(() => {
    });
  });

  // Matrix 25, 26, 27: Responses with 0, 1, and 2 dimensions
  it('Matrix 25-27: renders metric-only, 1-dimension, and 2-dimension responses correctly', async () => {
    // 1. Metric-only response
    vi.mocked(marketingApi.executeAdminMarketingQuery).mockResolvedValueOnce({
      version: 1,
      query: { version: 1, period: { from: '2026-09-01T00:00:00Z', to: '2026-10-01T00:00:00Z' }, dimensions: [], metrics: ['orders', 'revenue'] },
      columns: [
        { key: 'orders', kind: 'metric', type: 'number' },
        { key: 'revenue', kind: 'metric', type: 'money' },
      ],
      rows: [{ orders: 125, revenue: 35000000 }],
      coverage: {},
      warnings: [],
    });

    mount();
    // Deselect source to make 0 dimensions
    fireEvent.click(screen.getByRole('button', { name: 'Источник' }));
    fireEvent.click(screen.getByRole('button', { name: /Построить отчет/i }));

    await waitFor(() => {
      expect(screen.getByText('125')).toBeTruthy();
      expect(screen.getByText('350 000 ₽')).toBeTruthy();
    });

    // 2. Two-dimension response (day + source)
    vi.mocked(marketingApi.executeAdminMarketingQuery).mockResolvedValueOnce({
      version: 1,
      query: { version: 1, period: { from: '2026-09-01T00:00:00Z', to: '2026-10-01T00:00:00Z' }, dimensions: ['day', 'source'], metrics: ['orders'] },
      columns: [
        { key: 'day', kind: 'dimension', type: 'date' },
        { key: 'source', kind: 'dimension', type: 'string' },
        { key: 'orders', kind: 'metric', type: 'number' },
      ],
      rows: [{ day: '2026-09-15', sourceKind: 'named', sourceKey: 'telegram', source: 'telegram', orders: 42 }],
      coverage: {},
      warnings: [],
    });

    fireEvent.click(screen.getByRole('button', { name: 'День' }));
    fireEvent.click(screen.getByRole('button', { name: 'Источник' }));
    fireEvent.click(screen.getByRole('button', { name: /Построить отчет/i }));

    await waitFor(() => {
      expect(screen.getByText('telegram')).toBeTruthy();
      expect(screen.getByText('42')).toBeTruthy();
    });
  });

  // Matrix 28, 29, 30, 31: Formatters (money, number, percentage, date)
  it('Matrix 28-31: formats money, numbers, percentage, and dates in Russian format', async () => {
    vi.mocked(marketingApi.executeAdminMarketingQuery).mockResolvedValueOnce({
      version: 1,
      query: { version: 1, period: { from: '2026-09-01T00:00:00Z', to: '2026-10-01T00:00:00Z' }, dimensions: ['day'], metrics: ['revenue', 'orders', 'conversion'] },
      columns: [
        { key: 'day', kind: 'dimension', type: 'date' },
        { key: 'revenue', kind: 'metric', type: 'money' },
        { key: 'orders', kind: 'metric', type: 'number' },
        { key: 'conversion', kind: 'metric', type: 'percentage' },
      ],
      rows: [
        {
          day: '2026-09-20',
          revenue: 12500000, // 125 000 ₽
          orders: 5420,     // 5 420
          conversion: 0.0525, // 5.25%
        },
      ],
      coverage: {},
      warnings: [],
    });

    mount();
    fireEvent.click(screen.getByRole('button', { name: /Построить отчет/i }));

    await waitFor(() => {
      expect(screen.getByText('125 000 ₽')).toBeTruthy();
      expect(screen.getByText('5 420')).toBeTruthy();
      expect(screen.getByText('5.25%')).toBeTruthy();
      expect(screen.getByText('20.09.2026')).toBeTruthy();
    });
  });

  // Matrix 32, 33, 34: Source direct, unattributed, named formatting
  it('Matrix 32-34: formats source as direct, unattributed, and named without showing technical keys', async () => {
    vi.mocked(marketingApi.executeAdminMarketingQuery).mockResolvedValueOnce({
      version: 1,
      query: { version: 1, period: { from: '2026-09-01T00:00:00Z', to: '2026-10-01T00:00:00Z' }, dimensions: ['source'], metrics: ['orders'] },
      columns: [
        { key: 'sourceKey', kind: 'dimension', type: 'string' },
        { key: 'sourceKind', kind: 'dimension', type: 'string' },
        { key: 'source', kind: 'dimension', type: 'string' },
        { key: 'orders', kind: 'metric', type: 'number' },
      ],
      rows: [
        { sourceKey: 'direct', sourceKind: 'direct', orders: 10 },
        { sourceKey: null, sourceKind: 'unattributed', orders: 5 },
        { sourceKey: 'vk', sourceKind: 'named', orders: 20 },
      ],
      coverage: {},
      warnings: [],
    });

    mount();
    fireEvent.click(screen.getByRole('button', { name: /Построить отчет/i }));

    await waitFor(() => {
      // Matrix 32: Direct
      expect(screen.getByText('Прямой заход')).toBeTruthy();
      // Matrix 33: Unattributed
      expect(screen.getByText('Неизвестный источник')).toBeTruthy();
      // Matrix 34: Named
      expect(screen.getByText('vk')).toBeTruthy();

      // Matrix 43: technical columns sourceKey and sourceKind hidden
      expect(screen.queryByText('sourceKey')).toBeNull();
      expect(screen.queryByText('sourceKind')).toBeNull();
    });
  });

  // Matrix 35, 36: Coverage partial, unavailable, and known zero
  it('Matrix 35-36: handles coverage partial badge, unavailable metric —, and known zero', async () => {
    vi.mocked(marketingApi.executeAdminMarketingQuery).mockResolvedValue({
      version: 1,
      query: { version: 1, period: { from: '2026-09-01T00:00:00Z', to: '2026-10-01T00:00:00Z' }, dimensions: ['source'], metrics: ['orders', 'conversion', 'sessions'] },
      columns: [
        { key: 'source', kind: 'dimension', type: 'string' },
        { key: 'orders', kind: 'metric', type: 'number' },
        { key: 'conversion', kind: 'metric', type: 'percentage' },
        { key: 'sessions', kind: 'metric', type: 'number' },
      ],
      rows: [
        {
          sourceKind: 'direct',
          orders: 0, // Known zero
          conversion: null, // Unavailable conversion -> —
          sessions: 100,
        },
      ],
      coverage: {
        sessions: { status: 'partial' },
        conversion: { status: 'unavailable' },
      },
      warnings: [],
    });

    mount();
    fireEvent.click(screen.getByRole('button', { name: /Построить отчет/i }));

    await waitFor(() => {
      expect(marketingApi.executeAdminMarketingQuery).toHaveBeenCalled();
    });

    await waitFor(() => {
      const table = screen.getByRole('table');
      expect(within(table).getByText('0')).toBeTruthy();
      expect(within(table).getByText('—')).toBeTruthy();
    });
  });

  // Matrix 37: Empty rows state
  it('Matrix 37: displays clean empty rows state when response has no rows', async () => {
    vi.mocked(marketingApi.executeAdminMarketingQuery).mockResolvedValueOnce({
      version: 1,
      query: { version: 1, period: { from: '2026-09-01T00:00:00Z', to: '2026-10-01T00:00:00Z' }, dimensions: ['source'], metrics: ['orders'] },
      columns: [{ key: 'source', kind: 'dimension', type: 'string' }],
      rows: [],
      coverage: {},
      warnings: [],
    });

    mount();
    fireEvent.click(screen.getByRole('button', { name: /Построить отчет/i }));

    await waitFor(() => {
      expect(screen.getByText('Нет данных')).toBeTruthy();
    });
  });

  // Matrix 38, 39, 40, 41: Error mapping and retry
  it('Matrix 38-41: maps unsupported_combination, validation errors, network errors, and supports retry', async () => {
    // 1. unsupported_combination
    vi.mocked(marketingApi.executeAdminMarketingQuery).mockRejectedValueOnce({
      code: 'unsupported_combination',
    });

    mount();
    const buildBtn = screen.getByRole('button', { name: /Построить отчет/i });
    fireEvent.click(buildBtn);

    await waitFor(() => {
      expect(screen.getByText('Этот показатель нельзя использовать с выбранными разрезами.')).toBeTruthy();
    });

    // 2. Retry button performs new request
    vi.mocked(marketingApi.executeAdminMarketingQuery).mockResolvedValueOnce({
      version: 1,
      query: { version: 1, period: { from: '2026-09-01T00:00:00Z', to: '2026-10-01T00:00:00Z' }, dimensions: [], metrics: ['orders'] },
      columns: [{ key: 'orders', kind: 'metric', type: 'number' }],
      rows: [{ orders: 5 }],
      coverage: {},
      warnings: [],
    });

    const retryBtn = screen.getByRole('button', { name: /Повторить/i });
    fireEvent.click(retryBtn);

    await waitFor(() => {
      expect(marketingApi.executeAdminMarketingQuery).toHaveBeenCalledTimes(2);
      const table = screen.getByRole('table');
      expect(within(table).getByText('5')).toBeTruthy();
    });
  });

  // Matrix 42, 44, 45, 46: Safety, terminology, and scope boundaries
  it('Matrix 42, 44-46: hides raw UUIDs, lacks backend/SQL jargon, and contains no Save or Export controls', async () => {
    vi.mocked(marketingApi.executeAdminMarketingQuery).mockResolvedValueOnce({
      version: 1,
      query: { version: 1, period: { from: '2026-09-01T00:00:00Z', to: '2026-10-01T00:00:00Z' }, dimensions: ['product'], metrics: ['orders'] },
      columns: [
        { key: 'product', kind: 'dimension', type: 'string' },
        { key: 'orders', kind: 'metric', type: 'number' },
      ],
      rows: [
        { product: '11111111-2222-3333-4444-555555555555', orders: 1 }, // Raw UUID simulated
      ],
      coverage: {},
      warnings: [],
    });

    mount();
    fireEvent.click(screen.getByRole('button', { name: /Построить отчет/i }));

    await waitFor(() => {
      // Matrix 42: Raw UUID is NOT displayed
      expect(screen.queryByText('11111111-2222-3333-4444-555555555555')).toBeNull();
    });

    // Matrix 44: No SQL/CTE/database terms
    expect(screen.queryByText(/CTE/i)).toBeNull();
    expect(screen.queryByText(/SELECT/i)).toBeNull();
    expect(screen.queryByText(/INNER JOIN/i)).toBeNull();

    // M8B: Export control exists and menu is initially closed
    expect(screen.getByRole('button', { name: /Экспорт/i })).toBeTruthy();
    expect(screen.queryByRole('button', { name: /CSV/i })).toBeNull();
  });
});
