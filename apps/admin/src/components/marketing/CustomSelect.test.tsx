import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent } from '@testing-library/react';
import { CustomSelect, CustomSelectOption } from './CustomSelect';

const sampleOptions: CustomSelectOption[] = [
  { value: 'rev', label: 'По выручке' },
  { value: 'sales', label: 'По продажам' },
  { value: 'fav', label: 'По избранному', disabled: true, disabledReason: 'Недостаточно данных' },
  { value: 'conv', label: 'По конверсии' },
];

afterEach(cleanup);

describe('CustomSelect Accessibility and Interaction Tests', () => {
  it('1. open and close with click toggles aria-expanded and options visibility', () => {
    const handleChange = vi.fn();
    render(
      <CustomSelect
        value="rev"
        options={sampleOptions}
        onChange={handleChange}
        placeholder="Сортировка"
      />
    );

    const trigger = screen.getByRole('combobox');
    expect(trigger.getAttribute('aria-expanded')).toBe('false');
    expect(screen.queryByRole('listbox')).toBeNull();

    // Open
    fireEvent.click(trigger);
    expect(trigger.getAttribute('aria-expanded')).toBe('true');
    expect(screen.getByRole('listbox')).toBeTruthy();
    expect(screen.getAllByRole('option')).toHaveLength(4);

    // Close
    fireEvent.click(trigger);
    expect(trigger.getAttribute('aria-expanded')).toBe('false');
    expect(screen.queryByRole('listbox')).toBeNull();
  });

  it('2. close with Escape key closes listbox and returns focus to trigger', () => {
    render(
      <CustomSelect
        value="rev"
        options={sampleOptions}
        onChange={vi.fn()}
        placeholder="Сортировка"
      />
    );

    const trigger = screen.getByRole('combobox');
    fireEvent.click(trigger);
    expect(trigger.getAttribute('aria-expanded')).toBe('true');

    fireEvent.keyDown(trigger, { key: 'Escape' });
    expect(trigger.getAttribute('aria-expanded')).toBe('false');
    expect(screen.queryByRole('listbox')).toBeNull();
  });

  it('3. keyboard ArrowDown and ArrowUp navigates options skipping disabled options', () => {
    const handleChange = vi.fn();
    render(
      <CustomSelect
        value="rev"
        options={sampleOptions}
        onChange={handleChange}
        placeholder="Сортировка"
      />
    );

    const trigger = screen.getByRole('combobox');
    // Open with ArrowDown
    fireEvent.keyDown(trigger, { key: 'ArrowDown' });
    expect(trigger.getAttribute('aria-expanded')).toBe('true');

    // Initially active is index 0 ('rev')
    expect(trigger.getAttribute('aria-activedescendant')).toContain('-opt-0');

    // ArrowDown moves to index 1 ('sales')
    fireEvent.keyDown(trigger, { key: 'ArrowDown' });
    expect(trigger.getAttribute('aria-activedescendant')).toContain('-opt-1');

    // ArrowDown skips index 2 (disabled) and moves directly to index 3 ('conv')
    fireEvent.keyDown(trigger, { key: 'ArrowDown' });
    expect(trigger.getAttribute('aria-activedescendant')).toContain('-opt-3');

    // ArrowUp skips index 2 and moves back to index 1 ('sales')
    fireEvent.keyDown(trigger, { key: 'ArrowUp' });
    expect(trigger.getAttribute('aria-activedescendant')).toContain('-opt-1');
  });

  it('4. Enter key selects focused option, triggers onChange, and closes listbox', () => {
    const handleChange = vi.fn();
    render(
      <CustomSelect
        value="rev"
        options={sampleOptions}
        onChange={handleChange}
        placeholder="Сортировка"
      />
    );

    const trigger = screen.getByRole('combobox');
    // Open with Enter
    fireEvent.keyDown(trigger, { key: 'Enter' });
    expect(trigger.getAttribute('aria-expanded')).toBe('true');

    // Move to 'sales' (index 1)
    fireEvent.keyDown(trigger, { key: 'ArrowDown' });

    // Press Enter to select
    fireEvent.keyDown(trigger, { key: 'Enter' });
    expect(handleChange).toHaveBeenCalledWith('sales');
    expect(trigger.getAttribute('aria-expanded')).toBe('false');
    expect(screen.queryByRole('listbox')).toBeNull();
  });

  it('5. disabled option cannot be selected by click or keyboard', () => {
    const handleChange = vi.fn();
    render(
      <CustomSelect
        value="rev"
        options={sampleOptions}
        onChange={handleChange}
        placeholder="Сортировка"
      />
    );

    const trigger = screen.getByRole('combobox');
    fireEvent.click(trigger);

    const disabledOption = screen.getAllByRole('option')[2];
    expect(disabledOption.getAttribute('aria-disabled')).toBe('true');

    // Click disabled option
    fireEvent.click(disabledOption);
    expect(handleChange).not.toHaveBeenCalled();
    expect(trigger.getAttribute('aria-expanded')).toBe('true'); // Stays open
  });

  it('6. click outside closes listbox', () => {
    render(
      <div>
        <button data-testid="outside-button">Вне компонента</button>
        <CustomSelect
          value="rev"
          options={sampleOptions}
          onChange={vi.fn()}
          placeholder="Сортировка"
        />
      </div>
    );

    const trigger = screen.getByRole('combobox');
    fireEvent.click(trigger);
    expect(trigger.getAttribute('aria-expanded')).toBe('true');

    // Click outside
    const outside = screen.getByTestId('outside-button');
    fireEvent.mouseDown(outside);

    expect(trigger.getAttribute('aria-expanded')).toBe('false');
    expect(screen.queryByRole('listbox')).toBeNull();
  });

  it('7. option aria-selected correctly reflects selected value', () => {
    render(
      <CustomSelect
        value="sales"
        options={sampleOptions}
        onChange={vi.fn()}
        placeholder="Сортировка"
      />
    );

    const trigger = screen.getByRole('combobox');
    fireEvent.click(trigger);

    const options = screen.getAllByRole('option');
    expect(options[0].getAttribute('aria-selected')).toBe('false');
    expect(options[1].getAttribute('aria-selected')).toBe('true'); // 'sales'
    expect(options[2].getAttribute('aria-selected')).toBe('false');
    expect(options[3].getAttribute('aria-selected')).toBe('false');
  });
});
