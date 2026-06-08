import { useEffect, useState } from 'react';
import { fetchBrands } from '../api/publicCatalog';
import type { Brand } from '../lib/mock-data';

export function Brands() {
  const [brands, setBrands] = useState<Brand[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;

    async function loadBrands() {
      setIsLoading(true);
      setError('');

      try {
        const data = await fetchBrands();
        if (!cancelled) {
          setBrands(data);
        }
      } catch {
        if (!cancelled) {
          setError('Не удалось загрузить бренды. Проверьте, запущен ли backend.');
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    }

    loadBrands();

    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className='relative z-10 min-h-screen pt-32 md:pt-40 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1400px]'>
        <section className="mb-12 border-b border-border-lighter dark:border-white/10 pb-8">
          <p className="text-[13px] font-medium tracking-[0.14em] text-ash uppercase mb-3">
            Сообщество
          </p>
          <div className="flex items-end justify-between">
            <h1 className="text-4xl md:text-5xl font-serif text-graphite dark:text-white tracking-tight leading-none">
              Продавцы
            </h1>
          </div>
        </section>

        {error && <div className="rounded-2xl border border-red-200 bg-red-50 p-4 text-sm text-red-700">{error}</div>}

        {isLoading ? (
          <div className="rounded-3xl border border-border-lighter bg-white/60 p-8 text-center text-ash dark:border-white/10 dark:bg-white/5">
            Загрузка брендов...
          </div>
        ) : brands.length === 0 ? (
          <div className="rounded-3xl border border-dashed border-border-lighter bg-white/60 p-8 text-center text-ash dark:border-white/10 dark:bg-white/5">
            Нет данных
          </div>
        ) : (
          <div className='mt-10 grid grid-cols-1 md:grid-cols-2 gap-5'>
            {brands.map((brand) => (
              <article
                key={brand.id}
                className="rounded-[2rem] border border-border-lighter bg-white/70 p-6 shadow-sm backdrop-blur-md dark:border-white/10 dark:bg-white/5"
              >
                <div className="flex items-center gap-4">
                  <div className="flex h-16 w-16 items-center justify-center overflow-hidden rounded-full bg-ice text-lg font-semibold text-graphite dark:bg-white/10 dark:text-white">
                    {brand.image ? <img src={brand.image} alt={brand.name} className="h-full w-full object-cover" /> : brand.name.slice(0, 1)}
                  </div>
                  <div>
                    <h3 className="text-2xl font-serif font-medium text-graphite dark:text-white">{brand.name}</h3>
                    <p className="mt-1 text-xs uppercase tracking-[0.14em] text-ash">{brand.id}</p>
                  </div>
                </div>
                <p className="mt-4 text-sm text-graphite-light dark:text-white/70">
                  Данные бренда загружены из API.
                </p>
              </article>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
