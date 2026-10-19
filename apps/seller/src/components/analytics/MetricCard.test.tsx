import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MetricCard } from './MetricCard';

describe('MetricCard - Option A UI and Accessibility', () => {
  it('renders title and formatted value', () => {
    render(
      <MetricCard
        title="Выручка"
        value="1 500 ₽"
      />
    );

    expect(screen.getByText('Выручка')).toBeTruthy();
    expect(screen.getByText('1 500 ₽')).toBeTruthy();
  });

  it('renders positive, negative, unchanged, and new comparison states correctly', () => {
    const { rerender } = render(
      <MetricCard
        title="Заказы"
        value="10 шт."
        comparisonState="positive"
        changePercent={25.5}
      />
    );
    expect(screen.getByText('25,5%')).toBeTruthy();

    rerender(
      <MetricCard
        title="Заказы"
        value="10 шт."
        comparisonState="negative"
        changePercent={-12.3}
      />
    );
    expect(screen.getByText('12,3%')).toBeTruthy();

    rerender(
      <MetricCard
        title="Заказы"
        value="10 шт."
        comparisonState="unchanged"
        changePercent={0}
      />
    );
    expect(screen.getByText('0%')).toBeTruthy();

    rerender(
      <MetricCard
        title="Заказы"
        value="10 шт."
        comparisonState="new"
      />
    );
    expect(screen.getByText('Новое')).toBeTruthy();
  });

  it('handles null, undefined, NaN, and Infinity changePercent gracefully without rendering NaN%', () => {
    const { rerender, container } = render(
      <MetricCard
        title="Заказы"
        value="5 шт."
        comparisonState="positive"
        changePercent={null}
      />
    );
    expect(container.textContent).not.toMatch(/NaN/);

    rerender(
      <MetricCard
        title="Заказы"
        value="5 шт."
        comparisonState="positive"
        changePercent={NaN}
      />
    );
    expect(container.textContent).not.toMatch(/NaN/);

    rerender(
      <MetricCard
        title="Заказы"
        value="5 шт."
        comparisonState="positive"
        changePercent={Infinity}
      />
    );
    expect(container.textContent).not.toMatch(/Infinity/);
  });

  it('renders accessible explanation tooltip with keyboard and screen reader support', () => {
    render(
      <MetricCard
        title="Заказы"
        value="3 шт."
        explanation="Оплаченные покупателями заказы за выбранный период. Включая ещё не доставленные. Отменённые после оплаты сохраняются в истории."
      />
    );

    const helpBtn = screen.getByRole('button', { name: /Справка: Заказы/i });
    expect(helpBtn).toBeTruthy();
    expect(helpBtn.getAttribute('title')).toBe(
      'Оплаченные покупателями заказы за выбранный период. Включая ещё не доставленные. Отменённые после оплаты сохраняются в истории.'
    );

    // Click button to open tooltip
    fireEvent.click(helpBtn);
    const tooltip = screen.getByRole('tooltip');
    expect(tooltip).toBeTruthy();
    expect(tooltip.textContent).toContain('Оплаченные покупателями заказы');

    // Press Escape to dismiss tooltip
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(screen.queryByRole('tooltip')).toBeNull();
  });

  it('renders supporting text when provided (e.g. delivered orders on AOV)', () => {
    const { rerender } = render(
      <MetricCard
        title="Средний чек"
        value="2 500 ₽"
        supportingText="Доставлено заказов: 4 шт."
      />
    );

    expect(screen.getByText('Доставлено заказов: 4 шт.')).toBeTruthy();

    rerender(
      <MetricCard
        title="Средний чек"
        value="2 500 ₽"
      />
    );

    expect(screen.queryByText(/Доставлено заказов/)).toBeNull();
  });
});
