import { createApiClient } from '@zamk/api-client/src/client';

export const resolveApiUrl = (): string => {
  const envUrl = (typeof import.meta !== 'undefined' && import.meta.env && import.meta.env.VITE_API_URL) || 'http://127.0.0.1:8080/api';
  if (typeof window !== 'undefined' && window.location) {
    try {
      const parsed = new URL(envUrl);
      if (
        (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1') &&
        (parsed.hostname === '127.0.0.1' || parsed.hostname === 'localhost')
      ) {
        parsed.hostname = window.location.hostname;
        return parsed.origin + parsed.pathname.replace(/\/$/, '');
      }
    } catch {
      // ignore parsing error
    }
  }
  return envUrl;
};

export const API_URL = resolveApiUrl();

createApiClient({ baseURL: API_URL });
