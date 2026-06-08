import {
  adminApproveProduct,
  adminBlockProduct,
  adminHideProduct,
  adminPublishProduct,
  adminRejectProduct,
  getAdminProducts as apiGetAdminProducts,
  getModerationProducts as apiGetModerationProducts,
  uploadAdminProductImage as apiUploadAdminProductImage,
} from '@zamk/api-client/src/admin';
import { ApiError } from '@zamk/api-client/src/errors';
import type {
  AdminProduct,
  AdminProductImage,
  AdminProductVariant,
  ModerationProduct,
} from '@zamk/api-client/src/types';

export type AdminProductStatus =
  | 'draft'
  | 'pending_moderation'
  | 'approved'
  | 'published'
  | 'rejected'
  | 'hidden'
  | 'blocked'
  | 'out_of_stock';

export interface AdminProductGalleryImage {
  id?: string;
  url: string;
  altText?: string;
  sortOrder?: number;
}

export interface AdminProductVariantDisplay {
  id: string;
  label: string;
  sku?: string;
  price?: number;
  isActive: boolean;
  inStock?: boolean;
}

export interface AdminProductView {
  id: string;
  title: string;
  description?: string;
  sellerId?: string;
  sellerName?: string;
  category?: string;
  brand?: string;
  price: number;
  oldPrice?: number;
  currency: string;
  image?: string;
  gallery: AdminProductGalleryImage[];
  variants: AdminProductVariantDisplay[];
  status: AdminProductStatus | string;
  statusLabel: string;
  createdAt?: string;
  updatedAt?: string;
  submittedAt?: string;
  moderationComment?: string;
}

type AdminProductListResponse = AdminProduct[] | ModerationProduct[] | {
  items?: AdminProduct[] | ModerationProduct[];
  totalCount?: number;
};

const statusLabels: Record<AdminProductStatus, string> = {
  draft: 'Черновик',
  pending_moderation: 'На модерации',
  approved: 'Одобрен',
  published: 'Опубликован',
  rejected: 'Отклонён',
  hidden: 'Скрыт',
  blocked: 'Заблокирован',
  out_of_stock: 'Нет в наличии',
};

const unwrapProducts = (response: AdminProductListResponse): Array<AdminProduct | ModerationProduct> => {
  if (Array.isArray(response)) {
    return response;
  }
  return response.items ?? [];
};

const centsToPrice = (value?: number): number | undefined => {
  if (typeof value !== 'number') {
    return undefined;
  }
  return value / 100;
};

const mapGallery = (product: AdminProduct | ModerationProduct): AdminProductGalleryImage[] => {
  const images = (product.images ?? []).map((image: AdminProductImage) => ({
    id: image.id,
    url: image.imageUrl,
    altText: image.altText,
    sortOrder: image.sortOrder,
  }));

  if (product.mainImageUrl && !images.some((image) => image.url === product.mainImageUrl)) {
    return [{ url: product.mainImageUrl, altText: product.title, sortOrder: -1 }, ...images];
  }

  return images;
};

const mapVariants = (variants?: AdminProductVariant[]): AdminProductVariantDisplay[] => {
  return (variants ?? []).map((variant) => {
    const parts = [variant.size, variant.color].filter(Boolean);
    return {
      id: variant.id,
      label: parts.length > 0 ? parts.join(' / ') : variant.sku || variant.id,
      sku: variant.sku,
      price: centsToPrice(variant.priceCents),
      isActive: variant.isActive,
      inStock: variant.inStock,
    };
  });
};

export const mapAdminProduct = (product: AdminProduct | ModerationProduct): AdminProductView => {
  const flexibleProduct = product as unknown as Record<string, unknown>;
  const gallery = mapGallery(product);

  return {
    id: product.id,
    title: product.title || String(flexibleProduct.name ?? 'Без названия'),
    description: product.description,
    sellerId: product.sellerId,
    sellerName: typeof flexibleProduct.sellerName === 'string' ? flexibleProduct.sellerName : undefined,
    category: typeof flexibleProduct.categoryName === 'string'
      ? flexibleProduct.categoryName
      : product.categoryId,
    brand: typeof flexibleProduct.brandName === 'string'
      ? flexibleProduct.brandName
      : product.brandId,
    price: centsToPrice(product.priceCents) ?? 0,
    oldPrice: centsToPrice(product.oldPriceCents),
    currency: product.currency || 'RUB',
    image: product.mainImageUrl || gallery[0]?.url,
    gallery,
    variants: mapVariants(product.variants),
    status: product.status,
    statusLabel: statusLabels[product.status as AdminProductStatus] ?? product.status,
    createdAt: product.createdAt,
    updatedAt: product.updatedAt,
    submittedAt: product.submittedAt,
    moderationComment: product.moderationComment,
  };
};

export const getAdminProducts = async (): Promise<AdminProductView[]> => {
  const response = await apiGetAdminProducts() as AdminProductListResponse;
  return unwrapProducts(response).map(mapAdminProduct);
};

export const getModerationProducts = async (): Promise<AdminProductView[]> => {
  const response = await apiGetModerationProducts() as AdminProductListResponse;
  return unwrapProducts(response).map(mapAdminProduct);
};

export const approveProduct = async (id: string, comment?: string): Promise<void> => {
  await adminApproveProduct(id, comment);
};

export const rejectProduct = async (id: string, comment: string): Promise<void> => {
  await adminRejectProduct(id, comment);
};

export const publishProduct = async (id: string, comment?: string): Promise<void> => {
  await adminPublishProduct(id, comment);
};

export const hideProduct = async (id: string, comment?: string): Promise<void> => {
  await adminHideProduct(id, comment);
};

export const blockProduct = async (id: string, comment?: string): Promise<void> => {
  await adminBlockProduct(id, comment);
};

export const uploadAdminProductImage = async (productId: string, file: File): Promise<void> => {
  await apiUploadAdminProductImage(productId, file);
};

export const getAdminProductErrorMessage = (error: unknown, fallback: string): string => {
  if (error instanceof ApiError) {
    if (error.status === 403) {
      return 'Недостаточно прав для этого действия.';
    }
    if (error.status === 400 || error.code === 'validation_error') {
      return 'Проверьте данные и повторите попытку.';
    }
    if (error.status === 409) {
      return 'Товар изменился. Обновите страницу и повторите попытку.';
    }
    if (error.status === 422 || error.code === 'invalid_status') {
      return 'Товар нельзя перевести в этот статус из текущего состояния.';
    }
    if (error.code === 'NETWORK_ERROR') {
      return 'Не удалось подключиться к серверу. Проверьте, запущен ли backend.';
    }
  }

  return fallback;
};
