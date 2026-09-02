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

      // Handle plain string error response
      if (typeof data === 'string' && data.trim()) {
        const trimmed = data.trim();
        const code = response.status === 409 ? 'duplicate_review' : trimmed;
        throw new ApiError(getSafeErrorMessage(code, trimmed), code, response.status, data);
      }

      if (response.status === 409) {
        throw new ApiError(getSafeErrorMessage('duplicate_review', 'duplicate review'), 'duplicate_review', 409, data);
      }

      throw new ApiError(data?.message || getSafeErrorMessage(data?.code, `HTTP Error ${response.status}`), data?.code || 'HTTP_ERROR', response.status, data);
    }

    return data;
  };

  return execute();
};

export const getSafeErrorMessage = (code?: string, fallback?: string): string => {
  switch (code) {
    case 'supply_carrier_required':
    case 'SUPPLY_CARRIER_REQUIRED':
      return 'Укажите транспортную компанию.';
    case 'supply_carrier_unsupported':
    case 'SUPPLY_CARRIER_UNSUPPORTED':
      return 'Выберите поддерживаемую транспортную компанию.';
    case 'supply_tracking_number_required':
    case 'SUPPLY_TRACKING_NUMBER_REQUIRED':
      return 'Укажите трек-номер отправления.';
    case 'supply_items_required':
    case 'SUPPLY_ITEMS_REQUIRED':
      return 'Укажите количество хотя бы для одного товара.';
    case 'supply_receiving_lookup_failed':
    case 'SUPPLY_RECEIVING_LOOKUP_FAILED':
      return 'Ошибка поиска поставки.';
    case 'supply_not_found':
    case 'SUPPLY_NOT_FOUND':
      return 'Поставка или грузоместо не найдено.';
    case 'supply_not_arrived':
    case 'SUPPLY_NOT_ARRIVED':
      return 'Поставка ещё не прибыла на склад.';
    case 'supply_not_ready_for_receiving':
    case 'SUPPLY_NOT_READY_FOR_RECEIVING':
      return 'Поставка ещё не готова к приёмке.';
    case 'supply_already_completed':
    case 'SUPPLY_ALREADY_COMPLETED':
      return 'Приёмка по этой поставке уже завершена.';
    case 'supply_cancelled':
    case 'SUPPLY_CANCELLED':
      return 'Поставка отменена.';
    case 'receiving_session_already_active':
    case 'RECEIVING_SESSION_ALREADY_ACTIVE':
      return 'Для этой поставки уже открыта приёмка.';
    case 'invalid_receiving_code':
    case 'INVALID_RECEIVING_CODE':
      return 'Введите номер поставки, грузоместа или отсканируйте QR-код.';
    case 'supply_unit_identity_mismatch':
    case 'SUPPLY_UNIT_IDENTITY_MISMATCH':
      return 'Идентификаторы товарных единиц не совпадают с составом поставки.';
    case 'unit_already_scanned':
    case 'UNIT_ALREADY_SCANNED':
      return 'Эта единица уже отсканирована.';
    case 'unit_not_found':
    case 'UNIT_NOT_FOUND':
      return fallback || 'Физическая единица с таким кодом не найдена.';
    case 'unit_not_in_supply':
    case 'UNIT_NOT_IN_SUPPLY':
      return 'Эта единица относится к другой поставке.';
    case 'serialized_unit_code_required':
    case 'SERIALIZED_UNIT_CODE_REQUIRED':
      return 'Для этой поставки сканируйте уникальную этикетку ZMU.';
    case 'scan_not_found':
    case 'SCAN_NOT_FOUND':
      return 'Скан не найден.';
    case 'scan_already_voided':
    case 'SCAN_ALREADY_VOIDED':
      return 'Этот скан уже был отменён.';
    case 'scan_not_in_session':
    case 'SCAN_NOT_IN_SESSION':
      return 'Скан не принадлежит этой сессии.';
    case 'receiving_session_finalized':
    case 'RECEIVING_SESSION_FINALIZED':
      return 'Сессия приёмки уже завершена.';
    case 'invalid_receiving_condition':
    case 'INVALID_RECEIVING_CONDITION':
      return 'Недопустимое состояние товара (допустимо: ok или damaged).';
    case 'supply_not_serialized':
    case 'SUPPLY_NOT_SERIALIZED':
      return 'Эта поставка использует старую схему приёмки по ZMK.';
    case 'supply_not_shipped':
    case 'SUPPLY_NOT_SHIPPED':
      return 'Поставка еще не отправлена продавцом.';
    case 'supply_invalid_status':
    case 'SUPPLY_INVALID_STATUS':
      return 'Недопустимый статус поставки для выполнения действия.';
    case 'product_media_portrait_required':
    case 'PRODUCT_MEDIA_PORTRAIT_REQUIRED':
      return 'Для товара нужны вертикальные фотографии.\nЗагрузите изображение в вертикальном формате.';
    case 'product_media_too_small':
    case 'PRODUCT_MEDIA_TOO_SMALL':
      return 'Изображение слишком маленькое.\nМинимальный размер — 800×1000 пикселей.';
    case 'product_media_invalid_crop':
    case 'PRODUCT_MEDIA_INVALID_CROP':
    case 'product_media_invalid_aspect':
    case 'PRODUCT_MEDIA_INVALID_ASPECT':
      return 'Не удалось сохранить кадр 4:5.\nПопробуйте выбрать область фотографии ещё раз.';
    case 'product_media_not_ready':
    case 'PRODUCT_MEDIA_NOT_READY':
      return 'Все фотографии товара должны быть настроены в формате 4:5 в разделе «Фото».';
    case 'product_main_image_missing':
    case 'PRODUCT_MAIN_IMAGE_MISSING':
      return 'Выберите главное фото товара в разделе «Фото».';
    case 'product_media_required':
    case 'PRODUCT_MEDIA_REQUIRED':
      return 'Загрузите хотя бы одну фотографию товара в разделе «Фото».';
    case 'product_category_required':
    case 'PRODUCT_CATEGORY_REQUIRED':
      return 'Выберите категорию товара в разделе «Категория».';
    case 'product_variants_required':
    case 'PRODUCT_VARIANTS_REQUIRED':
      return 'Добавьте хотя бы один вариант товара в разделе «Варианты».';
    case 'product_price_invalid':
    case 'PRODUCT_PRICE_INVALID':
      return 'Укажите цену для всех вариантов товара в разделе «Цена».';
    case 'product_sku_required':
    case 'PRODUCT_SKU_REQUIRED':
      return 'Заполните артикул продавца (SKU) для всех вариантов товара в разделе «Цена».';
    case 'product_size_chart_required':
    case 'PRODUCT_SIZE_CHART_REQUIRED':
      return 'Заполните таблицу размеров в разделе «Таблица размеров».';
    case 'product_size_chart_incomplete':
    case 'PRODUCT_SIZE_CHART_INCOMPLETE':
      return 'Заполните все обязательные параметры в разделе «Таблица размеров».';
    case 'product_composition_invalid':
    case 'PRODUCT_COMPOSITION_INVALID':
      return 'Проверьте состав материалов в разделе «Характеристики» (сумма должна составлять ровно 100%).';
    case 'product_required_attribute_missing':
    case 'PRODUCT_REQUIRED_ATTRIBUTE_MISSING':
      return fallback || 'Заполните обязательные характеристики товара в разделе «Характеристики».';
    case 'insufficient_permissions':
      return 'Недостаточно прав для выполнения действия.';
    case 'invalid_request':
      return 'Проверьте правильность заполнения формы';
    case 'validation_error':
      if (fallback) {
        if (fallback.toLowerCase().includes('password')) return 'Проверьте пароль (минимум 8 символов)';
        if (fallback.includes('4:5 renditions')) return 'Все фотографии товара должны быть настроены в формате 4:5 в разделе «Фото».';
        if (fallback.includes('main image is required')) return 'Выберите главное фото товара в разделе «Фото».';
        if (fallback.includes('at least one image is required')) return 'Загрузите хотя бы одну фотографию товара в разделе «Фото».';
        if (fallback.includes('variant price')) return 'Укажите цену для всех вариантов товара в разделе «Цена».';
        if (fallback.includes('variant seller SKU')) return 'Заполните артикул продавца (SKU) для всех вариантов товара в разделе «Цена».';
        if (fallback.includes('size chart')) return 'Заполните таблицу размеров в разделе «Таблица размеров».';
        if (fallback.includes('material composition')) return 'Проверьте состав материалов в разделе «Характеристики» (сумма должна составлять ровно 100%).';
        if (!fallback.startsWith('http') && !fallback.includes('{')) return fallback;
      }
      return 'Проверьте правильность заполнения формы';
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
    case 'invalid_quantity':
    case 'invalid return quantity':
    case 'INVALID_QUANTITY':
      return 'На это количество товара уже оформлена заявка на возврат.';
    case 'comment_required':
    case 'comment is required':
    case 'COMMENT_REQUIRED':
      return 'Пожалуйста, опишите причину возврата в комментарии.';
    case 'evidence_required':
    case '2 to 6 photos required for this return reason':
    case 'EVIDENCE_REQUIRED':
      return 'Для этой причины возврата необходимо прикрепить фотографии товара.';
    case 'evidence_too_many':
    case 'maximum 6 photos allowed':
    case 'EVIDENCE_TOO_MANY':
      return 'Максимум 6 фотографий.';
    case 'order_not_delivered':
    case 'can only return delivered orders':
    case 'ORDER_NOT_DELIVERED':
      if (fallback && (fallback.toLowerCase().includes('review') || fallback.toLowerCase().includes('отзыв'))) {
        return 'Отзыв можно оставить только после доставки заказа';
      }
      return 'Возврат возможен только для доставленных заказов.';
    case 'return_window_expired':
    case 'return window has expired':
    case 'RETURN_WINDOW_EXPIRED':
      return 'Срок оформления возврата для этого заказа истёк.';
    case 'invalid_return':
      if (fallback?.includes('window expired')) return 'Срок возврата истёк';
      if (fallback?.includes('not delivered')) return 'Заказ ещё не доставлен';
      if (fallback?.includes('quantity')) return 'На это количество товара уже оформлена заявка на возврат.';
      return 'Недопустимый возврат';
    case 'bad_request':
      return fallback || 'Некорректный запрос';
    default:
      if (fallback) {
        const lower = fallback.toLowerCase();
        if (lower.includes('duplicate review') || lower.includes('review already exists')) return 'Вы уже оставили отзыв на этот товар';
        if (lower.includes('not purchased') || lower.includes('item not purchased') || lower.includes('was not purchased')) return 'Вы можете оставить отзыв только на купленный товар';
        if (lower.includes('not delivered') || lower.includes('order not delivered') || lower.includes('must be delivered')) return 'Отзыв можно оставить только после доставки заказа';
        if (lower.includes('too long') || lower.includes('max 1000')) return 'Текст отзыва слишком длинный (максимум 1000 символов)';
        if (lower.includes('rating') && (lower.includes('between 1 and 5') || lower.includes('invalid rating'))) return 'Оценка должна быть от 1 до 5';
        if (!lower.startsWith('http')) return fallback;
      }
      return 'Произошла ошибка. Попробуйте позже';
  }
};
