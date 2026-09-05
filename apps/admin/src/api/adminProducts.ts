import {
  adminApproveProduct,
  adminBlockProduct,
  adminHideProduct,
  adminPublishProduct,
  adminRejectProduct,
  adminStartProductReview,
  getAdminProduct as apiGetAdminProduct,
  getAdminProducts as apiGetAdminProducts,
  type GetAdminProductsParams,
  getModerationProducts as apiGetModerationProducts,
  uploadAdminProductImage as apiUploadAdminProductImage,
  getAdminProductModerationHistory as apiGetAdminProductModerationHistory,
  createProductPreviewLink,
} from '@zamk/api-client/src/admin';
import { ApiError } from '@zamk/api-client/src/errors';
import type {
  AdminProduct,
  AdminProductImage,
  ModerationProduct,
} from '@zamk/api-client/src/types';

export { createProductPreviewLink };

export type AdminProductStatus =
  | 'draft'
  | 'pending_moderation'
  | 'in_review'
  | 'approved'
  | 'published'
  | 'rejected'
  | 'hidden'
  | 'blocked'
  | 'out_of_stock'
  | 'archived';

export interface AdminProductGalleryImage {
  id?: string;
  url: string;
  altText?: string;
  sortOrder?: number;
  colorId?: string;
}

export interface AdminProductVariantDisplay {
  id: string;
  label: string;
  sku?: string;
  sellerSku?: string;
  size?: string;
  color?: string;
  shadeName?: string;
  barcode?: string;
  price?: number; // in Rubles
  priceCents?: number; // in Cents
  isActive: boolean;
  inStock?: boolean;
  hasInventoryRecord?: boolean;
  totalStock?: number;
  reservedStock?: number;
  availableStock?: number;
}

export interface AdminProductCompositionItem {
  materialId: string;
  materialName?: string;
  percentage: number;
}

export interface AdminProductSizeChartRow {
  sizeValueId: string;
  sizeValueName?: string;
  measurements: Record<string, any>;
}

export interface AdminProductSizeChart {
  id?: string;
  categoryId?: string;
  rows: AdminProductSizeChartRow[];
}

export interface AdminProductView {
  source?: string;
  id: string;
  slug?: string;
  title: string;
  description?: string;
  sellerId?: string;
  sellerSlug?: string;
  sellerName?: string;
  sellerOwnerName?: string;
  sellerOwnerEmail?: string;
  sellerStatus?: string;
  sellerIsActive?: boolean;
  categoryId?: string;
  category?: string;
  brandId?: string;
  brand?: string;
  gender?: string;
  color?: string;
  material?: string;
  careInstructions?: string;
  price: number; // in Rubles
  priceCents?: number; // in Cents
  oldPrice?: number; // in Rubles
  oldPriceCents?: number; // in Cents
  currency: string;
  image?: string;
  mainImageUrl?: string;
  variantsCount?: number;
  activeVariantsCount?: number;
  stock?: number;
  totalStock?: number;
  reservedStock?: number;
  availableStock?: number;
  hasInventoryRecord?: boolean;
  minPriceCents?: number;
  maxPriceCents?: number;
  actualVisibility?: boolean;
  visibilityReasons?: string[];
  storefrontUrl?: string;
  rating?: number;
  gallery: AdminProductGalleryImage[];
  variants: AdminProductVariantDisplay[];
  materialComposition?: AdminProductCompositionItem[];
  sizeChart?: AdminProductSizeChart;
  attributes?: any[];
  status: AdminProductStatus | string;
  statusLabel: string;
  createdAt?: string;
  updatedAt?: string;
  submittedAt?: string;
  moderationComment?: string;
}

const statusLabels: Record<AdminProductStatus, string> = {
  draft: 'Черновик',
  pending_moderation: 'На модерации',
  in_review: 'На проверке',
  approved: 'Одобрен',
  published: 'Опубликован',
  rejected: 'Отклонён',
  hidden: 'Скрыт',
  blocked: 'Заблокирован',
  out_of_stock: 'Нет в наличии',
  archived: 'Архив',
};

const centsToPrice = (value?: number | null): number | undefined => {
  if (value === undefined || value === null) {
    return undefined;
  }
  return value / 100;
};


