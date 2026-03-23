import { Link } from 'react-router-dom';
import { motion } from 'framer-motion';
import { COLLECTIONS } from '../lib/mock-data';

export function Collections() {
  return (
    <div className="relative z-10 min-h-screen pt-16 md:pt-20 pb-20">
      <div className="container mx-auto px-4 sm:px-6 max-w-[1400px]">
        {/* Editorial Header */}
        <section className="mt-6 mb-12">
          <div className="flex flex-col gap-4 max-w-2xl">
            <h1 className="font-serif text-[clamp(2.5rem,5vw,5rem)] text-graphite leading-[0.95] tracking-tight">
              Кураторские<br />подборки
            </h1>
            <p className="text-graphite-light text-base md:text-lg max-w-lg leading-relaxed mix-blend-multiply dark:mix-blend-screen">
              Смысловая иерархия архива. От новых поступлений до редких находок, собранных редакцией ZAMK.
            </p>
          </div>
        </section>

        {/* Collections Grid */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 lg:gap-8">
          {COLLECTIONS.map((collection, i) => (
            <motion.div
              key={collection.id}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.6, delay: i * 0.1 }}
            >
              <Link
                to={`/catalog?collection=${collection.id}`}
                className="group block rounded-[2rem] border border-border-lighter bg-white/40 dark:bg-white/[0.03] backdrop-blur-xl p-3 shadow-sm transition-all duration-500 hover:-translate-y-1 hover:shadow-[0_16px_40px_rgba(120,150,185,0.12)] dark:hover:shadow-[0_16px_40px_rgba(0,0,0,0.6)]"
              >
                <div className="relative overflow-hidden rounded-[1.5rem] aspect-[4/3] bg-ash-light/20">
                  <img
                    src={collection.image}
                    alt={collection.title}
                    className="w-full h-full object-cover transition-transform duration-700 group-hover:scale-[1.03]"
                    loading="lazy"
                  />
                  <div className="absolute inset-0 bg-gradient-to-t from-graphite/70 via-graphite/10 to-transparent opacity-80" />
                  
                  {/* Text Overlay */}
                  <div className="absolute left-5 right-5 bottom-5 text-white">
                    <div className="flex justify-between items-end mb-1.5">
                      <p className="text-[10px] uppercase tracking-[0.16em] text-white/80 font-semibold">{collection.subtitle}</p>
                      {collection.itemCount > 0 && (
                        <span className="text-[10px] tracking-[0.05em] px-2 py-0.5 rounded-full border border-white/30 backdrop-blur-md bg-white/10">
                          {collection.itemCount}
                        </span>
                      )}
                    </div>
                    <h3 className="text-[26px] font-serif leading-tight">{collection.title}</h3>
                  </div>
                </div>
                
                {/* Outer Description */}
                <div className="px-3 pt-4 pb-2">
                  <p className="text-sm text-graphite-light leading-relaxed">
                    {collection.description}
                  </p>
                  <div className="mt-4 flex items-center text-[12px] font-medium text-primary tracking-[0.05em] uppercase opacity-0 -translate-x-2 transition-all duration-300 group-hover:opacity-100 group-hover:translate-x-0">
                    Смотреть выборку <span className="ml-1">→</span>
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
