import { SellerProduct, SellerProductSize, SellerProductStatus } from '../lib/seller-products';

function mapStatus(apiStatus: string): SellerProductStatus {
  const allowed: SellerProductStatus[] = ['draft', 'moderation', 'approved', 'published', 'rejected', 'hidden', 'blocked', 'out_of_stock'];
  if (apiStatus === 'pending_moderation') return 'moderation';
  if (allowed.includes(apiStatus as SellerProductStatus)) {
    return apiStatus as SellerProductStatus;
  }
  return 'draft';
}

export function adaptProductList(apiProducts: any[]): SellerProduct[] {
  return apiProducts.map(p => {
    let sizes: SellerProductSize[] = [];
    if (p.variants && Array.isArray(p.variants)) {
      sizes = p.variants.map((v: any) => ({
        size: v.size || 'Единый',
        stock: (v.inStock === true) ? 10 : 0 // UI expects a number, backend returns boolean `inStock` for variants? Wait, we'll see.
      }));
    }

    if (sizes.length === 0) {
      sizes = [{ size: 'Единый', stock: p.inStock ? 10 : 0 }];
    }

    const price = (p.priceCents || 0) / 100;
    const oldPrice = p.oldPriceCents ? p.oldPriceCents / 100 : undefined;

    return {
      id: p.id,
      title: p.title,
      sku: p.slug || p.id.substring(0, 8),
      category: p.categoryId || 'Одежда',
      brand: p.brandId || 'ZAMK',
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
      _raw: p // keep raw data just in case
    };
  });
}