const mapGallery = (product: AdminProduct | ModerationProduct): AdminProductGalleryImage[] => {
  const images = (product.images ?? []).map((image: AdminProductImage) => ({
    id: image.id,
    url: image.renditionUrl || image.imageUrl,
    altText: image.altText,
    sortOrder: image.sortOrder,
    colorId: (image as any).colorId,
  }));

  if (product.mainImageUrl && !images.some((image) => image.url === product.mainImageUrl)) {
    return [{ url: product.mainImageUrl, altText: product.title, sortOrder: -1 }, ...images];
  }

  return images;
};

const mapVariants = (variants?: any[]): AdminProductVariantDisplay[] => {
  return (variants ?? []).map((variant) => {
    const size = variant.size || variant.sizeValue;
    const color = variant.color || variant.colorName;
    const shade = variant.shadeName;
    const colorDisplay = color ? (shade ? `${color} (${shade})` : color) : undefined;
    const parts = [colorDisplay || color, size].filter(Boolean);
    const sku = variant.sellerSku || variant.sku;

    return {
      id: variant.id,
      label: parts.length > 0 ? parts.join(' / ') : sku || variant.id,
      sku: sku,
      sellerSku: variant.sellerSku,
      size: size,
      color: color,
      shadeName: shade,
      barcode: variant.barcode,
      price: centsToPrice(variant.priceCents),
      priceCents: variant.priceCents,
      isActive: variant.isActive,
      inStock: variant.inStock,
      hasInventoryRecord: variant.hasInventoryRecord,
      totalStock: variant.totalStock,
      reservedStock: variant.reservedStock,
      availableStock: variant.availableStock,
    };
  });
};

