import { ApiError } from './errors';
import { getAccessToken, setAccessToken, clearAccessToken } from './tokenStore';

export interface ApiClientConfig {
  baseURL: string;
}

let config: ApiClientConfig = {
  baseURL: 'http://127.0.0.1:8080/api',
};

export const createApiClient = (newConfig: ApiClientConfig) => {
  config = { ...config, ...newConfig };
};

export interface RequestOptions extends Omit<RequestInit, 'body'> {
  body?: any;
  params?: Record<string, string | number | boolean | undefined>;
  skipAuthRefresh?: boolean;
}

let refreshPromise: Promise<string | null> | null = null;

export const request = async <T>(
  method: string,
  path: string,
  options: RequestOptions = {}
): Promise<T> => {
  const execute = async (isRetry = false): Promise<any> => {
    const { body: inputBody, params, headers: optionHeaders, skipAuthRefresh, ...fetchOptions } = options;
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

    let response: Response;
    try {
      response = await fetch(url, {
        ...fetchOptions,
        method,
        headers,
        body,
        credentials: fetchOptions.credentials ?? 'include',
        signal: fetchOptions.signal ?? controller.signal,
      });
      clearTimeout(timeoutId);
    } catch (error) {
      clearTimeout(timeoutId);
      if (error instanceof Error && error.name === 'AbortError') {
        throw new ApiError('Сервер не отвечает. Проверьте, запущен ли backend.', 'TIMEOUT_ERROR');
      }
      throw new ApiError('Не удалось подключиться к серверу. Проверьте, запущен ли backend.', 'NETWORK_ERROR');
    }

    let data: any = null;
    const contentType = response.headers.get('content-type');
    if (contentType && contentType.includes('application/json')) {
      data = await response.json().catch(() => null);
    } else {
      const text = await response.text().catch(() => '');
      if (text) {
        try {
          data = JSON.parse(text);
        } catch {
          data = text;
        }
      }
    }

    if (!response.ok) {
      // 401 Unauthorized interceptor
      if (response.status === 401 && !skipAuthRefresh && !isRetry && path !== '/auth/login' && path !== '/auth/register' && path !== '/auth/refresh') {
        if (!refreshPromise) {
          refreshPromise = (async () => {
            try {
              const refreshOptions: RequestOptions = { skipAuthRefresh: true };
              const refreshRes = await request<any>('POST', '/auth/refresh', refreshOptions);
              if (refreshRes && refreshRes.accessToken) {
                setAccessToken(refreshRes.accessToken);
                return refreshRes.accessToken;
              }
              return null;
            } catch (err) {
              clearAccessToken();
              return null;
            } finally {
              refreshPromise = null;
            }
          })();
        }

        const newAccessToken = await refreshPromise;
        if (newAccessToken) {
          // Retry original request
          return execute(true);
        }
      }

      // Handle nested error shape: { error: { code, message } }
      if (data && data.error && typeof data.error === 'object') {
        const code = data.error.code;
        throw new ApiError(getSafeErrorMessage(code, data.error.message), code, response.status, data);
      }
      // Handle flat error shape: { error: "code", message: "..." } or { code: "code", message: "..." }
      if (data && (typeof data.error === 'string' || typeof data.code === 'string')) {
        const code = data.error || data.code;

        // Special logic for distinguishing Expired Session / Invalid Credentials
        if (response.status === 401) {
          if (path === '/auth/login') {
            throw new ApiError('Неверный email или пароль', 'INVALID_CREDENTIALS', 401, data);
          }
          if (path === '/auth/refresh') {
            throw new ApiError('Сессия истекла. Пожалуйста, войдите снова.', 'SESSION_EXPIRED', 401, data);
          }
        }

        throw new ApiError(getSafeErrorMessage(code, data.message), code, response.status, data);
      }

      throw new ApiError(data?.message || `HTTP Error ${response.status}`, data?.code || 'HTTP_ERROR', response.status, data);
    }

    return data;
  };

  return execute();
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
    case 'invalid_item':
      if (fallback?.includes('insufficient')) return 'Недостаточно товара на складе';
      if (fallback?.includes('variant')) return 'Выбранный вариант недоступен';
      return 'Товар недоступен для заказа';
    case 'insufficient_stock':
      return 'Недостаточно товара на складе';
    case 'invalid_return':
      if (fallback?.includes('window expired')) return 'Срок возврата истёк';
      if (fallback?.includes('not delivered')) return 'Заказ ещё не доставлен';
      if (fallback?.includes('quantity')) return 'Недопустимое количество для возврата';
      return 'Недопустимый возврат';
    case 'bad_request':
      return fallback || 'Некорректный запрос';
    default:
      if (fallback) {
        const lower = fallback.toLowerCase();
        if (lower.includes('duplicate review')) return 'Вы уже оставили отзыв на этот товар';
        if (lower.includes('not purchased')) return 'Вы можете оставить отзыв только на купленный товар';
        if (lower.includes('not delivered')) return 'Заказ ещё не доставлен';
        if (!lower.startsWith('http')) return fallback;
      }
      return 'Произошла ошибка. Попробуйте позже';
  }
};
