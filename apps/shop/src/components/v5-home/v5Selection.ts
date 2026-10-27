import type { Brand, Category, Product } from '../../types/catalog';
import type { UIHomeRecommendationBlock } from '../../api/publicCatalog';
import { dedupeProducts, hasRealImage } from '../home/homeSelection';

/**
 * Pure selection rules for the V5 home preview. No ranking happens here: the backend order is kept,
 * the client only picks, dedupes and labels the source honestly.
 */

export const V5_ARRIVALS_LIMIT = 3;
export const V5_FOR_YOU_LIMIT = 3;
/** Lead brand + up to 6 in the compact list; the rest is reachable through «Все бренды». */
export const V5_BRANDS_LIMIT = 7;
export const V5_INDEX_LIMIT = 5;

/** `for_you` — backend personal composer; `popular` — public 30-day ranking; `selection` — newest items not shown in arrivals. */
export type V5ForYouKind = 'for_you' | 'popular' | 'selection';

export interface V5ForYouSection {
  kind: V5ForYouKind;
  title: string;
  description: string;
  items: Product[];
}

export function pickArrivals(newest: Product[], limit = V5_ARRIVALS_LIMIT): Product[] {
  return newest.filter(hasRealImage).slice(0, limit);
}

export function chooseForYouSection(args: {
  isCustomer: boolean;
  customerBlocks: UIHomeRecommendationBlock[];
  guestPopular: Product[];
  newest: Product[];
  exclude: ReadonlySet<string>;
  limit?: number;
}): V5ForYouSection | null {
  const { isCustomer, customerBlocks, guestPopular, newest, exclude, limit = V5_FOR_YOU_LIMIT } = args;
  const usable = (items: Product[]) => dedupeProducts(items.filter(hasRealImage), exclude).slice(0, limit);

  if (isCustomer) {
    const forYou = usable(customerBlocks.find((b) => b.type === 'for_you')?.items ?? []);
    if (forYou.length > 0) {
      return {
        kind: 'for_you',
        title: 'Для вас',
        description: 'Подборка по вашему избранному, просмотрам и заказам.',
        items: forYou,
      };
    }
  }

  const popularSource = isCustomer ? customerBlocks.find((b) => b.type === 'popular')?.items ?? [] : guestPopular;
  const popular = usable(popularSource);
  if (popular.length > 0) {
    return {
      kind: 'popular',
      title: 'Популярное',
      description: 'Вещи, которые чаще всего покупали за последние 30 дней.',
      items: popular,
    };
  }

  const selection = usable(newest);
  if (selection.length > 0) {
    return {
      kind: 'selection',
      title: 'Выбор ZAMK',
      description: 'Свежие поступления каталога, которые не вошли в новинки выше.',
      items: selection,
    };
  }
  return null;
}

/** Categories for the hero index: only those that actually have loaded products. */
export function pickIndexCategories(categories: Category[], products: Product[], limit = V5_INDEX_LIMIT): Category[] {
  const used = new Set(products.map((p) => p.category));
  return categories.filter((c) => used.has(c.id)).slice(0, limit);
}

export function brandHref(brand: Pick<Brand, 'id' | 'slug'>): string {
  return `/brand/${brand.slug || brand.id}`;
}

export interface V5BrandEntry {
  brand: Brand;
  /** Position in the full brand list, used for the archive number. */
  index: number;
}

export interface V5BrandsSelection {
  lead: V5BrandEntry | null;
  rest: V5BrandEntry[];
  hiddenCount: number;
}

/**
 * The lead is the brand of the newest arrival when it is in the brand list (real data), otherwise the first brand.
 * The home page shows a limited selection, never the whole directory.
 */
export function pickBrandsForHome(brands: Brand[], featuredBrandId?: string, limit = V5_BRANDS_LIMIT): V5BrandsSelection {
  if (brands.length === 0) return { lead: null, rest: [], hiddenCount: 0 };
  const entries = brands.map((brand, index) => ({ brand, index }));
  const featured = featuredBrandId ? entries.find((e) => e.brand.id === featuredBrandId) : undefined;
  const lead = featured ?? entries[0];
  const others = entries.filter((e) => e !== lead);
  const rest = others.slice(0, limit - 1);
  return { lead, rest, hiddenCount: others.length - rest.length };
}

export function archiveNumber(index: number): string {
  return String(index + 1).padStart(3, '0');
}

export function articleCode(product: Pick<Product, 'id'>): string {
  return (String(product.id).split('-').pop() || String(product.id)).slice(0, 12).toUpperCase();
}

/**
 * The photo box is a fixed 3:4, so the layout never shifts. Photos close to 3:4 (2:3 .. 4:5) fill it and lose
 * at most ~13% on one axis (top-anchored, so heads and collars stay); anything wider/taller is shown whole instead of being cropped.
 */
export type V5PhotoFit = 'cover' | 'contain';
export const V5_PHOTO_RATIO_MIN = 0.65;
export const V5_PHOTO_RATIO_MAX = 0.82;

export function photoFit(naturalWidth: number, naturalHeight: number): V5PhotoFit {
  if (!naturalWidth || !naturalHeight) return 'cover';
  const ratio = naturalWidth / naturalHeight;
  return ratio >= V5_PHOTO_RATIO_MIN && ratio <= V5_PHOTO_RATIO_MAX ? 'cover' : 'contain';
}
