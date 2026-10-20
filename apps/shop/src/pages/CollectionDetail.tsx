import { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { ArrowLeft, ArrowRight } from 'lucide-react';
import { ProductCard } from '../components/product/ProductCard';
import { Button } from '../components/ui/Button';
import { fetchCollectionBySlug, type UICollectionDetail } from '../api/publicCatalog';

type LoadState =
  | { status: 'loading' }
  | { status: 'error' }
  | { status: 'not_found' }
  | { status: 'ready'; collection: UICollectionDetail };

export function CollectionDetail() {
  const { slug = '' } = useParams<{ slug: string }>();
  const [loaded, setLoaded] = useState<{ slug: string; state: LoadState } | null>(null);

  useEffect(() => {
    let cancelled = false;
    fetchCollectionBySlug(slug)
      .then((collection) => {
        if (cancelled) return;
        setLoaded({ slug, state: collection ? { status: 'ready', collection } : { status: 'not_found' } });
      })
      .catch(() => {
        if (!cancelled) setLoaded({ slug, state: { status: 'error' } });
      });
    return () => {
      cancelled = true;
    };
  }, [slug]);

  const state: LoadState = loaded && loaded.slug === slug ? loaded.state : { status: 'loading' };

  return (
    <div className='relative z-10 min-h-screen pt-24 md:pt-28 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1400px]'>
        <Link to='/collections' className='inline-flex items-center gap-2 text-sm text-ash hover:text-graphite dark:text-white/55 dark:hover:text-white'>
          <ArrowLeft className='w-4 h-4' /> Все подборки
        </Link>

        {state.status === 'loading' && (
          <div className='mt-6 glass-panel-strong p-12 text-center text-ash'>Загрузка подборки...</div>
        )}

        {state.status === 'error' && (
          <div className='mt-6 glass-panel-strong p-12 text-center text-error' role='alert'>
            Не удалось загрузить подборку. Попробуйте обновить страницу.
          </div>
        )}

        {state.status === 'not_found' && (
          <div className='mt-6 glass-panel-strong p-12 text-center'>
            <h1 className='text-3xl font-serif text-graphite dark:text-white'>Подборка не найдена</h1>
            <p className='mt-3 text-sm text-graphite-light dark:text-white/70'>Возможно, она ещё не опубликована или уже снята с публикации.</p>
            <Link to='/collections' className='inline-block mt-6'>
              <Button className='gap-2'>
                К списку подборок <ArrowRight className='w-4 h-4' />
              </Button>
            </Link>
          </div>
        )}

        {state.status === 'ready' && (
          <>
            <section className='mt-6 mb-10 border-b border-border-lighter dark:border-white/10 pb-8'>
              <p className='text-[11px] font-semibold tracking-[0.14em] text-ash uppercase mb-2'>Подборка</p>
              <h1 className='text-4xl md:text-5xl font-serif text-graphite dark:text-white tracking-tight leading-none'>
                {state.collection.title}
              </h1>
              {state.collection.description && (
                <p className='mt-4 max-w-2xl text-base text-graphite-light dark:text-white/70 leading-relaxed'>
                  {state.collection.description}
                </p>
              )}
              {state.collection.coverImageUrl && (
                <img
                  src={state.collection.coverImageUrl}
                  alt={state.collection.title}
                  className='mt-6 w-full max-h-[420px] object-cover rounded-[1.5rem]'
                />
              )}
            </section>

            {state.collection.products.length > 0 ? (
              <div className='grid grid-cols-2 lg:grid-cols-4 gap-4 md:gap-6'>
                {state.collection.products.map((product) => (
                  <ProductCard key={product.id} product={product} placement='collection' />
                ))}
              </div>
            ) : (
              <div className='glass-panel-strong p-12 text-center'>
                <h2 className='text-3xl font-serif text-graphite dark:text-white'>В подборке пока нет доступных вещей</h2>
                <p className='mt-3 text-sm text-graphite-light dark:text-white/70'>Загляните позже или посмотрите весь каталог.</p>
                <Link to='/catalog' className='inline-block mt-6'>
                  <Button className='gap-2'>
                    Перейти в каталог <ArrowRight className='w-4 h-4' />
                  </Button>
                </Link>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
