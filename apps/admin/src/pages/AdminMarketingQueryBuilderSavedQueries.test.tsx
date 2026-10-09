import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { AdminMarketingQueryBuilder } from './AdminMarketingQueryBuilder';
import type { SavedQuery } from '@zamk/api-client/src/types';

vi.mock('@zamk/api-client/src/admin', () => ({
  listAdminMarketingSavedQueries: vi.fn(),
  createAdminMarketingSavedQuery: vi.fn(),
  updateAdminMarketingSavedQuery: vi.fn(),
  deleteAdminMarketingSavedQuery: vi.fn(),
}));

vi.mock('../api/marketing', () => ({
  executeAdminMarketingQuery: vi.fn(),
}));

import {
  listAdminMarketingSavedQueries,
  createAdminMarketingSavedQuery,
  updateAdminMarketingSavedQuery,
  deleteAdminMarketingSavedQuery,
} from '@zamk/api-client/src/admin';
import { executeAdminMarketingQuery } from '../api/marketing';

const mockList = vi.mocked(listAdminMarketingSavedQueries);
const mockCreate = vi.mocked(createAdminMarketingSavedQuery);
const mockUpdate = vi.mocked(updateAdminMarketingSavedQuery);
const mockDelete = vi.mocked(deleteAdminMarketingSavedQuery);
const mockExecute = vi.mocked(executeAdminMarketingQuery);

const sampleAlpha: SavedQuery = {
  id: '11111111-1111-1111-1111-111111111111',
  name: 'Отчёт по источникам',
  description: 'Анализ источников трафика и выручки',
  queryVersion: 1,
  querySpec: {
    version: 1,
    period: { from: '2026-03-01T00:00:00.000Z', to: '2026-03-15T00:00:00.000Z' },
    dimensions: ['source'],
    metrics: ['revenue', 'orders'],
    filters: [{ dimension: 'source', operator: 'eq', values: ['telegram'] }],
    sort: [{ field: 'revenue', direction: 'desc' }],
    limit: 250,
  },
  createdAt: '2026-03-01T10:00:00.000Z',
  updatedAt: '2026-03-01T10:00:00.000Z',
};

const sampleBetaOmittedLimit: SavedQuery = {
  id: '22222222-2222-2222-2222-222222222222',
  name: 'Кампании за месяц',
  description: 'Обзор конверсий по кампаниям',
  queryVersion: 1,
  querySpec: {
    version: 1,
    period: { from: '2026-02-01T00:00:00.000Z', to: '2026-03-01T00:00:00.000Z' },
    dimensions: ['campaign'],
    metrics: ['orders', 'conversion'],
    // spec.limit is omitted intentionally to test default 100 hydration
  },
  createdAt: '2026-02-01T10:00:00.000Z',
  updatedAt: '2026-02-02T12:00:00.000Z',
};

