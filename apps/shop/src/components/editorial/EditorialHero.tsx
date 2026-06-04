import { motion } from 'framer-motion';
import { ArrowRight, Play } from 'lucide-react';
import { Link } from 'react-router-dom';
import { QuestionHero3D } from '../hero/QuestionHero3D';
import { HeroShadow } from '../hero/HeroShadow';

export function EditorialHero() {
  return (
    <section className="relative min-h-[88vh] flex items-center pt-24 pb-16 overflow-hidden">
      <div className="container mx-auto px-4 sm:px-6 relative z-10">
        <div className="studio-shell p-6 md:p-10 lg:p-12">
          <div className="grid grid-cols-1 lg:grid-cols-12 gap-10 lg:gap-8 items-center">

            {/* ── Left: text content — unchanged ─────────────────────────── */}
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
                    className="inline-flex items-center justify-center gap-2 h-14 px-8 rounded-full bg-graphite text-white dark:text-black hover:bg-primary-hover transition-all duration-300 font-medium group"
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

            {/* ── Right: 3D question mark + floating ZAMK words ──────────── */}
            <div className="lg:col-span-5 xl:col-span-6 relative">
              <motion.div
                initial={{ opacity: 0, scale: 0.95 }}
                animate={{ opacity: 1, scale: 1 }}
                transition={{ duration: 1, delay: 0.2, ease: [0.16, 1, 0.3, 1] }}
                className="relative w-full aspect-square lg:aspect-[4/5] max-w-xl mx-auto"
              >
                {/* 3D model — fills the container */}
                <div className="absolute inset-0" style={{ zIndex: 2 }}>
                  <QuestionHero3D />
                </div>

                {/* Soft oval shadow beneath the model */}
                <HeroShadow />
              </motion.div>
            </div>

          </div>
        </div>
      </div>
    </section>
  );
}
