import { Link } from 'react-router-dom';
import { motion } from 'framer-motion';
import { COLLECTIONS } from '../lib/mock-data';

export function Collections() {
  return (
    <div className="relative z-10 min-h-screen pt-24 md:pt-28 pb-20">
      <div className="container mx-auto px-4 sm:px-6 max-w-[1400px]">
        {/* Header */}
        <section className="mb-10">
          <p className="text-[11px] font-semibold tracking-[0.14em] text-ash uppercase mb-1">
            Подборки
          </p>
          <h1 className="text-[2rem] md:text-[2.5rem] font-serif text-graphite dark:text-white leading-tight">
            Кураторские наборы
          </h1>
        </section>

        {/* Collections List - вертикальный стек горизонтальных карточек */}
        <div className="space-y-4">
          {COLLECTIONS.map((collection, i) => (
            <motion.div
              key={collection.id}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.6, delay: i * 0.08 }}
            >
              <Link
                to={`/catalog?collection=${collection.id}`}
                className="group flex flex-col md:flex-row gap-4 rounded-xl border border-border-lighter dark:border-white/10 bg-white/60 dark:bg-white/[0.03] backdrop-blur-xl p-3 shadow-sm transition-all duration-500 hover:shadow-[0_12px_36px_rgba(120,150,185,0.12)] dark:hover:shadow-[0_12px_36px_rgba(0,0,0,0.5)] hover:border-primary/30"
              >
                {/* Image - горизонтальное соотношение */}
                <div className="relative overflow-hidden rounded-lg md:w-[280px] aspect-[16/10] md:aspect-[4/3] flex-shrink-0">
                  <img
                    src={collection.image}
                    alt={collection.title}
                    className="w-full h-full object-cover transition-transform duration-700 group-hover:scale-[1.04]"
                    loading="lazy"
                  />
                  {/* Subtle overlay */}
                  <div className="absolute inset-0 bg-gradient-to-r from-transparent via-transparent to-white/30 dark:to-black/30 md:block hidden" />
                </div>

                {/* Content */}
                <div className="flex flex-col justify-center py-2 md:py-4 px-1 md:px-3">
                  <div className="flex items-center gap-3 mb-2">
                    <span className="text-[10px] font-semibold tracking-[0.14em] text-ash uppercase">
                      {collection.subtitle}
                    </span>
                    {collection.itemCount > 0 && (
                      <span className="text-[10px] tracking-[0.05em] px-2 py-0.5 rounded-full bg-ice dark:bg-white/5 text-graphite-light dark:text-white/60">
                        {collection.itemCount} позиций
                      </span>
                    )}
                  </div>

                  <h3 className="text-[22px] md:text-[26px] font-serif text-graphite dark:text-white leading-tight group-hover:text-primary transition-colors">
                    {collection.title}
                  </h3>

                  <p className="mt-2 text-sm text-graphite-light dark:text-white/60 leading-relaxed line-clamp-2 max-w-md">
                    {collection.description}
                  </p>

                  <div className="mt-4 flex items-center text-[12px] font-medium text-primary tracking-[0.04em] uppercase opacity-60 group-hover:opacity-100 transition-opacity">
                    Открыть подборку
                    <span className="ml-1 transition-transform group-hover:translate-x-1">
                      →
                    </span>
                  </div>
                </div>
              </Link>
            </motion.div>
          ))}
        </div>
      </div>
    </div>
  );
}
