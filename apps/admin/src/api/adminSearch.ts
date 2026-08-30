import { getAdminGlobalSearch as apiGetAdminGlobalSearch } from '@zamk/api-client/src/admin';
import type { GlobalSearchResult, GlobalSearchResultType, GlobalSearchResponse } from '@zamk/api-client/src/types';

export type { GlobalSearchResult, GlobalSearchResultType, GlobalSearchResponse };

export type SearchGroupKey = 'orders' | 'returns' | 'inventory' | 'products' | 'customers';

export interface SearchGroup {
  key: SearchGroupKey;
  title: string;
  items: GlobalSearchResult[];
}

export const GROUP_TITLES: Record<SearchGroupKey, string> = {
  orders: 'Заказы',
  returns: 'Возвраты',
  inventory: 'Склад',
  products: 'Товары',
  customers: 'Покупатели',
};

export const RESULT_TYPE_TO_GROUP: Record<GlobalSearchResultType, SearchGroupKey> = {
  order: 'orders',
  return: 'returns',
  inventory_unit: 'inventory',
  product_variant: 'products',
  product: 'products',
  customer: 'customers',
};

export const GROUP_ORDER: SearchGroupKey[] = ['orders', 'returns', 'inventory', 'products', 'customers'];

export const groupSearchResults = (results: GlobalSearchResult[]): SearchGroup[] => {
  const groupsMap: Record<SearchGroupKey, GlobalSearchResult[]> = {
    orders: [],
    returns: [],
    inventory: [],
    products: [],
    customers: [],
  };

  for (const item of results) {
    const groupKey = RESULT_TYPE_TO_GROUP[item.type];
    if (groupKey && groupsMap[groupKey]) {
      groupsMap[groupKey].push(item);
    }
  }

  const output: SearchGroup[] = [];
  for (const key of GROUP_ORDER) {
    if (groupsMap[key].length > 0) {
      output.push({
        key,
        title: GROUP_TITLES[key],
        items: groupsMap[key],
      });
    }
  }

  return output;
};

export const getResultNavigationUrl = (result: GlobalSearchResult): string => {
  switch (result.type) {
    case 'order':
      return result.navigationTarget || (result.id ? `/orders/${result.id}` : '/orders');
    case 'product':
      return result.navigationTarget || (result.id ? `/products/${result.id}` : '/products');
    case 'product_variant':
      // Variant requires owning Product ID. Backend supplies navigationTarget = "/products/{productId}".
      // If navigationTarget is unexpectedly missing, fallback to generic "/products" list rather than broken "/products/{variantId}".
      return result.navigationTarget || '/products';
    case 'return':
      return `/returns?id=${encodeURIComponent(result.id)}&orderNumber=${encodeURIComponent(result.canonicalIdentifier)}`;
    case 'inventory_unit':
      return `/inventory?q=${encodeURIComponent(result.canonicalIdentifier)}`;
    case 'customer':
      return `/users?q=${encodeURIComponent(result.canonicalIdentifier)}`;
    default:
      return result.navigationTarget || '/dashboard';
  }
};

export const getAdminSearchErrorMessage = (err: unknown, fallback: string = 'Не удалось выполнить поиск.'): string => {
  if (err && typeof err === 'object') {
    const apiErr = err as { code?: string; message?: string; status?: number };
    if (apiErr.status === 403 || apiErr.code === 'forbidden') {
      return 'Недостаточно прав для выполнения поиска.';
    }
    if (apiErr.code === 'query_too_short') {
      return 'Введите не менее 2 символов для поиска.';
    }
  }
  return fallback;
};

export const searchAdminGlobal = async (query: string): Promise<GlobalSearchResponse> => {
  const trimmed = query.trim();
  if (trimmed.length < 2) {
    return { results: [] };
  }
  return apiGetAdminGlobalSearch(trimmed);
};
