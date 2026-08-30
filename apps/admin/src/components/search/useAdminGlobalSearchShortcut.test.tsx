import { describe, it, expect } from 'vitest';
import { useState } from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { useAdminGlobalSearchShortcut } from './useAdminGlobalSearchShortcut';

function ShortcutTestHarness() {
  const [isOpen, setIsOpen] = useState(false);
  useAdminGlobalSearchShortcut(isOpen, setIsOpen);

  return (
    <div>
      <div data-testid="status">{isOpen ? 'OPEN' : 'CLOSED'}</div>
      <input data-testid="normal-input" type="text" placeholder="Normal input" />
      <textarea data-testid="normal-textarea" placeholder="Normal textarea" />
      <select data-testid="normal-select">
        <option value="1">Option 1</option>
      </select>
      <div data-testid="content-editable" contentEditable={true}>
        Editable text
      </div>
      {isOpen && (
        <div data-testid="admin-search-palette">
          <input data-testid="palette-input" type="text" placeholder="Search..." />
        </div>
      )}
    </div>
  );
}

describe('useAdminGlobalSearchShortcut (Real Hook & DOM Event Wiring Tests)', () => {
  it('opens palette on Meta+K from document body', () => {
    render(<ShortcutTestHarness />);
    expect(screen.getByTestId('status').textContent).toBe('CLOSED');

    fireEvent.keyDown(document.body, { key: 'k', metaKey: true });
    expect(screen.getByTestId('status').textContent).toBe('OPEN');
  });

  it('opens palette on Ctrl+K from document body', () => {
    render(<ShortcutTestHarness />);
    expect(screen.getByTestId('status').textContent).toBe('CLOSED');

    fireEvent.keyDown(document.body, { key: 'k', ctrlKey: true });
    expect(screen.getByTestId('status').textContent).toBe('OPEN');
  });

  it('does NOT open palette when Meta+K / Ctrl+K is pressed inside normal INPUT', () => {
    render(<ShortcutTestHarness />);
    const input = screen.getByTestId('normal-input');
    input.focus();

    fireEvent.keyDown(input, { key: 'k', metaKey: true });
    expect(screen.getByTestId('status').textContent).toBe('CLOSED');

    fireEvent.keyDown(input, { key: 'k', ctrlKey: true });
    expect(screen.getByTestId('status').textContent).toBe('CLOSED');
  });

  it('does NOT open palette when Meta+K / Ctrl+K is pressed inside normal TEXTAREA', () => {
    render(<ShortcutTestHarness />);
    const textarea = screen.getByTestId('normal-textarea');
    textarea.focus();

    fireEvent.keyDown(textarea, { key: 'k', metaKey: true });
    expect(screen.getByTestId('status').textContent).toBe('CLOSED');
  });

  it('does NOT open palette when Meta+K / Ctrl+K is pressed inside SELECT', () => {
    render(<ShortcutTestHarness />);
    const select = screen.getByTestId('normal-select');
    select.focus();

    fireEvent.keyDown(select, { key: 'k', metaKey: true });
    expect(screen.getByTestId('status').textContent).toBe('CLOSED');
  });

  it('does NOT open palette when Meta+K / Ctrl+K is pressed inside contentEditable div', () => {
    render(<ShortcutTestHarness />);
    const editable = screen.getByTestId('content-editable');
    editable.focus();

    fireEvent.keyDown(editable, { key: 'k', metaKey: true });
    expect(screen.getByTestId('status').textContent).toBe('CLOSED');
  });

  it('toggles/closes palette when Meta+K is pressed while palette is already open', () => {
    render(<ShortcutTestHarness />);
    // Open palette
    fireEvent.keyDown(document.body, { key: 'k', metaKey: true });
    expect(screen.getByTestId('status').textContent).toBe('OPEN');

    const paletteInput = screen.getByTestId('palette-input');
    paletteInput.focus();

    // Press Meta+K inside search palette input
    fireEvent.keyDown(paletteInput, { key: 'k', metaKey: true });
    expect(screen.getByTestId('status').textContent).toBe('CLOSED');
  });
});
