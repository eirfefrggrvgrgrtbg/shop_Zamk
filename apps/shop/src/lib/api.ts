import { createApiClient } from '@zamk/api-client';

const baseURL = import.meta.env.VITE_API_URL || 'http://localhost:8080/api';

export const apiClient = createApiClient({ baseURL });
