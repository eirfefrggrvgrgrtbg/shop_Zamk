import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { AdminMarketingQueryBuilder } from './AdminMarketingQueryBuilder';
import type { SavedQuery } from '@zamk/api-client/src/types';

vi.mock('@zamk/api-client/src/admin', () => ({
  listAdminMarketingSavedQueries: vi.fn(),
  createAdminMarketingSavedQuery: vi.fn(),
  updateAdminMarketingSavedQuery: vi.fn(),
  deleteAdminMarketingSavedQuery: vi.fn(),
  exportAdminMarketingQuery: vi.fn(),
}));

vi.mock('../api/marketing', () => ({
  executeAdminMarketingQuery: vi.fn(),
}));

import {
  listAdminMarketingSavedQueries,
  createAdminMarketingSavedQuery,
  updateAdminMarketingSavedQuery,
  exportAdminMarketingQuery,
} from '@zamk/api-client/src/admin';
import { executeAdminMarketingQuery } from '../api/marketing';

const mockList = vi.mocked(listAdminMarketingSavedQueries);
const mockCreate = vi.mocked(createAdminMarketingSavedQuery);
const mockUpdate = vi.mocked(updateAdminMarketingSavedQuery);
const mockExecute = vi.mocked(executeAdminMarketingQuery);
const mockExport = vi.mocked(exportAdminMarketingQuery);

const sampleSavedQuery: SavedQuery = {
  id: '33333333-3333-3333-3333-333333333333',
  name: 'Saved Source Query',
  description: 'Analysis of sources',
  queryVersion: 1,
  querySpec: {
    version: 1,
    period: { from: '2026-04-01T00:00:00.000Z', to: '2026-04-10T00:00:00.000Z' },
    dimensions: ['source'],
    metrics: ['revenue'],
    filters: [{ dimension: 'source', operator: 'eq', values: ['telegram'] }],
    sort: [{ field: 'revenue', direction: 'desc' }],
    limit: 500,
  },
  createdAt: '2026-04-01T10:00:00.000Z',
  updatedAt: '2026-04-01T10:00:00.000Z',
};

