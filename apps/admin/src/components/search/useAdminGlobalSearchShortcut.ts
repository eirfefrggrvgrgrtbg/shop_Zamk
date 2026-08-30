import { useEffect } from 'react';
import { isGlobalSearchShortcutEligible } from './shortcutUtils';

export function useAdminGlobalSearchShortcut(
  isSearchOpen: boolean,
  setIsSearchOpen: (open: boolean | ((prev: boolean) => boolean)) => void
) {
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (isGlobalSearchShortcutEligible(e, isSearchOpen)) {
        e.preventDefault();
        setIsSearchOpen((prev) => !prev);
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isSearchOpen, setIsSearchOpen]);
}
