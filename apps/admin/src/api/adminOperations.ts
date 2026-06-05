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

// Sellers
export const getAdminSellers = async (): Promise<AdminSeller[]> => {
  return await apiGetSellers();
};

export const createAdminSeller = async (data: any): Promise<{ seller: AdminSeller; temporaryPassword?: string }> => {
  return await apiCreateSeller(data);
};

export const updateAdminSellerStatus = async (id: string, status: string): Promise<AdminSeller> => {
  return await apiUpdateSellerStatus(id, status);
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
