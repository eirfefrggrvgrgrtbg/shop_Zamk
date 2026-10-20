import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowRight } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { SectionHeader } from '../components/editorial/StudioKit';
import { fetchCollections, type UICollectionSummary } from '../api/publicCatalog';

export function Collections() {
  const [items, setItems] = useState<UICollectionSummary[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;
    fetchCollections()
      .then((data) => {
        if (!cancelled) setItems(data);
      })
      .catch(() => {
        if (!cancelled) setError('Не удалось загрузить подборки. Попробуйте обновить страницу.');
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className='relative z-10 min-h-screen pt-24 md:pt-28 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1400px]'>
        <SectionHeader
          label='Подборки'
          title='Редакторские подборки'
          description='Вещи независимых брендов, собранные редакцией ZAMK по темам.'
        />

        {isLoading ? (
          <div className='glass-panel-strong p-12 text-center text-ash'>Загрузка подборок...</div>
        ) : error ? (
          <div className='glass-panel-strong p-12 text-center text-error' role='alert'>{error}</div>
        ) : items.length > 0 ? (
          <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6'>
            {items.map((c) => (
              <Link
                key={c.id}
                to={`/collections/${c.slug}`}
                className='group glass-panel-strong overflow-hidden flex flex-col'
              >
                {c.coverImageUrl && (
                  <img src={c.coverImageUrl} alt={c.title} className='h-56 w-full object-cover' loading='lazy' />
                )}
                <div className='p-6 flex-1 flex flex-col'>
                  <h2 className='font-serif text-2xl text-graphite dark:text-white leading-tight'>{c.title}</h2>
                  {c.description && (
                    <p className='mt-2 text-sm text-graphite-light dark:text-white/70 line-clamp-3'>{c.description}</p>
                  )}
                  <span className='mt-auto pt-4 inline-flex items-center gap-2 text-sm text-ash group-hover:text-graphite dark:group-hover:text-white'>
                    Смотреть подборку <ArrowRight className='w-4 h-4' />
                  </span>
                </div>
              </Link>
            ))}
          </div>
        ) : (
          <div className='glass-panel-strong p-12 text-center'>
            <h3 className='text-3xl font-serif text-graphite dark:text-white'>Подборок пока нет</h3>
            <p className='mt-3 text-sm text-graphite-light dark:text-white/70'>Редакция ещё не опубликовала ни одной подборки.</p>
            <Link to='/catalog' className='inline-block mt-6'>
              <Button className='gap-2'>
                Перейти в каталог <ArrowRight className='w-4 h-4' />
              </Button>
            </Link>
          </div>
        )}
      </div>
    </div>
  );
}
