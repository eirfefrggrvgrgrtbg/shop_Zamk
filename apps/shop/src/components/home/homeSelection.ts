import type { Category, Product } from '../../types/catalog';
import type { UIHomeRecommendationBlock } from '../../api/publicCatalog';

/** Home selection rules. Pure and deterministic: no ranking happens on the client, only picking/deduping. */

export function hasRealImage(product: Product): boolean {
  const url = product.image || product.images?.[0]?.url || '';
  return Boolean(url) && !url.startsWith('data:') && !url.includes('placehold.co');
}

/** Products suitable for editorial photography in the hero (real photo only), in the given order. */
export function pickHeroProducts(products: Product[], limit = 3): Product[] {
  return products.filter(hasRealImage).slice(0, limit);
}

/**
 * Categories that actually have products in the loaded storefront selection.
 * Avoids empty/test categories on the home screen without inventing counts.
 */
export function categoriesWithProducts(categories: Category[], products: Product[], limit = 8): Category[] {
  const used = new Set(products.map((p) => p.category));
  return categories.filter((c) => used.has(c.id)).slice(0, limit);
}

export interface PersonalSection {
  /** `for_you` is based on the customer's own signals; `popular` is the general selection. */
  kind: 'for_you' | 'popular';
  items: Product[];
}

export function dedupeProducts(items: Product[], excludeIds: ReadonlySet<string> = new Set()): Product[] {
  const seen = new Set(excludeIds);
  const out: Product[] = [];
  for (const item of items) {
    if (seen.has(item.id)) continue;
    seen.add(item.id);
    out.push(item);
  }
  return out;
}

/**
 * Chooses the single "personal" section of the home screen.
 * - customer with signals (backend returned a `for_you` block) -> `for_you`
 * - otherwise (guest, new customer, no matching products) -> general `popular` selection
 * - nothing available -> null (section is hidden, no artificial filling)
 */
export function choosePersonalSection(
  customerBlocks: UIHomeRecommendationBlock[],
  guestPopular: Product[],
): PersonalSection | null {
  const forYou = customerBlocks.find((b) => b.type === 'for_you' && b.items.length > 0);
  if (forYou) return { kind: 'for_you', items: forYou.items };

  const popular = customerBlocks.find((b) => b.type === 'popular' && b.items.length > 0);
  if (popular) return { kind: 'popular', items: popular.items };

  if (guestPopular.length > 0) return { kind: 'popular', items: guestPopular };
  return null;
}
