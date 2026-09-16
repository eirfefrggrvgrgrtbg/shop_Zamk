import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import { ProductCard } from '../product/ProductCard';
import { SectionHeader } from '../editorial/StudioKit';
import { fetchForYouProducts } from '../../api/publicCatalog';
import type { Product } from '../../types/catalog';
import { useAuth } from '../../contexts/AuthContext';

const reveal = {
  initial: { opacity: 0, y: 24 },
  whileInView: { opacity: 1, y: 0 },
  viewport: { once: true, margin: '-60px' },
  transition: { duration: 0.8, ease: [0.16, 1, 0.3, 1] as const },
};

export function ForYouBlock() {
  const { isAuthenticated, user } = useAuth();
  const [products, setProducts] = useState<Product[]>([]);
  const [loading, setLoading] = useState(true);

  const isCustomer = isAuthenticated && (!user?.role || user?.role === 'customer');

  useEffect(() => {
    if (!isCustomer) {
      setProducts([]);
      setLoading(false);
      return;
    }

    let cancelled = false;
    fetchForYouProducts(12)
      .then((res) => {
        if (!cancelled) {
          setProducts(res.items);
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
  }, [isCustomer]);

  if (!isCustomer || loading || products.length === 0) {
    return null;
  }

  return (
    <motion.section {...reveal} data-testid="for-you-block" className="glass-panel p-7 md:p-10 relative overflow-hidden">
      <div className="absolute inset-0 bg-gradient-to-r from-primary/5 via-transparent to-transparent opacity-30"></div>
      <SectionHeader
        label="Рекомендации"
        title="Для вас"
      />

      <div className="flex gap-4 overflow-x-auto pb-4 hide-scrollbar">
        {products.map((product) => (
          <div key={product.id} className="w-[200px] md:w-[240px] flex-shrink-0">
            <ProductCard product={product} />
          </div>
        ))}
      </div>
    </motion.section>
  );
}
