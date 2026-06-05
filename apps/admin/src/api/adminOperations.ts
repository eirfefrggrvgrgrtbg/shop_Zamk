import {
  getAdminSellers as apiGetSellers,
  createAdminSeller as apiCreateSeller,
  updateAdminSellerStatus as apiUpdateSellerStatus,
  getAdminCategories as apiGetCategories,
  createAdminCategory as apiCreateCategory,
  getAdminBrands as apiGetBrands,
  createAdminBrand as apiCreateBrand,
  uploadAdminBrandLogo as apiUploadBrandLogo
} from '@zamk/api-client/src/admin';
import type { AdminSeller, Category, Brand } from '@zamk/api-client/src/types';

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
