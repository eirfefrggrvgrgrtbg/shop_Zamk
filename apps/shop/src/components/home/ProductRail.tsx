import { ProductCard } from '../product/ProductCard';
import type { Product } from '../../types/catalog';

interface ProductRailProps {
  products: Product[];
  placement: string;
  /** `grid` wraps on desktop; `rail` always scrolls horizontally. */
  layout?: 'grid' | 'rail';
}

/** Horizontal swipe rail on mobile; 4-column grid on desktop for `grid` layout. */
export function ProductRail({ products, placement, layout = 'rail' }: ProductRailProps) {
  if (layout === 'grid') {
    return (
      <div className="-mx-4 px-4 sm:mx-0 sm:px-0 flex gap-4 overflow-x-auto snap-x snap-mandatory pb-4 hide-scrollbar sm:grid sm:grid-cols-3 lg:grid-cols-4 sm:gap-5 sm:overflow-visible sm:pb-0">
        {products.map((product) => (
          <div key={product.id} className="w-[62vw] max-w-[260px] flex-shrink-0 snap-start sm:w-auto sm:max-w-none">
            <ProductCard product={product} placement={placement} />
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className="-mx-4 px-4 sm:mx-0 sm:px-0 flex gap-4 overflow-x-auto snap-x snap-mandatory pb-4 hide-scrollbar">
      {products.map((product) => (
        <div key={product.id} className="w-[62vw] max-w-[260px] flex-shrink-0 snap-start md:w-[240px]">
          <ProductCard product={product} placement={placement} />
        </div>
      ))}
    </div>
  );
}

export function ProductRailSkeleton({ count = 4 }: { count?: number }) {
  return (
    <div data-testid="product-rail-skeleton" className="grid grid-cols-2 lg:grid-cols-4 gap-4 md:gap-5">
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className="aspect-[3/4.4] animate-pulse rounded-2xl bg-ice/80 dark:bg-white/5" />
      ))}
    </div>
  );
}
