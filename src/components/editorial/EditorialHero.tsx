import { motion } from 'framer-motion';
import { ArrowRight, Play } from 'lucide-react';
import { Link } from 'react-router-dom';

export function EditorialHero() {
  return (
    <section className="relative min-h-[88vh] flex items-center pt-24 pb-16 overflow-hidden">
      <div className="container mx-auto px-4 sm:px-6 relative z-10">
        <div className="studio-shell p-6 md:p-10 lg:p-12">
          <div className="grid grid-cols-1 lg:grid-cols-12 gap-10 lg:gap-8 items-center">
          
            <div className="lg:col-span-7 xl:col-span-6 relative z-20">
              <motion.div
                initial={{ opacity: 0, x: -30 }}
                animate={{ opacity: 1, x: 0 }}
                transition={{ duration: 0.8, ease: [0.16, 1, 0.3, 1] }}
              >
                <div className="inline-flex items-center gap-3 px-4 py-2 rounded-full border border-border-soft bg-white mb-8">
                  <span className="w-2 h-2 rounded-full bg-primary"></span>
                  <span className="studio-label text-graphite">Кураторская платформа ZAMK</span>
                </div>

                <h1 className="text-5xl sm:text-6xl md:text-7xl lg:text-[5.3rem] font-serif font-medium text-graphite leading-[0.95] tracking-tight mb-7">
                  Архив
                  <br />
                  <span className="text-primary">новой волны</span>
                  <br />
                  стиля
                </h1>

                <p className="text-base md:text-lg text-graphite-light max-w-xl mb-10 leading-relaxed">
                  Цифровая витрина для независимых брендов, одежды, обуви и аксессуаров.
                  Спокойный премиальный подход без массового ритейла.
                </p>

                <div className="flex flex-col sm:flex-row gap-4">
                  <Link
                    to="/catalog"
                    className="inline-flex items-center justify-center gap-2 h-14 px-8 rounded-full bg-graphite text-white hover:bg-primary-hover transition-all duration-300 font-medium group"
                  >
                    Перейти в каталог
                    <ArrowRight className="w-4 h-4 group-hover:translate-x-1 transition-transform" />
                  </Link>
                  <Link
                    to="/new"
                    className="inline-flex items-center justify-center gap-2 h-14 px-8 rounded-full border border-border-soft bg-white text-graphite hover:border-primary/50 transition-colors font-medium"
                  >
                    <Play className="w-4 h-4 text-primary" />
                    Смотреть новинки
                  </Link>
                </div>
              </motion.div>
            </div>

            <div className="lg:col-span-5 xl:col-span-6 relative">
              <motion.div
                initial={{ opacity: 0, scale: 0.95 }}
                animate={{ opacity: 1, scale: 1 }}
                transition={{ duration: 1, delay: 0.2, ease: [0.16, 1, 0.3, 1] }}
                className="relative w-full aspect-[4/5] sm:aspect-square lg:aspect-[4/5] max-w-xl mx-auto"
              >
                <div className="absolute inset-0 rounded-[2rem] overflow-hidden border border-border-lighter bg-white p-2.5 shadow-md">
                  <div className="w-full h-full rounded-[1.5rem] overflow-hidden relative">
                    <img
                      src="https://images.unsplash.com/photo-1483985988355-763728e1935b?q=80&w=1740&auto=format&fit=crop"
                      alt="Модный лукбук"
                      className="w-full h-full object-cover rounded-[1rem]"
                    />
                    <div className="absolute inset-0 bg-gradient-to-t from-[#22344f66] via-transparent to-transparent"></div>
                  </div>
                </div>

                <motion.div
                  className="absolute top-8 -left-3 md:-left-8 rounded-3xl border border-border-lighter bg-white/95 backdrop-blur-sm px-4 py-3 max-w-[220px] shadow-sm"
                  initial={{ opacity: 0, y: 20 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.8, delay: 0.5, ease: 'easeOut' }}
                >
                  <p className="studio-label mb-1">Фокус сезона</p>
                  <p className="text-sm font-medium text-graphite">Точный крой, холодная палитра и функциональные слои</p>
                </motion.div>

                <motion.div
                  className="absolute bottom-10 -right-4 md:-right-8 rounded-full border border-border-lighter bg-white/95 backdrop-blur-sm p-3 pl-4 flex items-center gap-3 shadow-sm"
                  initial={{ opacity: 0, y: 20 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.8, delay: 0.7, ease: 'easeOut' }}
                >
                  <div className="flex -space-x-2">
                    <img src="https://images.unsplash.com/photo-1534528741775-53994a69daeb?w=100&h=100&fit=crop" className="w-8 h-8 rounded-full border-2 border-white" alt="Куратор" />
                    <img src="https://images.unsplash.com/photo-1506794778202-cad84cf45f1d?w=100&h=100&fit=crop" className="w-8 h-8 rounded-full border-2 border-white" alt="Стилист" />
                  </div>
                  <div className="text-xs font-semibold text-graphite pr-2">
                    Новая
                    <br />
                    поставка
                  </div>
                </motion.div>
              </motion.div>
            </div>

          </div>
        </div>
      </div>
    </section>
  );
}
