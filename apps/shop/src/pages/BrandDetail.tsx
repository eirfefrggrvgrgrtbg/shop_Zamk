import { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { ProductCard } from '../components/product/ProductCard';
import { fetchPublicBrand, type PublicBrandStorefront } from '../api/publicCatalog';
import type { Product } from '../types/catalog';

function formatItemCount(count: number): string {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod100 >= 11 && mod100 <= 14) return `${count} вещей`;
  if (mod10 === 1) return `${count} вещь`;
  if (mod10 >= 2 && mod10 <= 4) return `${count} вещи`;
  return `${count} вещей`;
}

export function BrandDetail() {
  const params = useParams<{ id?: string; slugOrId?: string }>();
  const identifier = params.id || params.slugOrId;

  const [brand, setBrand] = useState<PublicBrandStorefront['brand'] | null>(null);
  const [products, setProducts] = useState<Product[]>([]);
  const [totalCount, setTotalCount] = useState<number>(0);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function loadBrand() {
      if (!identifier) return;
      setIsLoading(true);
      setError(null);

      try {
        const res = await fetchPublicBrand(identifier);
        if (!cancelled) {
          setBrand(res?.brand || null);
          setProducts(res?.items || []);
          setTotalCount(res?.totalCount || 0);
        }
      } catch (err: any) {
        if (!cancelled) {
          if (err?.status === 404 || err?.response?.status === 404 || err?.message?.includes('404')) {
            setError('not_found');
          } else {
            setError('server_error');
          }
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    }

    loadBrand();

    return () => {
      cancelled = true;
    };
  }, [identifier]);

  if (isLoading) {
    return (
      <div className='relative z-10 min-h-screen pt-36 pb-20'>
        <div className='container mx-auto px-4 sm:px-6 max-w-xl text-center text-xs font-mono uppercase tracking-widest text-ash'>
          Загрузка витрины бренда...
        </div>
      </div>
    );
  }

  if (error || !brand) {
    return (
      <div className='relative z-10 min-h-screen pt-36 pb-20'>
        <div className='container mx-auto px-4 sm:px-6 max-w-xl text-center'>
          <h1 className='text-3xl md:text-4xl font-serif text-graphite dark:text-white mb-3'>
            {error === 'server_error' ? 'Ошибка загрузки' : 'Бренд не найден'}
          </h1>
          <p className='text-sm text-graphite-light dark:text-white/60 mb-6'>
            {error === 'server_error'
              ? 'Не удалось загрузить данные бренда. Пожалуйста, попробуйте позже.'
              : 'Возможно, страница бренда была перемещена или ещё не опубликована.'}
          </p>
          <div className='flex justify-center gap-3'>
            <Link to='/brands'>
              <Button>К брендам</Button>
            </Link>
            <Link to='/catalog'>
              <Button variant='outline'>В каталог</Button>
            </Link>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className='relative z-10 min-h-screen pt-24 md:pt-28 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1400px]'>
        {/* Navigation Breadcrumb */}
        <Link
          to='/brands'
          className='inline-flex items-center gap-2 text-xs uppercase tracking-widest text-ash hover:text-graphite dark:text-white/60 dark:hover:text-white transition-colors'
        >
          <ArrowLeft className='w-3.5 h-3.5' /> Все бренды
        </Link>

        {/* Optional Cover Banner */}
        {brand.coverUrl && (
          <div className='mt-6 mb-8 overflow-hidden rounded-[1.5rem] max-h-[380px] border border-border-lighter dark:border-white/10'>
            <img src={brand.coverUrl} alt={`${brand.name} cover`} className='w-full h-full object-cover' />
          </div>
        )}

        {/* Brand Header Identity */}
        <section className='mt-6 pb-10 border-b border-border-lighter dark:border-white/10'>
          <div className='flex flex-col sm:flex-row items-start sm:items-center gap-6 sm:gap-8'>
            {brand.logoUrl && (
              <div className='w-20 h-20 sm:w-24 sm:h-24 md:w-28 md:h-28 rounded-2xl border border-border-lighter dark:border-white/10 overflow-hidden bg-white/70 dark:bg-white/5 p-2 shrink-0 flex items-center justify-center backdrop-blur-sm shadow-xs'>
                <img src={brand.logoUrl} alt={`${brand.name} logo`} className='w-full h-full object-contain' />
              </div>
            )}
            <div className='flex-1 min-w-0'>
              <div className='flex items-center gap-3 mb-2'>
                <span className='text-[10px] font-mono uppercase tracking-[0.2em] text-ash'>
                  Независимый бренд
                </span>
              </div>
              <h1 className='text-3xl sm:text-4xl md:text-5xl lg:text-6xl font-serif text-graphite dark:text-white tracking-tight leading-tight'>
                {brand.name}
              </h1>
              {brand.description && (
                <p className='mt-4 max-w-2xl text-sm md:text-base text-graphite-light dark:text-white/70 font-light leading-relaxed'>
                  {brand.description}
                </p>
              )}
            </div>
          </div>
        </section>

        {/* Product Catalog Section */}
        <div className='mt-10 mb-8 flex items-baseline justify-between gap-4'>
          <div>
            <p className='text-[10px] font-mono uppercase tracking-widest text-ash mb-1'>Архив</p>
            <h2 className='text-2xl md:text-3xl font-serif text-graphite dark:text-white'>
              Коллекция
            </h2>
          </div>
          <span className='text-xs font-mono uppercase tracking-widest text-ash'>
            {totalCount > 0 ? formatItemCount(totalCount) : '0 вещей'}
          </span>
        </div>

        {/* Products Grid */}
        {products.length > 0 ? (
          <div className='grid grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-4 md:gap-6'>
            {products.map((product) => (
              <ProductCard key={product.id} product={product} placement='brand_storefront' />
            ))}
          </div>
        ) : (
          <div className='py-20 px-6 rounded-2xl border border-dashed border-border-lighter dark:border-white/10 bg-white/40 dark:bg-white/5 text-center'>
            <h3 className='text-xl md:text-2xl font-serif text-graphite dark:text-white mb-2'>
              В архиве бренда пока нет доступных вещей
            </h3>
            <p className='text-sm text-graphite-light dark:text-white/60 max-w-md mx-auto mb-6'>
              Новые поступления и дропы бренда появятся здесь после публикации.
            </p>
            <div className='flex flex-wrap justify-center gap-3'>
              <Link to='/catalog'>
                <Button>В каталог</Button>
              </Link>
              <Link to='/brands'>
                <Button variant='outline'>Все бренды</Button>
              </Link>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
