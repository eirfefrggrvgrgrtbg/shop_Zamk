import { Link } from 'react-router-dom';
import { Button } from '../components/ui/Button';

export function Collections() {
  return (
    <div className="relative z-10 min-h-screen pt-24 md:pt-28 pb-20">
      <div className="container mx-auto px-4 sm:px-6 max-w-[900px]">
        <section className="mb-10">
          <p className="text-[11px] font-semibold tracking-[0.14em] text-ash uppercase mb-1">Подборки</p>
          <h1 className="text-[2rem] md:text-[2.5rem] font-serif text-graphite dark:text-white leading-tight">Кураторские наборы</h1>
        </section>

        <div className="rounded-[2rem] border border-dashed border-border-lighter bg-white/70 p-10 text-center shadow-sm dark:border-white/10 dark:bg-white/5">
          <h2 className="text-2xl font-serif text-graphite dark:text-white">Коллекции пока не подключены</h2>
          <p className="mt-3 text-sm text-ash">Backend endpoint для подборок ещё не добавлен. Здесь не показываются fake коллекции.</p>
          <Link to="/catalog" className="mt-6 inline-block">
            <Button>Перейти в каталог</Button>
          </Link>
        </div>
      </div>
    </div>
  );
}
