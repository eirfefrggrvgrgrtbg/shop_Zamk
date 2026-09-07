/**
 * Sanitizes and validates return paths to ensure internal navigation only,
 * preventing open-redirect vulnerabilities.
 */
export function getSafeReturnPath(from: unknown, fallback = '/dashboard'): string {
  let target = '';

  if (typeof from === 'string') {
    target = from.trim();
  } else if (from && typeof from === 'object' && 'pathname' in from) {
    const loc = from as { pathname?: string; search?: string; hash?: string };
    if (typeof loc.pathname === 'string') {
      target = `${loc.pathname}${loc.search || ''}${loc.hash || ''}`.trim();
    }
  }

  if (!target) {
    return fallback;
  }

  // Must begin with a single '/' and not '//' (protocol-relative) or '/\'
  if (!target.startsWith('/') || target.startsWith('//') || target.startsWith('/\\')) {
    return fallback;
  }

  // Prevent control characters or backslashes
  if (/[\r\n\t\\]/.test(target)) {
    return fallback;
  }

  // Prevent protocol / scheme injections (e.g. '/foo:bar' before any query/hash)
  const pathPart = target.split(/[?#]/)[0];
  if (pathPart.includes(':')) {
    return fallback;
  }

  return target;
}
