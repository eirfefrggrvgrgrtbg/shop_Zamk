import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { fetchPublicSeller } from '../api/publicCatalog';
import { ProductCard } from '../components/product/ProductCard';
import { SortDropdown } from '../components/editorial/StudioKit';
import type { Product } from '../types/catalog';

const SORT_OPTIONS = [
  { value: 'newest', label: 'Сначала новые' },
  { value: 'price_asc', label: 'Цена по возрастанию' },
  { value: 'price_desc', label: 'Цена по убыванию' },
];

export function SellerDetail() {
  const { slugOrId } = useParams<{ slugOrId: string }>();
  
  const [seller, setSeller] = useState<any>(null);
  const [products, setProducts] = useState<Product[]>([]);
  const [totalCount, setTotalCount] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [sortBy, setSortBy] = useState('newest');

  useEffect(() => {
    async function load() {
      if (!slugOrId) return;
      setIsLoading(true);
      setError(null);
      try {
        const res = await fetchPublicSeller(slugOrId, { sort: sortBy });
        setSeller(res.seller);
        setProducts(res.products.items);
        setTotalCount(res.products.totalCount);
      } catch (err: any) {
        if (err?.response?.status === 404 || err?.status === 404 || err?.message?.includes('404')) {
          setError('not_found');
        } else {
          setError('unknown_error');
        }
      } finally {
        setIsLoading(false);
      }
    }
    load();
  }, [slugOrId, sortBy]);

  if (isLoading) {
    return (
      <div className="min-h-screen pt-32 pb-24 px-6 md:px-12 flex items-center justify-center bg-[#f1f5f9] dark:bg-[#0a0a0a]">
        <div className="text-[12px] font-mono tracking-widest text-black/40 dark:text-white/40 uppercase">
          Загрузка...
        </div>
      </div>
    );
  }

  if (error === 'not_found' || !seller) {
    return (
      <div className="min-h-screen pt-32 pb-24 px-6 md:px-12 flex flex-col items-center justify-center bg-[#f1f5f9] dark:bg-[#0a0a0a]">
        <h1 className="text-4xl md:text-5xl font-light tracking-tight text-black dark:text-white mb-6">
          Магазин не найден
        </h1>
        <p className="text-[13px] font-sans font-light text-black/60 dark:text-white/60 mb-12">
          Возможно, магазин был удален или заблокирован.
        </p>
        <Link 
          to="/catalog"
          className="h-12 px-8 flex items-center justify-center bg-black dark:bg-white text-white dark:text-black text-[11px] font-mono tracking-widest uppercase hover:bg-black/80 dark:hover:bg-white/80 transition-colors"
        >
          В каталог
        </Link>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-[#f1f5f9] dark:bg-[#0a0a0a] transition-colors duration-500 pt-24 pb-24">
      {/* Seller Header */}
      <div className="border-b border-black/10 dark:border-white/10 bg-white/50 dark:bg-black/50 backdrop-blur-xl sticky top-[60px] z-30">
        <div className="max-w-[1600px] mx-auto px-6 md:px-12 py-8 md:py-12">
          <div className="flex flex-col md:flex-row gap-8 items-start md:items-center">
            {/* Logo */}
            <div className="w-24 h-24 md:w-32 md:h-32 rounded-2xl overflow-hidden bg-black/5 dark:bg-white/5 shrink-0 border border-black/5 dark:border-white/5">
              {seller.logoUrl ? (
                <img src={seller.logoUrl} alt={seller.brandName} className="w-full h-full object-cover" />
              ) : (
                <div className="w-full h-full flex items-center justify-center text-black/20 dark:text-white/20">
                  <span className="text-[10px] font-mono tracking-widest uppercase text-center leading-tight">Нет<br/>лого</span>
                </div>
              )}
            </div>

            {/* Info */}
            <div className="flex-1 min-w-0">
              <div className="inline-flex items-center gap-2 px-2.5 py-1 rounded-full bg-black/5 dark:bg-white/5 border border-black/10 dark:border-white/10 mb-4">
                <span className="w-1.5 h-1.5 rounded-full bg-green-500 animate-pulse" />
                <span className="text-[10px] font-mono tracking-widest text-black/60 dark:text-white/60 uppercase">
                  Магазин продавца
                </span>
              </div>
              <h1 className="text-3xl md:text-5xl font-light tracking-tight text-black dark:text-white mb-4">
                {seller.brandName}
              </h1>
              {seller.description && (
                <p className="text-[14px] md:text-[15px] font-sans font-light text-black/60 dark:text-white/60 max-w-2xl leading-relaxed">
                  {seller.description}
                </p>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Main Content */}
      <div className="max-w-[1600px] mx-auto px-6 md:px-12 mt-12">
        <div className="flex items-center justify-between mb-8">
          <h2 className="text-[13px] font-mono tracking-widest uppercase text-black/40 dark:text-white/40">
            Все товары [{totalCount}]
          </h2>
          <SortDropdown 
            value={sortBy}
            onChange={setSortBy}
            options={SORT_OPTIONS}
          />
        </div>

        {products.length > 0 ? (
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4 md:gap-8">
            {products.map((product) => (
              <ProductCard key={product.id} product={product} />
            ))}
          </div>
        ) : (
          <div className="py-32 flex flex-col items-center justify-center border border-dashed border-black/10 dark:border-white/10 rounded-2xl">
            <p className="text-[14px] font-sans font-light text-black/50 dark:text-white/50 text-center mb-6">
              У этого продавца пока нет активных товаров
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
