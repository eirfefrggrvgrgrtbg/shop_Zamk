import { Link } from 'react-router-dom';
import { ArrowRight } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { EmptyState } from '../components/ui/EmptyState';

export function Favorites() {
  return (
    <div className='relative z-10 min-h-screen pt-32 md:pt-40 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-5xl'>
        <section className="mb-12 border-b border-border-lighter pb-8">
          <p className="text-[13px] font-medium tracking-[0.14em] text-ash uppercase mb-3">Личный архив</p>
          <h1 className="text-4xl md:text-5xl font-serif text-graphite dark:text-white tracking-tight leading-none">Избранное</h1>
        </section>

        <div className='mt-8 bg-white/60 dark:bg-white/5 border border-border-soft dark:border-white/10 rounded-3xl p-8 backdrop-blur-md shadow-sm'>
          <EmptyState
            icon='heart'
            title='Избранное пока не подключено'
            description='Backend endpoint для избранного ещё не добавлен. Fake товары здесь не показываются.'
            action={
              <Link to='/catalog'>
                <Button className='gap-2'>
                  Перейти в каталог <ArrowRight className='w-4 h-4' />
                </Button>
              </Link>
            }
          />
        </div>
      </div>
    </div>
  );
}