describe('AdminMarketingQueryBuilderSavedQueries — Behavioral Acceptance Suite', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockList.mockResolvedValue([]);
    mockExecute.mockResolvedValue({
      version: 1,
      query: sampleAlpha.querySpec,
      columns: [],
      rows: [],
      coverage: {},
      warnings: [],
    });
  });

  const renderBuilder = () =>
    render(
      <MemoryRouter>
        <AdminMarketingQueryBuilder />
      </MemoryRouter>
    );

  describe('1. Panel & List Behavior (Matrix 1-8, 54, 55, 59-61)', () => {
    it('Matrix 1-3: renders builder, reveals Сохранённые action, opens panel with proper title', async () => {
      renderBuilder();
      const openBtn = screen.getByRole('button', { name: /Сохранённые/i });
      expect(openBtn).toBeTruthy();

      fireEvent.click(openBtn);
      expect(screen.getByText('Сохранённые запросы')).toBeTruthy();
      expect(mockList).toHaveBeenCalledTimes(1);
    });

    it('Matrix 4-5: displays loading spinner then empty state when no saved queries exist', async () => {
      let resolveList: (val: SavedQuery[]) => void = () => {};
      mockList.mockReturnValue(new Promise((res) => { resolveList = res; }));

      renderBuilder();
      fireEvent.click(screen.getByRole('button', { name: /Сохранённые/i }));

      // Empty state appears after resolution
      resolveList([]);
      await waitFor(() => {
        expect(screen.getByText('Сохранённых запросов пока нет')).toBeTruthy();
        expect(screen.getByText(/Настройте отчёт и сохраните его/i)).toBeTruthy();
      });
    });

    it('Matrix 6-8, 59-61: renders populated list preserving backend order, hides UUIDs/JSON/technical fields, renders description as text', async () => {
      mockList.mockResolvedValue([sampleBetaOmittedLimit, sampleAlpha]);

      renderBuilder();
      fireEvent.click(screen.getByRole('button', { name: /Сохранённые/i }));

      await waitFor(() => {
        expect(screen.getByText('Кампании за месяц')).toBeTruthy();
        expect(screen.getByText('Отчёт по источникам')).toBeTruthy();
      });

      // Verify descriptions rendered as text
      expect(screen.getByText('Анализ источников трафика и выручки')).toBeTruthy();
      expect(screen.getByText('Обзор конверсий по кампаниям')).toBeTruthy();

      // Technical fields must be hidden
      expect(screen.queryByText('11111111-1111-1111-1111-111111111111')).toBeNull();
      expect(screen.queryByText('22222222-2222-2222-2222-222222222222')).toBeNull();
      expect(screen.queryByText(/queryVersion/i)).toBeNull();
      expect(screen.queryByText(/querySpec/i)).toBeNull();

      // Backend order preserved: Beta is first, Alpha is second
      const sidebar = screen.getByTestId('saved-queries-sidebar'); const items = within(sidebar).getAllByRole('heading', { level: 3 });
      expect(items[0].textContent).toContain('Кампании за месяц');
      expect(items[1].textContent).toContain('Отчёт по источникам');
    });

    it('Matrix 54-55: list failure displays human error message and retry button refetches list', async () => {
      mockList.mockRejectedValueOnce({ status: 500, message: 'Server error' });

      renderBuilder();
      fireEvent.click(screen.getByRole('button', { name: /Сохранённые/i }));

      await waitFor(() => {
        expect(screen.getByText('Не удалось выполнить действие. Попробуйте ещё раз.')).toBeTruthy();
      });

      mockList.mockResolvedValueOnce([sampleAlpha]);
      const retryBtn = screen.getByRole('button', { name: /Повторить/i });
      fireEvent.click(retryBtn);

      await waitFor(() => {
        expect(screen.getByText('Отчёт по источникам')).toBeTruthy();
      });
      expect(mockList).toHaveBeenCalledTimes(2);
    });
  });

  describe('2. Save New Query Modal & Validation (Matrix 9-18, 56, 57, 58)', () => {
    it('Matrix 9-10: Save button exists and opens save modal', async () => {
      renderBuilder();
      const saveBtn = screen.getByRole('button', { name: /^Сохранить$/i });
      expect(saveBtn).toBeTruthy();

      fireEvent.click(saveBtn);
      expect(screen.getByText('Сохранить запрос')).toBeTruthy();
      expect(screen.getByLabelText(/Название \*/i)).toBeTruthy();
      expect(screen.getByLabelText(/Описание/i)).toBeTruthy();
    });

    it('Matrix 11-13: rejects blank name, enforces 120 max name and 500 max description', async () => {
      renderBuilder();
      fireEvent.click(screen.getByRole('button', { name: /^Сохранить$/i }));

      const modal = screen.getByRole('dialog'); const submitBtn = within(modal).getByRole('button', { name: 'Сохранить' });
      fireEvent.click(submitBtn);

      expect(screen.getByText('Введите название запроса')).toBeTruthy();
      expect(mockCreate).not.toHaveBeenCalled();

      const nameInput = screen.getByLabelText(/Название \*/i) as HTMLInputElement;
      const descInput = screen.getByLabelText(/Описание/i) as HTMLTextAreaElement;
      expect(nameInput.maxLength).toBe(120);
      expect(descInput.maxLength).toBe(500);
    });

    it('Matrix 14-16: submitting valid query creates exact QueryRequest payload, does NOT execute analytics, and refreshes list', async () => {
      mockCreate.mockResolvedValue(sampleAlpha);

      renderBuilder();
      fireEvent.click(screen.getByRole('button', { name: /^Сохранить$/i }));

      fireEvent.change(screen.getByLabelText(/Название \*/i), { target: { value: 'Новый отчёт' } });
      fireEvent.change(screen.getByLabelText(/Описание/i), { target: { value: 'Описание отчёта' } });

      const modal = screen.getByRole('dialog'); const submitBtn = within(modal).getByRole('button', { name: 'Сохранить' });
      fireEvent.click(submitBtn);

      await waitFor(() => {
        expect(mockCreate).toHaveBeenCalledTimes(1);
      });

      const callArgs = mockCreate.mock.calls[0][0];
      expect(callArgs.name).toBe('Новый отчёт');
      expect(callArgs.description).toBe('Описание отчёта');
      expect(callArgs.querySpec.version).toBe(1);
      expect(callArgs.querySpec.dimensions).toEqual(['source']);
      expect(callArgs.querySpec.metrics).toEqual(['revenue', 'orders']);
      expect(callArgs.querySpec.limit).toBe(100);

      // Does NOT execute analytics
      expect(mockExecute).not.toHaveBeenCalled();

      // Modal closes and active header context appears
      await waitFor(() => {
        expect(screen.queryByText('Сохранить запрос')).toBeNull();
        expect(screen.getByText(/Сохранённый запрос: Отчёт по источникам/i)).toBeTruthy();
      });
    });

    it('Matrix 17: duplicate name error 409 maps to human message', async () => {
      mockCreate.mockRejectedValueOnce({ status: 409, code: 'duplicate_name' });

      renderBuilder();
      fireEvent.click(screen.getByRole('button', { name: /^Сохранить$/i }));

      fireEvent.change(screen.getByLabelText(/Название \*/i), { target: { value: 'Дубликат' } });
      const modal = screen.getByRole('dialog'); fireEvent.click(within(modal).getByRole('button', { name: 'Сохранить' }));

      await waitFor(() => {
        expect(screen.getByText('Запрос с таким названием уже существует.')).toBeTruthy();
      });
    });

    it('Matrix 18: network error displays human message', async () => {
      mockCreate.mockRejectedValueOnce(new Error('Network error'));

      renderBuilder();
      fireEvent.click(screen.getByRole('button', { name: /^Сохранить$/i }));

      fireEvent.change(screen.getByLabelText(/Название \*/i), { target: { value: 'Тест' } });
      const modal = screen.getByRole('dialog'); fireEvent.click(within(modal).getByRole('button', { name: 'Сохранить' }));

      await waitFor(() => {
        expect(screen.getByText('Не удалось выполнить действие. Попробуйте ещё раз.')).toBeTruthy();
      });
    });

    it('Matrix 56-57: no Save As buttons', () => {
      renderBuilder();
      expect(screen.queryByRole('button', { name: /Save As/i })).toBeNull();
      expect(screen.queryByRole('button', { name: /Сохранить как/i })).toBeNull();
    });
  });

  describe('3. Open Saved Query & Hydration (Matrix 19-29, 62)', () => {
    it('Matrix 19-24, 26-29: opening saved query hydrates dimensions, metrics, filters, sort, explicit limit, absolute period, shows active context, NOT dirty, and does NOT auto-run', async () => {
      mockList.mockResolvedValue([sampleAlpha]);

      renderBuilder();
      fireEvent.click(screen.getByRole('button', { name: /Сохранённые/i }));

      await waitFor(() => {
        expect(screen.getByText('Отчёт по источникам')).toBeTruthy();
      });

      const selectBtn = screen.getByRole('button', { name: /Выбрать/i });
      fireEvent.click(selectBtn);

      // Panel closes
      expect(screen.queryByText('Сохранённые запросы')).toBeNull();

      // Does NOT auto-run
      expect(mockExecute).not.toHaveBeenCalled();

      // Active context shown
      expect(screen.getByText(/Сохранённый запрос: Отчёт по источникам/i)).toBeTruthy();

      // Immediately after open: NOT dirty!
      expect(screen.queryByText(/Есть несохранённые изменения/i)).toBeNull();
      expect(screen.getByRole('button', { name: /Сохранено/i }).hasAttribute("disabled")).toBe(true);

      // Hydration check: explicit limit 250
      const limitSelect = screen.getByLabelText('Лимит строк') as HTMLSelectElement;
      expect(limitSelect.value).toBe('250');
    });

    it('Matrix 25: opening saved query with omitted limit hydrates default limit 100', async () => {
      mockList.mockResolvedValue([sampleBetaOmittedLimit]);

      renderBuilder();
      fireEvent.click(screen.getByRole('button', { name: /Сохранённые/i }));

      await waitFor(() => {
        expect(screen.getByText('Кампании за месяц')).toBeTruthy();
      });

      fireEvent.click(screen.getByRole('button', { name: /Выбрать/i }));

      // Default limit 100 preserved (M5 contract)
      const limitSelect = screen.getByLabelText('Лимит строк') as HTMLSelectElement;
      expect(limitSelect.value).toBe('100');
    });

    it('Matrix 62: manual click on Построить отчет executes the hydrated query request', async () => {
      mockList.mockResolvedValue([sampleAlpha]);

      renderBuilder();
      fireEvent.click(screen.getByRole('button', { name: /Сохранённые/i }));
      await waitFor(() => screen.getByText('Отчёт по источникам'));
      fireEvent.click(screen.getByRole('button', { name: /Выбрать/i }));

      expect(mockExecute).not.toHaveBeenCalled();

      const runBtn = screen.getByRole('button', { name: /Построить отчет/i });
      fireEvent.click(runBtn);

      expect(mockExecute).toHaveBeenCalledTimes(1);
      const req = mockExecute.mock.calls[0][0];
      expect(req.dimensions).toEqual(['source']);
      expect(req.metrics).toEqual(['revenue', 'orders']);
      expect(req.limit).toBe(250);
    });
  });

  describe('4. Dirty State Tracking (Matrix 30-35)', () => {
    it('Matrix 30-35: modifying dimension, metric, filter, or limit makes builder dirty', async () => {
      mockList.mockResolvedValue([sampleAlpha]);

      renderBuilder();
      fireEvent.click(screen.getByRole('button', { name: /Сохранённые/i }));
      await waitFor(() => screen.getByText('Отчёт по источникам'));
      fireEvent.click(screen.getByRole('button', { name: /Выбрать/i }));

      expect(screen.queryByText(/Есть несохранённые изменения/i)).toBeNull();

      // 30: Modify dimension
      const dayDimBtn = screen.getByRole('button', { name: /День/i });
      fireEvent.click(dayDimBtn);

      expect(screen.getByText(/Есть несохранённые изменения/i)).toBeTruthy();
      expect(screen.getByRole('button', { name: /Сохранить изменения/i }).hasAttribute("disabled")).toBe(false);

      // Revert dimension
      fireEvent.click(dayDimBtn);
      expect(screen.queryByText(/Есть несохранённые изменения/i)).toBeNull();

      // 31: Modify metric
      const sessionsMetricBtn = screen.getByRole('button', { name: /Сессии/i });
      fireEvent.click(sessionsMetricBtn);
      expect(screen.getByText(/Есть несохранённые изменения/i)).toBeTruthy();
      fireEvent.click(sessionsMetricBtn);
      expect(screen.queryByText(/Есть несохранённые изменения/i)).toBeNull();

      // 35: Modify limit
      const limitSelect = screen.getByLabelText('Лимит строк');
      fireEvent.change(limitSelect, { target: { value: '500' } });
      expect(screen.getByText(/Есть несохранённые изменения/i)).toBeTruthy();
      fireEvent.change(limitSelect, { target: { value: '250' } });
      expect(screen.queryByText(/Есть несохранённые изменения/i)).toBeNull();
    });
  });

  describe('5. Save Changes (Matrix 36-40)', () => {
    it('Matrix 36-40: save changes sends PATCH with querySpec, does not execute analytics, clears dirty state, and updates active query', async () => {
      mockList.mockResolvedValue([sampleAlpha]);
      const updatedAlpha: SavedQuery = {
        ...sampleAlpha,
        updatedAt: '2026-03-02T15:00:00.000Z',
        querySpec: {
          ...sampleAlpha.querySpec,
          limit: 500,
        },
      };
      mockUpdate.mockResolvedValue(updatedAlpha);

      renderBuilder();
      fireEvent.click(screen.getByRole('button', { name: /Сохранённые/i }));
      await waitFor(() => screen.getByText('Отчёт по источникам'));
      fireEvent.click(screen.getByRole('button', { name: /Выбрать/i }));

      // Modify limit to make dirty
      const limitSelect = screen.getByLabelText('Лимит строк');
      fireEvent.change(limitSelect, { target: { value: '500' } });

      const saveChangesBtn = screen.getByRole('button', { name: /Сохранить изменения/i });
      fireEvent.click(saveChangesBtn);

      await waitFor(() => {
        expect(mockUpdate).toHaveBeenCalledTimes(1);
      });

      expect(mockUpdate).toHaveBeenCalledWith(sampleAlpha.id, {
        querySpec: expect.objectContaining({ limit: 500 }),
      });

      // Does not execute analytics
      expect(mockExecute).not.toHaveBeenCalled();

      // Clears dirty
      await waitFor(() => {
        expect(screen.queryByText(/Есть несохранённые изменения/i)).toBeNull();
        expect(screen.getByRole('button', { name: /Сохранено/i }).hasAttribute("disabled")).toBe(true);
      });
    });
  });

  describe('6. Rename Saved Query (Matrix 41-45)', () => {
    it('Matrix 41-45: rename opens modal with name only, sends PATCH name, handles 409, and refreshes active title', async () => {
      mockList.mockResolvedValue([sampleAlpha]);
      const renamedAlpha: SavedQuery = {
        ...sampleAlpha,
        name: 'Переименованный отчёт',
      };
      mockUpdate.mockResolvedValue(renamedAlpha);

      renderBuilder();
      // First select Alpha so it is active
      fireEvent.click(screen.getByRole('button', { name: /Сохранённые/i }));
      await waitFor(() => screen.getByText('Отчёт по источникам'));
      fireEvent.click(screen.getByRole('button', { name: /Выбрать/i }));

      // Open sidebar again to rename
      fireEvent.click(screen.getByRole('button', { name: /Сохранённые/i }));
      await waitFor(() => screen.getByText('Отчёт по источникам'));

      const renameBtn = screen.getByTitle('Переименовать');
      fireEvent.click(renameBtn);

      // Rename modal opened
      expect(screen.getByText('Переименовать запрос')).toBeTruthy();
      // Description is hidden in rename mode (Blocker 10)
      expect(screen.queryByLabelText(/Описание/i)).toBeNull();

      const nameInput = screen.getByLabelText(/Название \*/i);
      fireEvent.change(nameInput, { target: { value: 'Переименованный отчёт' } });

      const modal = screen.getByRole('dialog'); const submitRename = within(modal).getByRole('button', { name: 'Переименовать' });
      fireEvent.click(submitRename);

      await waitFor(() => {
        expect(mockUpdate).toHaveBeenCalledWith(sampleAlpha.id, {
          name: 'Переименованный отчёт',
        });
      });

      // Active title in header updates
      await waitFor(() => {
        expect(screen.getByText(/Сохранённый запрос: Переименованный отчёт/i)).toBeTruthy();
      });
    });
  });

  describe('7. Delete Saved Query & Outcome Ownership (Matrix 46-53)', () => {
    it('Matrix 46-49: delete asks in-product confirmation, cancel aborts, confirm deletes and refreshes', async () => {
      mockList.mockResolvedValue([sampleAlpha]);
      mockDelete.mockResolvedValue();

      renderBuilder();
      fireEvent.click(screen.getByRole('button', { name: /Сохранённые/i }));
      await waitFor(() => screen.getByText('Отчёт по источникам'));

      const trashBtn = screen.getByTitle('Удалить');
      fireEvent.click(trashBtn);

      // In-product compact confirmation shown (Blocker 4)
      const confirmDialog = screen.getByTestId('delete-confirmation-dialog');
      expect(confirmDialog).toBeTruthy();
      expect(within(confirmDialog).getByText('Удалить «Отчёт по источникам»?')).toBeTruthy();

      // Cancel
      const cancelBtn = within(confirmDialog).getByRole('button', { name: 'Отмена' });
      fireEvent.click(cancelBtn);
      expect(screen.queryByTestId('delete-confirmation-dialog')).toBeNull();
      expect(mockDelete).not.toHaveBeenCalled();

      // Open confirm again and click Delete
      fireEvent.click(screen.getByTitle('Удалить'));
      const deleteConfirmBtn = within(screen.getByTestId('delete-confirmation-dialog')).getByRole('button', { name: 'Удалить' });
      fireEvent.click(deleteConfirmBtn);

      await waitFor(() => {
        expect(mockDelete).toHaveBeenCalledWith(sampleAlpha.id);
      });
    });

    it('Matrix 50-53: deleting active query clears active context while KEEPING all Builder controls intact (Blocker 3)', async () => {
      mockList.mockResolvedValue([sampleAlpha]);
      mockDelete.mockResolvedValue();

      renderBuilder();
      // 1. Select Alpha so it is active
      fireEvent.click(screen.getByRole('button', { name: /Сохранённые/i }));
      await waitFor(() => screen.getByText('Отчёт по источникам'));
      fireEvent.click(screen.getByRole('button', { name: /Выбрать/i }));

      expect(screen.getByText(/Сохранённый запрос: Отчёт по источникам/i)).toBeTruthy();
      const limitSelect = screen.getByLabelText('Лимит строк') as HTMLSelectElement;
      expect(limitSelect.value).toBe('250');

      // 2. Open sidebar and delete Alpha
      fireEvent.click(screen.getByRole('button', { name: /Сохранённые/i }));
      await waitFor(() => screen.getByText('Отчёт по источникам'));

      fireEvent.click(screen.getByTitle('Удалить'));
      const deleteConfirmBtn = within(screen.getByTestId('delete-confirmation-dialog')).getByRole('button', { name: 'Удалить' });
      fireEvent.click(deleteConfirmBtn);

      await waitFor(() => {
        expect(mockDelete).toHaveBeenCalledWith(sampleAlpha.id);
      });

      // Active context banner cleared
      expect(screen.queryByText(/Сохранённый запрос: Отчёт по источникам/i)).toBeNull();

      // Primary save action becomes "Сохранить"
      expect(screen.getByRole('button', { name: /^Сохранить$/i })).toBeTruthy();

      // Builder controls remain completely preserved! (Blocker 3)
      expect(limitSelect.value).toBe('250');
      expect(screen.getByRole('button', { name: /Источник/i }).className).toContain('bg-gray-900');
    });
  });
});
