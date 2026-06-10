/**
 * API LAYER AUDIT — adminOperations.ts
 *
 * This file is a thin wrapper around @zamk/api-client/src/admin.ts for the pages that
 * don't have their own dedicated api/ file. Below is the layer mapping:
 *
 * Pages that use THIS file (adminOperations.ts):
 *   - AdminSellers.tsx        → getAdminSellers, createAdminSeller, updateAdminSellerStatus
 *   - AdminCategories.tsx     → getAdminCategories, createAdminCategory
 *   - AdminBrands.tsx         → getAdminBrands, createAdminBrand, uploadAdminBrandLogo
 *   - AdminCatalog.tsx        → all of the above
 *   - AdminProducts.tsx       → getAdminProducts, getModerationProducts, approve/reject/publish etc.
 *   - AdminModeration.tsx     → getModerationProducts, approve/reject/publish/hide/block
 *   - AdminDashboard.tsx      → getAdminSellers (via adminOperations), getAdminProducts, getModerationProducts
 *
 * Pages that use DEDICATED api/ files (not this file):
 *   - AdminOrders.tsx         → apps/admin/src/api/adminOrders.ts
 *   - AdminPayments.tsx       → apps/admin/src/api/adminPayments.ts
 *   - AdminPayouts.tsx        → apps/admin/src/api/adminPayouts.ts
 *   - AdminShipments.tsx      → apps/admin/src/api/adminShipments.ts
 *   - AdminReturns.tsx        → apps/admin/src/api/adminReturns.ts
 *   - AdminRefunds.tsx        → apps/admin/src/api/adminRefunds.ts
 *   - AdminReviews.tsx        → apps/admin/src/api/adminReviews.ts
 *   - AdminInventory.tsx      → apps/admin/src/api/adminInventory.ts
 *
 * Known mismatches / TODOs:
 *   TODO: getAdminProducts returns AdminProduct[] but backend may wrap in {items, totalCount}
 *         — not breaking now but should be verified after backend pagination is enforced.
 *   TODO: getAdminCategories/getAdminBrands also may need { items } unwrap in the future.
 */

import {
  getAdminSellers as apiGetSellers,
  createAdminSeller as apiCreateSeller,
  updateAdminSellerStatus as apiUpdateSellerStatus,
  getAdminCategories as apiGetCategories,
  createAdminCategory as apiCreateCategory,
  getAdminBrands as apiGetBrands,
  createAdminBrand as apiCreateBrand,
  uploadAdminBrandLogo as apiUploadBrandLogo,
  getAdminProducts as apiGetProducts,
  getModerationProducts as apiGetModerationProducts,
  adminApproveProduct as apiApproveProduct,
  adminRejectProduct as apiRejectProduct,
  adminPublishProduct as apiPublishProduct,
  adminHideProduct as apiHideProduct,
  adminBlockProduct as apiBlockProduct,
  uploadAdminProductImage as apiUploadProductImage
} from '@zamk/api-client/src/admin';
import type { AdminSeller, Category, Brand, AdminProduct, ModerationProduct } from '@zamk/api-client/src/types';

// Sellers — P0 fix: apiGetSellers now returns { items, totalCount }
export const getAdminSellers = async (): Promise<{ items: AdminSeller[]; totalCount: number }> => {
  return await apiGetSellers();
};

// Backend never returns plaintext password; temporaryPasswordReturned is a boolean flag only.
// Frontend displays the locally-typed password after successful creation.
export const createAdminSeller = async (data: any): Promise<{ seller: AdminSeller; temporaryPasswordReturned: boolean }> => {
  return await apiCreateSeller(data);
};

export const updateAdminSellerStatus = async (id: string, status: string, reason?: string): Promise<void> => {
  await apiUpdateSellerStatus(id, status, reason);
};

// Categories
export const getAdminCategories = async (): Promise<Category[]> => {
  return await apiGetCategories();
};

export const createAdminCategory = async (data: { name: string; slug: string; parentId?: string; description?: string; sortOrder?: number }): Promise<Category> => {
  return await apiCreateCategory(data);
};

// Brands
export const getAdminBrands = async (): Promise<Brand[]> => {
  return await apiGetBrands();
};

export const createAdminBrand = async (data: { name: string; slug: string; description?: string }): Promise<Brand> => {
  return await apiCreateBrand(data);
};

export const uploadAdminBrandLogo = async (brandId: string, file: File): Promise<{ logoUrl: string }> => {
  return await apiUploadBrandLogo(brandId, file);
};

// Products & Moderation
export const getAdminProducts = async (): Promise<AdminProduct[]> => {
  return await apiGetProducts();
};

export const getModerationProducts = async (): Promise<ModerationProduct[]> => {
  return await apiGetModerationProducts();
};

export const approveProduct = async (id: string, comment?: string): Promise<void> => {
  return await apiApproveProduct(id, comment);
};

export const rejectProduct = async (id: string, comment: string): Promise<void> => {
  return await apiRejectProduct(id, comment);
};

export const publishProduct = async (id: string, comment?: string): Promise<void> => {
  return await apiPublishProduct(id, comment);
};

export const hideProduct = async (id: string, comment?: string): Promise<void> => {
  return await apiHideProduct(id, comment);
};

export const blockProduct = async (id: string, comment?: string): Promise<void> => {
  return await apiBlockProduct(id, comment);
};

export const uploadAdminProductImage = async (productId: string, file: File): Promise<{ imageUrl: string }> => {
  return await apiUploadProductImage(productId, file);
};
