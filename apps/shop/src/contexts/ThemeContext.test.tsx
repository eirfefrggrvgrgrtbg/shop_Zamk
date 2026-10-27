/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest';
import { render, screen, cleanup, act } from '@testing-library/react';
import { ThemeProvider, useTheme } from './ThemeContext';

function mockMatchMedia(prefersDark: boolean) {
  window.matchMedia = ((q: string) => ({
    matches: q.includes('dark') ? prefersDark : false,
    media: q,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  })) as unknown as typeof window.matchMedia;
}

function Probe() {
  const { theme, isDark, setTheme } = useTheme();
  return (
    <div>
      <span data-testid="t">{theme}:{String(isDark)}</span>
      <button onClick={() => setTheme('dark')}>dark</button>
    </div>
  );
}

function installStorage() {
  const data = new Map<string, string>();
  vi.stubGlobal('localStorage', {
    getItem: (k: string) => (data.has(k) ? data.get(k)! : null),
    setItem: (k: string, v: string) => void data.set(k, String(v)),
    removeItem: (k: string) => void data.delete(k),
    clear: () => data.clear(),
  });
}

describe('Theme: light by default, chosen dark is kept', () => {
  beforeEach(() => {
    installStorage();
    document.documentElement.classList.remove('dark');
  });
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it.each([false, true])('is light when nothing is stored (OS prefers dark: %s)', (osDark) => {
    mockMatchMedia(osDark);
    render(<ThemeProvider><Probe /></ThemeProvider>);
    expect(document.documentElement.classList.contains('dark')).toBe(false);
    expect(screen.getByTestId('t').textContent).toBe('light:false');
    expect(localStorage.getItem('zamk_theme')).toBe('light');
  });

  it('keeps an explicit stored "system" choice and follows the OS', () => {
    mockMatchMedia(true);
    localStorage.setItem('zamk_theme', 'system');
    render(<ThemeProvider><Probe /></ThemeProvider>);
    expect(screen.getByTestId('t').textContent).toBe('system:true');
    expect(document.documentElement.classList.contains('dark')).toBe(true);
  });

  it('keeps a stored dark choice across remounts (reload) and does not reset it', () => {
    mockMatchMedia(false);
    localStorage.setItem('zamk_theme', 'dark');
    const first = render(<ThemeProvider><Probe /></ThemeProvider>);
    expect(document.documentElement.classList.contains('dark')).toBe(true);
    first.unmount();
    render(<ThemeProvider><Probe /></ThemeProvider>);
    expect(localStorage.getItem('zamk_theme')).toBe('dark');
    expect(screen.getByTestId('t').textContent).toBe('dark:true');
  });

  it('switching to dark persists the choice', () => {
    mockMatchMedia(false);
    render(<ThemeProvider><Probe /></ThemeProvider>);
    act(() => screen.getByText('dark').click());
    expect(localStorage.getItem('zamk_theme')).toBe('dark');
    expect(document.documentElement.classList.contains('dark')).toBe(true);
  });
});
