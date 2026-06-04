import { Link } from 'react-router-dom';
import { ChevronRight, Star } from 'lucide-react';
import { SELLERS } from '../lib/mock-data';

export function Brands() {
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

        <div className='mt-10 grid grid-cols-1 md:grid-cols-2 gap-5'>
          {SELLERS.map((seller) => (
            <Link
              key={seller.id}
              to={`/seller/${seller.slug}`}
              className="group block overflow-hidden rounded-[2rem] border border-border-lighter dark:border-white/10 bg-white/70 dark:bg-white/5 backdrop-blur-md transition-all shadow-sm hover:-translate-y-0.5 hover:shadow-md dark:hover:shadow-none"
            >
              <div className='relative h-28 md:h-32 overflow-hidden'>
                <img src={seller.coverImage} alt={seller.name} className='h-full w-full object-cover transition-transform duration-700 group-hover:scale-105' />
                <div className='absolute inset-0 bg-gradient-to-r from-black/40 via-black/15 to-transparent' />
                <div className='absolute inset-0 bg-white/30 dark:bg-black/30' />
              </div>

              <div className='relative px-5 pb-5 pt-0'>
                <div className='-mt-7 mb-3 h-14 w-14 overflow-hidden rounded-full border-2 border-white dark:border-black shadow-sm'>
                  <img src={seller.avatar} alt={seller.name} className='h-full w-full object-cover' />
                </div>

                <div className='flex items-start justify-between gap-3'>
                  <div>
                    <h3 className='text-2xl font-serif font-medium text-graphite dark:text-gray-100 leading-tight tracking-tight'>{seller.name}</h3>
                    <p className='mt-1 text-xs uppercase tracking-[0.14em] text-ash dark:text-gray-500'>{seller.city}, {seller.country}</p>
                  </div>
                  <ChevronRight className='mt-1 w-5 h-5 text-ash group-hover:text-graphite dark:group-hover:text-white transition-colors' />
                </div>

                <div className='mt-3 flex items-center gap-2 text-sm text-ash dark:text-gray-400/90'>
                  <Star className='w-3.5 h-3.5 text-yellow-500 fill-yellow-500' />
                  <span className='font-semibold text-graphite dark:text-gray-100'>{seller.rating.toFixed(1)}</span>
                  <span className='text-ash/70 dark:text-gray-500'>•</span>
                  <span>{seller.reviewCount} отзывов</span>
                </div>

                <p className='mt-2 text-sm text-graphite-light dark:text-gray-400 line-clamp-2'>{seller.shortDescription}</p>
              </div>
            </Link>
          ))}
        </div>
      </div>
    </div>
  );
}