describe('AdminMarketingQueryBuilderExport — M8B Behavioral Acceptance Suite', () => {
  let createdObjectURLs: string[] = [];
  let revokedObjectURLs: string[] = [];
  let clickedAnchors: HTMLAnchorElement[] = [];

  beforeEach(() => {
    vi.clearAllMocks();
    mockList.mockResolvedValue([]);
    mockExecute.mockResolvedValue({
      version: 1,
      query: { version: 1, period: { from: '', to: '' }, dimensions: [], metrics: [] },
      columns: [{ key: 'source', kind: 'dimension', type: 'string' }],
      rows: [{ source: 'direct' }],
      coverage: {},
      warnings: [],
    });

    createdObjectURLs = [];
    revokedObjectURLs = [];
    clickedAnchors = [];

    window.URL.createObjectURL = vi.fn().mockImplementation((_blob: Blob) => {
      const url = `blob:mock-url-${createdObjectURLs.length + 1}`;
      createdObjectURLs.push(url);
      return url;
    });

    window.URL.revokeObjectURL = vi.fn().mockImplementation((url: string) => {
      revokedObjectURLs.push(url);
    });

    // Spy on HTMLAnchorElement click and document.body appendChild/removeChild
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
      clickedAnchors.push(this);
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  const renderBuilder = () => {
    return render(
      <MemoryRouter>
        <AdminMarketingQueryBuilder />
      </MemoryRouter>
    );
  };

  it('1-7: Export button exists, disabled with zero metrics, enabled with metrics, menu opens with CSV and XLSX', () => {
    renderBuilder();

    // 1 & 2: Query builder renders and Export button exists
    const exportBtn = screen.getByRole('button', { name: /Экспорт/i });
    expect(exportBtn).toBeTruthy();

    // 4: Initially enabled with default metrics ['revenue', 'orders']
    expect(exportBtn.hasAttribute('disabled')).toBe(false);

    // 3: Deselect all metrics -> Export disabled
    const revMetric = screen.getByRole('button', { name: /^Выручка$/i });
    const ordMetric = screen.getByRole('button', { name: /^Заказы$/i });
    fireEvent.click(revMetric);
    fireEvent.click(ordMetric);
    expect(exportBtn.hasAttribute('disabled')).toBe(true);

    // Re-enable by clicking revenue
    fireEvent.click(revMetric);
    expect(exportBtn.hasAttribute('disabled')).toBe(false);

    // 5-7: Click Export opens menu with CSV and Excel (.xlsx) options
    expect(screen.queryByRole('menuitem', { name: /^CSV$/i })).toBeNull();
    fireEvent.click(exportBtn);

    const csvOption = screen.getByRole('menuitem', { name: /^CSV$/i });
    const xlsxOption = screen.getByRole('menuitem', { name: /Excel \(\.xlsx\)/i });
    expect(csvOption).toBeTruthy();
    expect(xlsxOption).toBeTruthy();
  });

  it('8, 9, 12-14, 18-25, 28-31, 38: CSV export sends current canonical QueryRequest, triggers browser download, and does NOT alter Builder state', async () => {
    mockExport.mockResolvedValue({
      blob: new Blob(['source;revenue\ndirect;123.45'], { type: 'text/csv' }),
      filename: 'zamk-marketing-report-2026-10-09.csv',
      contentType: 'text/csv; charset=utf-8',
    });

    renderBuilder();

    // 38: WITHOUT prior build, export directly
    const exportBtn = screen.getByRole('button', { name: /Экспорт/i });
    fireEvent.click(exportBtn);

    const csvOption = screen.getByRole('menuitem', { name: /^CSV$/i });
    fireEvent.click(csvOption);

    // 8, 9, 18-23: Verify exportAdminMarketingQuery called with format 'csv' and current QueryRequest
    await waitFor(() => {
      expect(mockExport).toHaveBeenCalledTimes(1);
    });

    const firstCall = mockExport.mock.calls[0]!;
    const [calledFormat, calledQuery] = firstCall;
    expect(calledFormat).toBe('csv');
    expect(calledQuery?.version).toBe(1);
    expect(calledQuery?.dimensions).toEqual(['source']);
    expect(calledQuery?.metrics).toEqual(['revenue', 'orders']);
    expect(calledQuery?.limit).toBe(100);
    expect(calledQuery?.period).toBeDefined();

    // 12-14: Export does NOT call execute, create, or update
    expect(mockExecute).not.toHaveBeenCalled();
    expect(mockCreate).not.toHaveBeenCalled();
    expect(mockUpdate).not.toHaveBeenCalled();

    // 24, 25, 28-31: Browser download triggered with server filename and cleanup performed
    expect(window.URL.createObjectURL).toHaveBeenCalledTimes(1);
    expect(clickedAnchors.length).toBe(1);
    expect(clickedAnchors[0].download).toBe('zamk-marketing-report-2026-10-09.csv');
    expect(window.URL.revokeObjectURL).toHaveBeenCalledWith(createdObjectURLs[0]);
  });

  it('10, 11, 26, 27: XLSX export sends format xlsx with canonical QueryRequest and downloads with server filename', async () => {
    mockExport.mockResolvedValue({
      blob: new Blob(['fake xlsx bytes'], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' }),
      filename: 'zamk-marketing-report-2026-10-09.xlsx',
      contentType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    });

    renderBuilder();

    const exportBtn = screen.getByRole('button', { name: /Экспорт/i });
    fireEvent.click(exportBtn);

    const xlsxOption = screen.getByRole('menuitem', { name: /Excel \(\.xlsx\)/i });
    fireEvent.click(xlsxOption);

    await waitFor(() => {
      expect(mockExport).toHaveBeenCalledTimes(1);
    });

    const firstCall = mockExport.mock.calls[0]!;
    const [calledFormat, calledQuery] = firstCall;
    expect(calledFormat).toBe('xlsx');
    expect(calledQuery?.dimensions).toEqual(['source']);
    expect(calledQuery?.metrics).toEqual(['revenue', 'orders']);

    expect(clickedAnchors.length).toBe(1);
    expect(clickedAnchors[0].download).toBe('zamk-marketing-report-2026-10-09.xlsx');
  });

  it('15-17, 39, 40: Active Saved Query exports clean/dirty state, preserves dirty state, and does not alter active query', async () => {
    mockList.mockResolvedValue([sampleSavedQuery]);
    mockExport.mockResolvedValue({
      blob: new Blob(['content'], { type: 'text/csv' }),
      filename: 'zamk-marketing-report-2026-10-09.csv',
      contentType: 'text/csv',
    });

    renderBuilder();

    // Open saved queries and load sampleSavedQuery
    fireEvent.click(screen.getByRole('button', { name: /Сохранённые/i }));
    await waitFor(() => {
      expect(screen.getByText('Saved Source Query')).toBeTruthy();
    });
    fireEvent.click(screen.getByRole('button', { name: /Выбрать/i }));

    // Verify loaded context (Clean state)
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Сохранено/i })).toBeTruthy();
    });

    // 15: Export clean state
    const exportBtn = screen.getByRole('button', { name: /Экспорт/i });
    fireEvent.click(exportBtn);
    fireEvent.click(screen.getByRole('menuitem', { name: /^CSV$/i }));

    await waitFor(() => {
      expect(mockExport).toHaveBeenCalledTimes(1);
    });
    expect(mockExport.mock.calls[0]?.[1]?.limit).toBe(500);

    // Make state DIRTY by changing limit to 250
    const limitSelect = screen.getByLabelText('Лимит строк');
    fireEvent.change(limitSelect, { target: { value: '250' } });

    // Verify dirty button appears
    expect(screen.getByRole('button', { name: /Сохранить изменения/i })).toBeTruthy();

    // 16 & 17: Export DIRTY state without auto-saving
    fireEvent.click(screen.getByRole('button', { name: /Экспорт/i }));
    fireEvent.click(screen.getByRole('menuitem', { name: /^CSV$/i }));

    await waitFor(() => {
      expect(mockExport).toHaveBeenCalledTimes(2);
    });
    expect(mockExport.mock.calls[1]?.[1]?.limit).toBe(250); // DIRTY value exported!

    // 17, 40: Dirty state and active query remain untouched after export
    expect(screen.getByRole('button', { name: /Сохранить изменения/i })).toBeTruthy();
    expect(mockUpdate).not.toHaveBeenCalled();
    expect(screen.getByText(/Saved Source Query/i)).toBeTruthy();
  });

  it('32, 33: Loading state prevents second click and double submit', async () => {
    let resolveExport: (val: any) => void;
    const pendingPromise = new Promise(resolve => {
      resolveExport = resolve;
    });
    mockExport.mockReturnValue(pendingPromise as any);

    renderBuilder();

    const exportBtn = screen.getByRole('button', { name: /Экспорт/i });
    fireEvent.click(exportBtn);
    fireEvent.click(screen.getByRole('menuitem', { name: /^CSV$/i }));

    // While loading, button shows "Экспорт..." and is disabled
    expect(screen.getByRole('button', { name: /Экспорт\.\.\./i })).toBeTruthy();
    expect(screen.getByRole('button', { name: /Экспорт\.\.\./i }).hasAttribute('disabled')).toBe(true);

    // 32 & 33: Further clicks do not trigger another export
    fireEvent.click(screen.getByRole('button', { name: /Экспорт\.\.\./i }));
    expect(mockExport).toHaveBeenCalledTimes(1);

    // Resolve export
    resolveExport!({
      blob: new Blob(['csv']),
      filename: 'report.csv',
      contentType: 'text/csv',
    });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /^Экспорт$/i })).toBeTruthy();
    });
  });

  it('34-37: Human-safe error mapping hides raw technical details on 400, 403, and network errors', async () => {
    // 34: 400 Bad Request
    mockExport.mockRejectedValueOnce({
      status: 400,
      code: 'invalid_dimensions',
      message: 'Raw backend internal error: sql column missing',
    });

    renderBuilder();

    fireEvent.click(screen.getByRole('button', { name: /Экспорт/i }));
    fireEvent.click(screen.getByRole('menuitem', { name: /^CSV$/i }));

    await waitFor(() => {
      expect(screen.getByText('Не удалось экспортировать отчёт. Проверьте параметры запроса.')).toBeTruthy();
    });
    expect(screen.queryByText(/sql column missing/i)).toBeNull();

    // 35: 403 Forbidden
    mockExport.mockRejectedValueOnce({
      status: 403,
      code: 'permission_denied',
      message: 'Raw unauthorized RBAC exception',
    });

    fireEvent.click(screen.getByRole('button', { name: /Экспорт/i }));
    fireEvent.click(screen.getByRole('menuitem', { name: /^CSV$/i }));

    await waitFor(() => {
      expect(screen.getByText('Недостаточно прав для экспорта отчёта.')).toBeTruthy();
    });
    expect(screen.queryByText(/RBAC exception/i)).toBeNull();

    // 36: Network / 500
    mockExport.mockRejectedValueOnce(new Error('Network error 500'));

    fireEvent.click(screen.getByRole('button', { name: /Экспорт/i }));
    fireEvent.click(screen.getByRole('menuitem', { name: /^CSV$/i }));

    await waitFor(() => {
      expect(screen.getByText('Не удалось экспортировать отчёт. Попробуйте ещё раз.')).toBeTruthy();
    });
  });
});
