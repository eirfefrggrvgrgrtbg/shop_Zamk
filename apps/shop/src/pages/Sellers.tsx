import { Link } from 'react-router-dom';
import { Button } from '../components/ui/Button';

export function Sellers() {
  return (
    <div className='relative z-10 min-h-screen pt-32 md:pt-40 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[900px]'>
        <section className="mb-12 border-b border-border-lighter dark:border-white/10 pb-8">
          <p className="text-[13px] font-medium tracking-[0.14em] text-ash uppercase mb-3">Продавцы</p>
          <h1 className="text-4xl md:text-5xl font-serif text-graphite dark:text-white tracking-tight leading-none">Публичные страницы продавцов</h1>
        </section>

        <div className="rounded-[2rem] border border-dashed border-border-lighter bg-white/70 p-10 text-center shadow-sm dark:border-white/10 dark:bg-white/5">
          <h2 className="text-2xl font-serif text-graphite dark:text-white">Публичные страницы продавцов пока не подключены</h2>
          <p className="mt-3 text-sm text-ash">После появления public seller storefront endpoint здесь будут реальные продавцы.</p>
          <Link to="/brands" className="mt-6 inline-block">
            <Button>Смотреть бренды</Button>
          </Link>
        </div>
      </div>
    </div>
  );
}
