import { useEffect, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { motion, AnimatePresence } from 'framer-motion';
import { X, ChevronRight, Package, Layers, Tag, Star, Archive, Grid3X3 } from 'lucide-react';
import { fetchBrands, fetchCategories } from '../../api/publicCatalog';
import type { Brand, Category } from '../../types/catalog';

interface SidebarProps {
  isOpen: boolean;
  onClose: () => void;
}

export function Sidebar({ isOpen, onClose }: SidebarProps) {
  const location = useLocation();
  const [categories, setCategories] = useState<Category[]>([]);
  const [brands, setBrands] = useState<Brand[]>([]);

  useEffect(() => {
    if (isOpen) document.body.style.overflow = 'hidden';
    else document.body.style.overflow = 'unset';
    return () => { document.body.style.overflow = 'unset'; };
  }, [isOpen]);

  useEffect(() => {
    onClose();
  }, [location.pathname]);

  useEffect(() => {
    if (!isOpen || categories.length > 0 || brands.length > 0) return;
    let cancelled = false;

    async function loadNavigationData() {
      try {
        const [apiCategories, apiBrands] = await Promise.all([fetchCategories(), fetchBrands()]);
        if (!cancelled) {
          setCategories(apiCategories);
          setBrands(apiBrands);
        }
      } catch {
        if (!cancelled) {
          setCategories([]);
          setBrands([]);
        }
      }
    }

    loadNavigationData();

    return () => {
      cancelled = true;
    };
  }, [isOpen, categories.length, brands.length]);

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={onClose}
            className="fixed inset-0 z-50 bg-graphite/20 backdrop-blur-sm"
          />

          <motion.div
            initial={{ x: '100%' }}
            animate={{ x: 0 }}
            exit={{ x: '100%' }}
            transition={{ type: 'spring', damping: 25, stiffness: 200 }}
            className="fixed top-0 right-0 bottom-0 z-50 w-[320px] md:w-[360px] bg-white/95 dark:bg-[#111214]/95 backdrop-blur-xl border-l border-border-lighter dark:border-white/10 flex flex-col overflow-hidden"
          >
            <div className="flex items-center justify-between px-6 py-5 border-b border-border-lighter dark:border-white/10">
              <h2 className="text-lg font-serif font-medium text-graphite dark:text-white">Навигация</h2>
              <button onClick={onClose} className="p-2 text-ash hover:text-graphite dark:hover:text-white rounded-full hover:bg-ice dark:hover:bg-white/10 transition-all" aria-label="Закрыть меню">
                <X className="w-5 h-5" />
              </button>
            </div>

            <div className="flex-1 overflow-y-auto px-5 py-6 space-y-8">
              <section>
                <h3 className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.14em] text-ash mb-4">
                  <Grid3X3 className="w-3.5 h-3.5" /> Категории
                </h3>
                <div className="space-y-1">
                  {categories.length > 0 ? categories.map((cat) => (
                    <Link key={cat.id} to={`/catalog?category=${cat.slug}`} onClick={onClose} className="flex items-center justify-between px-4 py-3 rounded-xl text-sm text-graphite dark:text-white/90 hover:bg-ice dark:hover:bg-white/5 transition-colors group">
                      <span className="flex items-center gap-3"><span className="text-base opacity-60">{cat.icon}</span>{cat.name}</span>
                      <ChevronRight className="w-4 h-4 text-ash opacity-0 -translate-x-1 group-hover:opacity-100 group-hover:translate-x-0 transition-all" />
                    </Link>
                  )) : <p className="px-4 py-3 text-sm text-ash">Категории пока не добавлены</p>}
                </div>
              </section>

              <section>
                <h3 className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.14em] text-ash mb-4">
                  <Layers className="w-3.5 h-3.5" /> Подборки
                </h3>
                <div className="rounded-xl border border-dashed border-border-lighter p-4 text-sm text-ash dark:border-white/10">
                  Коллекции пока не подключены
                </div>
              </section>

              <section>
                <h3 className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.14em] text-ash mb-4">
                  <Tag className="w-3.5 h-3.5" /> Бренды
                </h3>
                <div className="flex flex-wrap gap-2">
                  {brands.length > 0 ? brands.map((brand) => (
                    <Link key={brand.id} to={`/brand/${brand.id}`} onClick={onClose} className="px-3 py-1.5 rounded-full text-xs font-medium bg-ice dark:bg-white/5 text-graphite dark:text-white/80 hover:bg-graphite hover:text-white dark:hover:bg-white/20 transition-colors">
                      {brand.name}
                    </Link>
                  )) : <p className="text-sm text-ash">Бренды пока не добавлены</p>}
                </div>
              </section>

              <section>
                <h3 className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.14em] text-ash mb-4">
                  <Star className="w-3.5 h-3.5" /> Быстрый доступ
                </h3>
                <div className="space-y-1">
                  <Link to="/new" onClick={onClose} className="flex items-center gap-3 px-4 py-3 rounded-xl text-sm text-graphite dark:text-white/90 hover:bg-ice dark:hover:bg-white/5 transition-colors">
                    <Package className="w-4 h-4 text-primary" /> Новинки
                  </Link>
                  <Link to="/about" onClick={onClose} className="flex items-center gap-3 px-4 py-3 rounded-xl text-sm text-graphite dark:text-white/90 hover:bg-ice dark:hover:bg-white/5 transition-colors">
                    <Archive className="w-4 h-4 text-ash" /> О витрине
                  </Link>
                </div>
              </section>
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
}
