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
  const { body: inputBody, params, headers: optionHeaders, ...fetchOptions } = options;
  let url = `${config.baseURL}${path}`;

  if (params) {
    const searchParams = new URLSearchParams();
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined && value !== null) {
        searchParams.append(key, String(value));
      }
    }
    const queryString = searchParams.toString();
    if (queryString) {
      url += `?${queryString}`;
    }
  }

  const headers = new Headers(optionHeaders || {});

  // Determine if body is JSON or FormData
  let body: BodyInit | null = null;
  if (inputBody instanceof FormData) {
    body = inputBody;
    // Don't set Content-Type for FormData, the browser sets it automatically with the boundary
  } else if (inputBody !== undefined && inputBody !== null) {
    body = JSON.stringify(inputBody);
    headers.set('Content-Type', 'application/json');
  }

  const accessToken = getAccessToken();
  if (accessToken) {
    headers.set('Authorization', `Bearer ${accessToken}`);
  }

  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), 15_000);

  try {
    const response = await fetch(url, {
      ...fetchOptions,
      method,
      headers,
      body,
      credentials: fetchOptions.credentials ?? 'include', // Important for refresh token httpOnly cookies
      signal: fetchOptions.signal ?? controller.signal,
    });
    clearTimeout(timeoutId);

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
      // Handle nested error shape: { error: { code, message } }
      if (data && data.error && typeof data.error === 'object') {
        const code = data.error.code;
        throw new ApiError(getSafeErrorMessage(code, data.error.message), code, response.status);
      }
      // Handle flat error shape: { error: "code", message: "..." }
      if (data && typeof data.error === 'string') {
        const code = data.error;
        throw new ApiError(getSafeErrorMessage(code, data.message), code, response.status);
      }
      throw new ApiError(`HTTP Error ${response.status}`, 'HTTP_ERROR', response.status);
    }

    return data as T;
  } catch (error) {
    clearTimeout(timeoutId);
    if (error instanceof ApiError) {
      throw error;
    }
    if (error instanceof Error && error.name === 'AbortError') {
      throw new ApiError(
        'Сервер не отвечает. Проверьте, запущен ли backend.',
        'TIMEOUT_ERROR'
      );
    }
    throw new ApiError(
      'Не удалось подключиться к серверу. Проверьте, запущен ли backend.',
      'NETWORK_ERROR'
    );
  }
};

const getSafeErrorMessage = (code?: string, fallback?: string): string => {
  switch (code) {
    case 'insufficient_permissions':
      return 'Недостаточно прав для выполнения действия.';
    case 'invalid_request':
      return 'Проверьте правильность заполнения формы';
    case 'validation_error':
      return fallback?.toLowerCase().includes('password')
        ? 'Проверьте пароль (минимум 8 символов)'
        : 'Проверьте правильность заполнения формы';
    case 'duplicate_email':
      return 'Пользователь с таким email уже существует';
    case 'invalid_credentials':
      return 'Неверный email или пароль';
    case 'unauthorized':
      return 'Необходима авторизация. Войдите в аккаунт';
    case 'forbidden':
      return 'Доступ запрещён';
    case 'not_found':
      return 'Ресурс не найден';
    case 'internal_error':
      return 'Произошла ошибка на сервере. Попробуйте позже';
    case 'bad_request':
      return fallback || 'Некорректный запрос';
    default:
      return fallback && !fallback.toLowerCase().startsWith('http')
        ? fallback
        : 'Произошла ошибка. Попробуйте позже';
  }
};
