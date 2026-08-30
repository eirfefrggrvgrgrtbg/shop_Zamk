import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import React, { act } from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { AdminSearchPalette } from './AdminSearchPalette';
import * as adminSearchApi from '../../api/adminSearch';
import type { GlobalSearchResult, GlobalSearchResponse } from '../../api/adminSearch';

// Location observer component to assert real React Router navigations
function LocationObserver({ onLocationChange }: { onLocationChange: (loc: { pathname: string; search: string }) => void }) {
  const location = useLocation();
  React.useEffect(() => {
    onLocationChange({ pathname: location.pathname, search: location.search });
  }, [location, onLocationChange]);
  return null;
}

describe('AdminSearchPalette (Real Component Tests)', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  // A. isOpen=false -> palette absent
  it('A. does not render when isOpen=false', () => {
    const { container } = render(
      <MemoryRouter>
        <AdminSearchPalette isOpen={false} onClose={() => {}} />
      </MemoryRouter>
    );
    expect(container.firstChild).toBeNull();
    expect(screen.queryByTestId('admin-search-palette')).toBeNull();
  });

  // B. isOpen=true -> dialog present & search input receives focus after effect
  it('B. renders dialog with accessibility attributes and auto-focuses search input', async () => {
    render(
      <MemoryRouter>
        <AdminSearchPalette isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    const dialog = screen.getByTestId('admin-search-palette');
    expect(dialog).toBeDefined();
    expect(dialog.getAttribute('role')).toBe('dialog');
    expect(dialog.getAttribute('aria-modal')).toBe('true');
    expect(dialog.getAttribute('aria-label')).toBe('Глобальный поиск');

    const input = screen.getByTestId('admin-search-input');
    expect(input).toBeDefined();
    expect(input.getAttribute('placeholder')).toBe('Поиск по заказу, ZMU, ZMK, SKU, email или товару');

    // Advance 50ms autofocus timer
    await act(async () => {
      vi.advanceTimersByTime(50);
    });

    expect(document.activeElement).toBe(input);
  });

  // C. click real backdrop -> onClose called; click inner card -> onClose NOT called
  it('C. closes on backdrop click and does not close on dialog content click', () => {
    const onClose = vi.fn();
    render(
      <MemoryRouter>
        <AdminSearchPalette isOpen={true} onClose={onClose} />
      </MemoryRouter>
    );

    const overlay = screen.getByTestId('admin-search-palette-overlay');
    const dialog = screen.getByTestId('admin-search-palette');

    // Click inside dialog
    fireEvent.click(dialog);
    expect(onClose).not.toHaveBeenCalled();

    // Click backdrop overlay
    fireEvent.click(overlay);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  // D. press Escape on real input -> onClose called
  it('D. closes on Escape keydown in input', () => {
    const onClose = vi.fn();
    render(
      <MemoryRouter>
        <AdminSearchPalette isOpen={true} onClose={onClose} />
      </MemoryRouter>
    );

    const input = screen.getByTestId('admin-search-input');
    fireEvent.keyDown(input, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  // E. type 1 char, advance timers past debounce -> API not called
  it('E. does not call API when input query is < 2 characters', async () => {
    const searchSpy = vi.spyOn(adminSearchApi, 'searchAdminGlobal');
    render(
      <MemoryRouter>
        <AdminSearchPalette isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    const input = screen.getByTestId('admin-search-input');
    fireEvent.change(input, { target: { value: 'a' } });

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    expect(searchSpy).not.toHaveBeenCalled();
  });

  // F. type 2+ chars, advance fake timers 250ms -> API called once with trimmed query
  it('F. calls API once with trimmed query after 250ms debounce', async () => {
    const searchSpy = vi.spyOn(adminSearchApi, 'searchAdminGlobal').mockResolvedValue({
      results: [],
    });

    render(
      <MemoryRouter>
        <AdminSearchPalette isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    const input = screen.getByTestId('admin-search-input');
    fireEvent.change(input, { target: { value: '  ORD-100  ' } });

    // Before debounce
    expect(searchSpy).not.toHaveBeenCalled();

    // Advance debounce
    await act(async () => {
      vi.advanceTimersByTime(250);
    });

    expect(searchSpy).toHaveBeenCalledTimes(1);
    expect(searchSpy).toHaveBeenCalledWith('ORD-100');
  });

  // G. Stale-clear test: pending request, clear to "", resolve old request -> old result NOT present
  it('G. prevents in-flight request from repopulating results after user clears input', async () => {
    let resolveOldRequest!: (val: GlobalSearchResponse) => void;
    const oldPromise = new Promise<GlobalSearchResponse>((resolve) => {
      resolveOldRequest = resolve;
    });

    const searchSpy = vi.spyOn(adminSearchApi, 'searchAdminGlobal').mockReturnValue(oldPromise);

    render(
      <MemoryRouter>
        <AdminSearchPalette isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    const input = screen.getByTestId('admin-search-input');
    fireEvent.change(input, { target: { value: 'ORD-100193' } });

    // Advance 250ms to trigger API call
    await act(async () => {
      vi.advanceTimersByTime(250);
    });

    expect(searchSpy).toHaveBeenCalledTimes(1);

    // User clears input to "" before response resolves
    fireEvent.change(input, { target: { value: '' } });

    // Now resolve the old delayed network response
    await act(async () => {
      resolveOldRequest({
        results: [
          {
            type: 'order',
            id: 'ord-100193-id',
            title: 'ORD-100193',
            subtitle: 'Old order',
            canonicalIdentifier: 'ORD-100193',
            navigationTarget: '/orders/ord-100193-id',
          },
        ],
      });
    });

    // Assert the old result was discarded and is NOT in the rendered DOM
    expect(screen.queryByText('ORD-100193')).toBeNull();
    expect(screen.queryByTestId('admin-search-item-order-ord-100193-id')).toBeNull();
  });

  // H. Stale-race test: slow query A then fast query B, resolve B first then A -> only B remains
  it('H. resolves race condition: newer query B prevails over slower older query A', async () => {
    let resolveA!: (val: GlobalSearchResponse) => void;
    const promiseA = new Promise<GlobalSearchResponse>((resolve) => {
      resolveA = resolve;
    });

    let resolveB!: (val: GlobalSearchResponse) => void;
    const promiseB = new Promise<GlobalSearchResponse>((resolve) => {
      resolveB = resolve;
    });

    const searchSpy = vi.spyOn(adminSearchApi, 'searchAdminGlobal').mockImplementation((q) => {
      if (q === 'query-A') return promiseA;
      if (q === 'query-B') return promiseB;
      return Promise.resolve({ results: [] });
    });

    render(
      <MemoryRouter>
        <AdminSearchPalette isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    const input = screen.getByTestId('admin-search-input');

    // Type query-A and advance debounce
    fireEvent.change(input, { target: { value: 'query-A' } });
    await act(async () => {
      vi.advanceTimersByTime(250);
    });
    expect(searchSpy).toHaveBeenCalledWith('query-A');

    // Type query-B and advance debounce
    fireEvent.change(input, { target: { value: 'query-B' } });
    await act(async () => {
      vi.advanceTimersByTime(250);
    });
    expect(searchSpy).toHaveBeenCalledWith('query-B');

    // Resolve B first (fast)
    await act(async () => {
      resolveB({
        results: [
          {
            type: 'product',
            id: 'prod-b',
            title: 'RESULT_B_TITLE',
            subtitle: 'Product B',
            canonicalIdentifier: 'B',
            navigationTarget: '/products/prod-b',
          },
        ],
      });
    });

    expect(screen.getByText('RESULT_B_TITLE')).toBeDefined();

    // Resolve A last (slow)
    await act(async () => {
      resolveA({
        results: [
          {
            type: 'order',
            id: 'ord-a',
            title: 'RESULT_A_TITLE',
            subtitle: 'Order A',
            canonicalIdentifier: 'A',
            navigationTarget: '/orders/ord-a',
          },
        ],
      });
    });

    // RESULT_A_TITLE must be discarded; only RESULT_B_TITLE remains
    expect(screen.queryByText('RESULT_A_TITLE')).toBeNull();
    expect(screen.getByText('RESULT_B_TITLE')).toBeDefined();
  });

  // I. Mock results across multiple types -> real DOM group headings
  it('I. renders grouped headings and formatted items across multiple result types', async () => {
    const mockResults: GlobalSearchResult[] = [
      {
        type: 'order',
        id: 'ord-1',
        title: 'ORD-100193',
        subtitle: 'Customer · Delivered',
        canonicalIdentifier: 'ORD-100193',
        navigationTarget: '/orders/ord-1',
      },
      {
        type: 'return',
        id: 'ret-1',
        title: 'ORD-100193',
        subtitle: 'Approved Return',
        canonicalIdentifier: 'ORD-100193',
        navigationTarget: '/returns',
      },
      {
        type: 'inventory_unit',
        id: 'zmu-1',
        title: 'ZMU-7K9M2X4P8R3V5W6Y',
        subtitle: 'Dev Coat · In Stock',
        canonicalIdentifier: 'ZMU-7K9M2X4P8R3V5W6Y',
        navigationTarget: '/inventory',
      },
      {
        type: 'product_variant',
        id: 'var-1',
        title: 'ZMK-9901',
        subtitle: 'Dev Coat M / Black',
        canonicalIdentifier: 'ZMK-9901',
        navigationTarget: '/products/prod-1',
      },
      {
        type: 'customer',
        id: 'cust-1',
        title: 'Nikita Osipov',
        subtitle: 'nikita@zamk.local',
        canonicalIdentifier: 'nikita@zamk.local',
        navigationTarget: '/users',
      },
    ];

    vi.spyOn(adminSearchApi, 'searchAdminGlobal').mockResolvedValue({
      results: mockResults,
    });

    render(
      <MemoryRouter>
        <AdminSearchPalette isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    const input = screen.getByTestId('admin-search-input');
    fireEvent.change(input, { target: { value: 'ORD' } });

    await act(async () => {
      vi.advanceTimersByTime(250);
    });

    // Check group headings in rendered DOM
    expect(screen.getByTestId('admin-search-group-orders')).toBeDefined();
    expect(screen.getByText('Заказы')).toBeDefined();

    expect(screen.getByTestId('admin-search-group-returns')).toBeDefined();
    expect(screen.getByText('Возвраты')).toBeDefined();

    expect(screen.getByTestId('admin-search-group-inventory')).toBeDefined();
    expect(screen.getByText('Склад')).toBeDefined();

    expect(screen.getByTestId('admin-search-group-products')).toBeDefined();
    expect(screen.getByText('Товары')).toBeDefined();

    expect(screen.getByTestId('admin-search-group-customers')).toBeDefined();
    expect(screen.getByText('Покупатели')).toBeDefined();
  });

  // J. ArrowDown / ArrowUp on real input -> actual selected row changes in rendered DOM
  it('J. updates active selected row in DOM on ArrowDown and ArrowUp', async () => {
    const mockResults: GlobalSearchResult[] = [
      {
        type: 'order',
        id: 'ord-1',
        title: 'ORD-1',
        subtitle: 'Order 1',
        canonicalIdentifier: 'ORD-1',
        navigationTarget: '/orders/ord-1',
      },
      {
        type: 'return',
        id: 'ret-1',
        title: 'RET-1',
        subtitle: 'Return 1',
        canonicalIdentifier: 'RET-1',
        navigationTarget: '/returns',
      },
    ];

    vi.spyOn(adminSearchApi, 'searchAdminGlobal').mockResolvedValue({
      results: mockResults,
    });

    render(
      <MemoryRouter>
        <AdminSearchPalette isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    const input = screen.getByTestId('admin-search-input');
    fireEvent.change(input, { target: { value: 'test' } });

    await act(async () => {
      vi.advanceTimersByTime(250);
    });

    const item1 = screen.getByTestId('admin-search-item-order-ord-1');
    const item2 = screen.getByTestId('admin-search-item-return-ret-1');

    // Initially item 1 is active (has border-l-2 border-indigo-600)
    expect(item1.className).toContain('border-indigo-600');
    expect(item2.className).not.toContain('border-indigo-600');

    // Press ArrowDown
    fireEvent.keyDown(input, { key: 'ArrowDown' });

    // Item 2 is now active
    expect(item2.className).toContain('border-indigo-600');
    expect(item1.className).not.toContain('border-indigo-600');

    // Press ArrowUp
    fireEvent.keyDown(input, { key: 'ArrowUp' });

    // Item 1 is active again
    expect(item1.className).toContain('border-indigo-600');
  });

  // K. Enter on selected result -> real router navigation
  it('K. executes router navigation on Enter key', async () => {
    let currentLocation = { pathname: '/', search: '' };
    const onClose = vi.fn();

    vi.spyOn(adminSearchApi, 'searchAdminGlobal').mockResolvedValue({
      results: [
        {
          type: 'order',
          id: 'ord-target-id',
          title: 'ORD-TARGET',
          subtitle: 'Target',
          canonicalIdentifier: 'ORD-TARGET',
          navigationTarget: '/orders/ord-target-id',
        },
      ],
    });

    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <LocationObserver onLocationChange={(loc) => { currentLocation = loc; }} />
        <AdminSearchPalette isOpen={true} onClose={onClose} />
      </MemoryRouter>
    );

    const input = screen.getByTestId('admin-search-input');
    fireEvent.change(input, { target: { value: 'ORD' } });

    await act(async () => {
      vi.advanceTimersByTime(250);
    });

    fireEvent.keyDown(input, { key: 'Enter' });

    expect(currentLocation.pathname).toBe('/orders/ord-target-id');
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  // L. Mouse click a result -> real navigation & onClose
  it('L. executes router navigation on mouse click and closes palette', async () => {
    let currentLocation = { pathname: '/', search: '' };
    const onClose = vi.fn();

    vi.spyOn(adminSearchApi, 'searchAdminGlobal').mockResolvedValue({
      results: [
        {
          type: 'return',
          id: 'ret-click-id',
          title: 'ORD-CLICK',
          subtitle: 'Return Click',
          canonicalIdentifier: 'ORD-CLICK',
          navigationTarget: '/returns',
        },
      ],
    });

    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <LocationObserver onLocationChange={(loc) => { currentLocation = loc; }} />
        <AdminSearchPalette isOpen={true} onClose={onClose} />
      </MemoryRouter>
    );

    const input = screen.getByTestId('admin-search-input');
    fireEvent.change(input, { target: { value: 'ORD' } });

    await act(async () => {
      vi.advanceTimersByTime(250);
    });

    const item = screen.getByTestId('admin-search-item-return-ret-click-id');
    fireEvent.click(item);

    expect(currentLocation.pathname).toBe('/returns');
    expect(currentLocation.search).toBe('?id=ret-click-id&orderNumber=ORD-CLICK');
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  // M. No results state after resolved []
  it('M. displays "Ничего не найдено" when API returns empty results', async () => {
    vi.spyOn(adminSearchApi, 'searchAdminGlobal').mockResolvedValue({
      results: [],
    });

    render(
      <MemoryRouter>
        <AdminSearchPalette isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    const input = screen.getByTestId('admin-search-input');
    fireEvent.change(input, { target: { value: 'UNKNOWN-QUERY' } });

    await act(async () => {
      vi.advanceTimersByTime(250);
    });

    expect(screen.getByTestId('admin-search-no-results')).toBeDefined();
    expect(screen.getByText('Ничего не найдено')).toBeDefined();
  });

  // N. API error state
  it('N. displays human error message and does not leak internal server details on error', async () => {
    vi.spyOn(adminSearchApi, 'searchAdminGlobal').mockRejectedValue({
      status: 403,
      code: 'forbidden',
    });

    render(
      <MemoryRouter>
        <AdminSearchPalette isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    const input = screen.getByTestId('admin-search-input');
    fireEvent.change(input, { target: { value: 'PRIVATE-DATA' } });

    await act(async () => {
      vi.advanceTimersByTime(250);
    });

    expect(screen.getByTestId('admin-search-error')).toBeDefined();
    expect(screen.getByText('Недостаточно прав для выполнения поиска.')).toBeDefined();
  });

  // O. Close and reopen starts clean
  it('O. resets input query and results when closed and reopened', async () => {
    vi.spyOn(adminSearchApi, 'searchAdminGlobal').mockResolvedValue({
      results: [
        {
          type: 'order',
          id: 'ord-1',
          title: 'ORD-RESET-TEST',
          subtitle: 'Subtitle',
          canonicalIdentifier: 'ORD-1',
          navigationTarget: '/orders/ord-1',
        },
      ],
    });

    const { rerender } = render(
      <MemoryRouter>
        <AdminSearchPalette isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    const input = screen.getByTestId('admin-search-input') as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'ORD-RESET' } });

    await act(async () => {
      vi.advanceTimersByTime(250);
    });

    expect(screen.getByText('ORD-RESET-TEST')).toBeDefined();

    // Close palette
    rerender(
      <MemoryRouter>
        <AdminSearchPalette isOpen={false} onClose={() => {}} />
      </MemoryRouter>
    );

    expect(screen.queryByTestId('admin-search-palette')).toBeNull();

    // Reopen palette
    rerender(
      <MemoryRouter>
        <AdminSearchPalette isOpen={true} onClose={() => {}} />
      </MemoryRouter>
    );

    const reopenedInput = screen.getByTestId('admin-search-input') as HTMLInputElement;
    expect(reopenedInput.value).toBe('');
    expect(screen.queryByText('ORD-RESET-TEST')).toBeNull();
  });
});
