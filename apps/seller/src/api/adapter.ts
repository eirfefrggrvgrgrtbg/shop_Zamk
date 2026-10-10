import { SellerProduct, SellerProductSize, SellerProductStatus } from '../lib/seller-products';
import { formatVariantLabel } from '../lib/seller-variants';

function mapStatus(apiStatus: string): SellerProductStatus {
  const allowed: SellerProductStatus[] = ['draft', 'pending_moderation', 'in_review', 'approved', 'published', 'rejected', 'hidden', 'blocked', 'out_of_stock', 'archived'];
  if (allowed.includes(apiStatus as SellerProductStatus)) {
    return apiStatus as SellerProductStatus;
  }
  return 'draft';
}

function isUuid(str: string): boolean {
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(str.trim());
}

export function resolveCategoryDisplayName(p: any, categoryMap?: Record<string, string>): string {
  if (typeof p?.categoryName === 'string' && p.categoryName.trim() && !isUuid(p.categoryName)) {
    return p.categoryName.trim();
  }
  if (p?.category && typeof p.category.name === 'string' && p.category.name.trim() && !isUuid(p.category.name)) {
    return p.category.name.trim();
  }
  if (typeof p?.category === 'string' && p.category.trim() && !isUuid(p.category)) {
    return p.category.trim();
  }
  const catId = (typeof p?.categoryId === 'string' ? p.categoryId : (typeof p?.category_id === 'string' ? p.category_id : '')).trim();
  if (catId && categoryMap && typeof categoryMap[catId] === 'string' && categoryMap[catId].trim() && !isUuid(categoryMap[catId])) {
    return categoryMap[catId].trim();
  }
  if (catId && !isUuid(catId)) {
    return catId;
  }
  return 'Категория не определена';
}

export function adaptProductList(apiProducts: any[], categoryMap?: Record<string, string>): SellerProduct[] {
  return (apiProducts || []).map(p => {
    let sizes: SellerProductSize[] = [];
    if (p.variants && Array.isArray(p.variants)) {
      sizes = p.variants.map((v: any) => ({
        size: formatVariantLabel(v),
        stock: v.availableStock ?? v.totalStock ?? ((v.inStock === true) ? 1 : 0)
      }));
    }

    if (sizes.length === 0) {
      const directStock = p.availableStock ?? p.totalStock ?? (p.inStock ? 1 : 0);
      sizes = [{ size: 'Единый вариант', stock: directStock }];
    }

    let price = (p.priceCents || 0) / 100;
    if (price === 0 && p.variants && Array.isArray(p.variants)) {
      const varPrices = p.variants.map((v: any) => v.priceCents || 0).filter((c: number) => c > 0);
      if (varPrices.length > 0) {
        price = Math.min(...varPrices) / 100;
      }
    }
    const oldPrice = p.oldPriceCents ? p.oldPriceCents / 100 : undefined;

    return {
      id: p.id,
      title: p.title,
      sku: p.slug || p.id.substring(0, 8),
      category: resolveCategoryDisplayName(p, categoryMap),
      brand: p.brandName || p.brand || (typeof p.brandId === 'string' && !isUuid(p.brandId) ? p.brandId : 'ZAMK'),
      price: price,
      oldPrice: oldPrice,
      cost: price * 0.5, // Mock value since backend doesn't have cost yet
      status: mapStatus(p.status),
      issue: 'no_issue', // Derived or mock
      views: 0,
      orders: 0,
      rating: p.rating?.average || 0,
      returns: 0,
      revenue: 0,
      adsSpend: 0,
      ctr: 0,
      conversion: 0,
      quality: 100,
      mainPhoto: p.mainImageUrl || '',
      photos: p.images ? p.images.map((img: any) => img.imageUrl) : [],
      description: p.description || '',
      material: p.material || '',
      color: p.color || '',
      season: 'Всесезон',
      sizes: sizes,
      updatedAt: new Date(p.updatedAt || p.createdAt).toLocaleString('ru-RU'),
      rejectionReason: p.moderationComment,
      actualVisibility: p.actualVisibility,
      storefrontUrl: typeof p.storefrontUrl === 'string' && p.storefrontUrl.trim() !== '' ? p.storefrontUrl : undefined,
      visibilityReasons: Array.isArray(p.visibilityReasons) ? p.visibilityReasons : undefined
    };
  });
}
