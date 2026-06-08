import { createApiClient } from '@zamk/api-client/src/client';

export const API_URL = import.meta.env.VITE_API_URL || 'http://127.0.0.1:8080/api';

createApiClient({ baseURL: API_URL });
