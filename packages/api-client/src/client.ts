import { ApiError } from './errors';
import { getAccessToken } from './tokenStore';

export interface ApiClientConfig {
  baseURL: string;
}

let config: ApiClientConfig = {
  baseURL: 'http://localhost:8080/api',
};

export const createApiClient = (newConfig: ApiClientConfig) => {
  config = { ...config, ...newConfig };
};

export interface RequestOptions extends Omit<RequestInit, 'body'> {
  body?: any;
  params?: Record<string, string | number | boolean | undefined>;
}

export const request = async <T>(
  method: string,
  path: string,
  options: RequestOptions = {}
): Promise<T> => {
  let url = `${config.baseURL}${path}`;

  if (options.params) {
    const searchParams = new URLSearchParams();
    for (const [key, value] of Object.entries(options.params)) {
      if (value !== undefined && value !== null) {
        searchParams.append(key, String(value));
      }
    }
    const queryString = searchParams.toString();
    if (queryString) {
      url += `?${queryString}`;
    }
  }

  const headers = new Headers(options.headers || {});

  // Determine if body is JSON or FormData
  let body: BodyInit | null = null;
  if (options.body instanceof FormData) {
    body = options.body;
    // Don't set Content-Type for FormData, the browser sets it automatically with the boundary
  } else if (options.body) {
    body = JSON.stringify(options.body);
    headers.set('Content-Type', 'application/json');
  }

  const accessToken = getAccessToken();
  if (accessToken) {
    headers.set('Authorization', `Bearer ${accessToken}`);
  }

  try {
    const response = await fetch(url, {
      method,
      headers,
      body,
      credentials: 'include', // Important for refresh token httpOnly cookies
      ...options,
    });

    let data: any = null;
    const contentType = response.headers.get('content-type');
    if (contentType && contentType.includes('application/json')) {
      data = await response.json();
    } else {
      const text = await response.text();
      if (text) {
        try {
          data = JSON.parse(text);
        } catch {
          data = text;
        }
      }
    }

    if (!response.ok) {
      // Handle standard backend error shape
      if (data && data.error && typeof data.error === 'object') {
        throw new ApiError(data.error.message || 'API Error', data.error.code, response.status);
      }
      throw new ApiError(`HTTP Error ${response.status}`, 'HTTP_ERROR', response.status);
    }

    return data as T;
  } catch (error) {
    if (error instanceof ApiError) {
      throw error;
    }
    throw new ApiError(
      error instanceof Error ? error.message : 'Network Error',
      'NETWORK_ERROR'
    );
  }
};
