import { useEffect, useState } from 'react';
import { ProductCard } from './ProductCard';
import { fetchSimilarProducts } from '../../api/publicCatalog';
import type { Product } from '../../types/catalog';

interface SimilarProductsBlockProps {
  productId: string;
}

export function SimilarProductsBlock({ productId }: SimilarProductsBlockProps) {
  const [products, setProducts] = useState<Product[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);

    fetchSimilarProducts(productId, 8)
      .then((res) => {
        if (!cancelled) {
          // Source product must never appear inside its own similar-products rail
          const filtered = (res.items || []).filter((p) => p.id !== productId);
          setProducts(filtered);
          setLoading(false);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setProducts([]);
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [productId]);

  // Hide the entire block if empty or still loading without products
  if (loading || products.length === 0) {
    return null;
  }

  return (
    <section className="mt-16 pt-12 border-t border-border-lighter dark:border-white/10" data-testid="similar-products-block">
      <div className="flex items-center justify-between mb-8">
        <div>
          <span className="text-xs uppercase tracking-widest text-ash font-medium">Рекомендации</span>
          <h2 className="text-2xl font-serif text-graphite dark:text-white mt-1">Похожие товары</h2>
        </div>
      </div>
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4 md:gap-6">
        {products.map((p) => (
          <ProductCard key={p.id} product={p} />
        ))}
      </div>
    </section>
  );
}
