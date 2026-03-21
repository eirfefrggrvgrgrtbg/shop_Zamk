import { Link } from 'react-router-dom';
import { ArrowRight } from 'lucide-react';
import { motion } from 'framer-motion';
import { BRANDS } from '../lib/mock-data';

export function Brands() {
  return (
    <div className="min-h-screen relative z-10">
      <div className="container mx-auto px-4 sm:px-6 max-w-7xl pt-32 pb-8">
        <h1 className="text-3xl sm:text-4xl font-serif text-graphite mb-2">Бренды</h1>
        <p className="text-sm text-ash mb-10">{BRANDS.length} партнёров магазина</p>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5">
          {BRANDS.map((brand, i) => (
            <motion.div
              key={brand.id}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: i * 0.05, duration: 0.5, ease: [0.16, 1, 0.3, 1] }}
            >
              <Link
                to={`/brand/${brand.id}`}
                className="group block bg-white rounded-3xl border border-border-lighter overflow-hidden hover:shadow-lg transition-all duration-500"
              >
                <div className="relative h-48 overflow-hidden">
                  <img
                    src={brand.image}
                    alt={brand.name}
                    className="w-full h-full object-cover transition-transform duration-700 group-hover:scale-105"
                  />
                  <div className="absolute inset-0 bg-gradient-to-t from-black/40 to-transparent" />
                  <div className="absolute bottom-4 left-5">
                    <h3 className="text-xl font-serif text-white">{brand.name}</h3>
                    <p className="text-xs text-white/70">{brand.country}</p>
                  </div>
                </div>
                <div className="p-5">
                  <p className="text-sm text-ash leading-relaxed line-clamp-2 mb-3">{brand.description}</p>
                  <span className="inline-flex items-center gap-1 text-xs font-medium text-primary group-hover:gap-2 transition-all">
                    Перейти <ArrowRight className="w-3 h-3" />
                  </span>
                </div>
              </Link>
            </motion.div>
          ))}
        </div>
      </div>
    </div>
  );
}