export const mapAdminProduct = (product: AdminProduct | ModerationProduct): AdminProductView => {
  const flexibleProduct = product as unknown as Record<string, unknown>;
  const gallery = mapGallery(product);

  let priceCents = typeof flexibleProduct.priceCents === 'number' ? flexibleProduct.priceCents : product.priceCents;
  if ((!priceCents || priceCents <= 0) && product.variants && Array.isArray(product.variants)) {
    const varPrices = product.variants.map((v: any) => v.priceCents || 0).filter((c: number) => c > 0);
    if (varPrices.length > 0) {
      priceCents = Math.min(...varPrices);
    }
  }
  const oldPriceCents = typeof flexibleProduct.oldPriceCents === 'number' ? flexibleProduct.oldPriceCents : product.oldPriceCents;
  const mainImageUrl = typeof flexibleProduct.mainImageUrl === 'string' ? flexibleProduct.mainImageUrl : product.mainImageUrl;

  const totalStock = typeof flexibleProduct.totalStock === 'number' ? flexibleProduct.totalStock : (typeof flexibleProduct.stock === 'number' ? flexibleProduct.stock : undefined);
  const reservedStock = typeof flexibleProduct.reservedStock === 'number' ? flexibleProduct.reservedStock : undefined;
  const availableStock = typeof flexibleProduct.availableStock === 'number' ? flexibleProduct.availableStock : (totalStock !== undefined ? totalStock - (reservedStock || 0) : undefined);

  return {
    id: product.id,
    slug: typeof flexibleProduct.slug === 'string' ? flexibleProduct.slug : undefined,
    title: product.title || String(flexibleProduct.name ?? 'Без названия'),
    description: product.description,
    sellerId: product.sellerId,
    sellerSlug: typeof flexibleProduct.sellerSlug === 'string' ? flexibleProduct.sellerSlug : undefined,
    sellerName: typeof flexibleProduct.sellerName === 'string' ? flexibleProduct.sellerName : undefined,
    sellerOwnerName: typeof flexibleProduct.sellerOwnerName === 'string' ? flexibleProduct.sellerOwnerName : undefined,
    sellerOwnerEmail: typeof flexibleProduct.sellerOwnerEmail === 'string' ? flexibleProduct.sellerOwnerEmail : (typeof flexibleProduct.ownerEmail === 'string' ? flexibleProduct.ownerEmail : undefined),
    sellerStatus: typeof flexibleProduct.sellerStatus === 'string' ? flexibleProduct.sellerStatus : undefined,
    sellerIsActive: typeof flexibleProduct.sellerIsActive === 'boolean' ? flexibleProduct.sellerIsActive : (flexibleProduct.sellerStatus === 'active'),
    categoryId: product.categoryId,
    category: typeof flexibleProduct.categoryName === 'string'
      ? flexibleProduct.categoryName
      : product.categoryId,
    brandId: product.brandId,
    brand: typeof flexibleProduct.brandName === 'string'
      ? flexibleProduct.brandName
      : product.brandId,
    gender: typeof flexibleProduct.gender === 'string' ? flexibleProduct.gender : undefined,
    color: typeof flexibleProduct.color === 'string' ? flexibleProduct.color : undefined,
    material: typeof flexibleProduct.material === 'string' ? flexibleProduct.material : undefined,
    careInstructions: typeof flexibleProduct.careInstructions === 'string' ? flexibleProduct.careInstructions : undefined,
    priceCents: priceCents ?? 0,
    price: centsToPrice(priceCents) ?? 0,
    oldPriceCents: oldPriceCents ?? undefined,
    oldPrice: centsToPrice(oldPriceCents),
    currency: product.currency || 'RUB',
    image: mainImageUrl || gallery[0]?.url,
    mainImageUrl: mainImageUrl,
    variantsCount: typeof flexibleProduct.variantsCount === 'number' ? flexibleProduct.variantsCount : product.variants?.length,
    activeVariantsCount: typeof flexibleProduct.activeVariantsCount === 'number' ? flexibleProduct.activeVariantsCount : product.variants?.filter((v: any) => v.isActive).length,
    stock: availableStock ?? totalStock,
    totalStock,
    reservedStock,
    availableStock,
    hasInventoryRecord: typeof flexibleProduct.hasInventoryRecord === 'boolean' ? flexibleProduct.hasInventoryRecord : (totalStock !== undefined),
    minPriceCents: typeof flexibleProduct.minPriceCents === 'number' ? flexibleProduct.minPriceCents : undefined,
    maxPriceCents: typeof flexibleProduct.maxPriceCents === 'number' ? flexibleProduct.maxPriceCents : undefined,
    actualVisibility: typeof flexibleProduct.actualVisibility === 'boolean' ? flexibleProduct.actualVisibility : undefined,
    visibilityReasons: Array.isArray(flexibleProduct.visibilityReasons) ? flexibleProduct.visibilityReasons as string[] : undefined,
    storefrontUrl: typeof flexibleProduct.storefrontUrl === 'string' ? flexibleProduct.storefrontUrl : undefined,
    rating: typeof flexibleProduct.averageRating === 'number' ? flexibleProduct.averageRating : (typeof flexibleProduct.rating === 'number' ? flexibleProduct.rating : undefined),
    gallery,
    variants: mapVariants(product.variants),
    materialComposition: (flexibleProduct.materialComposition as any) || (product as any).materialComposition,
    sizeChart: (flexibleProduct.sizeChart as any) || (product as any).sizeChart,
    attributes: (flexibleProduct.attributes as any) || (product as any).attributes,
    status: product.status,
    statusLabel: statusLabels[product.status as AdminProductStatus] ?? product.status,
    createdAt: product.createdAt,
    updatedAt: product.updatedAt,
    submittedAt: product.submittedAt,
    moderationComment: product.moderationComment,
  };
};

export const getAdminProducts = async (
  pageOrParams?: number | GetAdminProductsParams,
  limitParam?: number,
  filtersParam?: any
): Promise<{ items: AdminProductView[]; totalCount: number }> => {
  const data = await apiGetAdminProducts(pageOrParams as any, limitParam, filtersParam);
  const items = (data.items || []).map(mapAdminProduct);
  return { items, totalCount: data.totalCount || 0 };
};

export const getAdminProduct = async (id: string): Promise<AdminProductView> => {
  const data = await apiGetAdminProduct(id);
  return mapAdminProduct(data);
};

export const getAdminProductModerationHistory = async (productId: string) => {
  return apiGetAdminProductModerationHistory(productId);
};

export const getModerationProducts = async (params?: {
  q?: string;
  status?: string;
  sellerId?: string;
  categoryId?: string;
  brandId?: string;
  hasProblems?: boolean;
  sortBy?: string;
  sortOrder?: string;
  limit?: number;
  offset?: number;
  signal?: AbortSignal;
}): Promise<{ items: AdminProductView[]; totalCount: number }> => {
  const response = await apiGetModerationProducts(params);
  const items = (response.items || []).map(mapAdminProduct);
  return { items, totalCount: response.totalCount || items.length };
};

export const startProductReview = async (id: string, expectedUpdatedAt?: string): Promise<void> => {
  await adminStartProductReview(id, expectedUpdatedAt);
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
