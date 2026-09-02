let lockCount = 0;
let originalOverflow = '';

export function lockBodyScroll(): void {
  if (typeof document === 'undefined') return;
  if (lockCount === 0) {
    originalOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
  }
  lockCount++;
}

export function unlockBodyScroll(): void {
  if (typeof document === 'undefined') return;
  if (lockCount > 0) {
    lockCount--;
    if (lockCount === 0) {
      document.body.style.overflow = originalOverflow || '';
    }
  }
}

export function getScrollLockCount(): number {
  return lockCount;
}

export function resetScrollLockForTesting(): void {
  lockCount = 0;
  originalOverflow = '';
}
