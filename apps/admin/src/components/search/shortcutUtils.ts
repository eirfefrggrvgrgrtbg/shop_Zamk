export function isGlobalSearchShortcutEligible(e: KeyboardEvent, isSearchOpen: boolean): boolean {
  if (!((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k')) {
    return false;
  }

  // When palette is already open, shortcut is always eligible to toggle/close it
  if (isSearchOpen) {
    return true;
  }

  const target = e.target as HTMLElement | null;
  if (!target) {
    return true;
  }

  // If focus is inside the search palette itself, allow
  if (target.closest?.('[data-testid="admin-search-palette"]')) {
    return true;
  }

  // If user is actively typing in a normal page form control, do NOT hijack Cmd/Ctrl+K
  const tagName = target.tagName ? target.tagName.toUpperCase() : '';
  if (tagName === 'INPUT' || tagName === 'TEXTAREA' || tagName === 'SELECT') {
    return false;
  }

  if (target.isContentEditable || target.getAttribute?.('contenteditable') === 'true') {
    return false;
  }

  return true;
}
