import { motion, useScroll, useTransform } from 'framer-motion';
import { Link } from 'react-router-dom';
import { ArrowRight } from 'lucide-react';
import { Button } from '../ui/Button';

export function EditorialHero() {
  const { scrollY } = useScroll();
  const yImage = useTransform(scrollY, [0, 800], [0, 120]);
  const yText = useTransform(scrollY, [0, 800], [0, -60]);
  const opacity = useTransform(scrollY, [0, 400], [1, 0.3]);

  return (
    <section className="relative h-[85vh] md:h-[80vh] w-full overflow-hidden bg-milk">
      {/* Background Image with Parallax */}
      <motion.div
        className="absolute top-0 right-0 w-full md:w-[65%] h-full z-0"
        style={{ y: yImage }}
      >
        <div className="relative w-full h-full">
          <img
            src="https://images.unsplash.com/photo-1490481651871-ab68de25d43d?q=80&w=2070&auto=format&fit=crop"
            alt="Fashion collection"
            className="w-full h-full object-cover object-[60%_20%]"
          />
          <div className="absolute inset-0 bg-gradient-to-r from-milk via-milk/60 to-transparent" />
          <div className="absolute inset-0 bg-gradient-to-t from-milk/40 to-transparent" />
        </div>
      </motion.div>

      {/* Foreground Content */}
      <div className="container mx-auto px-4 sm:px-6 max-w-7xl h-full relative z-10 flex items-center">
        <motion.div className="max-w-lg md:max-w-xl" style={{ y: yText, opacity }}>
          <motion.div
            initial={{ opacity: 0, y: 30 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.8, ease: [0.16, 1, 0.3, 1], delay: 0.2 }}
          >
            <span className="inline-block text-xs font-semibold tracking-[0.15em] text-primary-hover mb-4 uppercase bg-primary/10 px-3 py-1 rounded-full">
              Коллекция Весна '26
            </span>
            <h1 className="text-4xl sm:text-5xl md:text-6xl lg:text-7xl font-serif text-graphite leading-[0.95] mb-6">
              Цифровая<br />
              <span className="italic text-ash/70">Галерея</span><br />
              Стиля.
            </h1>
            <p className="text-base sm:text-lg text-ash max-w-md leading-relaxed mb-8">
              Откройте уникальные вещи от независимых дизайнеров. Кураторский подход к современной моде.
            </p>

            <div className="flex flex-wrap gap-3">
              <Link to="/catalog">
                <Button variant="primary" size="lg" className="gap-2">
                  Смотреть каталог
                  <ArrowRight className="w-4 h-4" />
                </Button>
              </Link>
              <Link to="/brands">
                <Button variant="secondary" size="lg">
                  Бренды
                </Button>
              </Link>
            </div>
          </motion.div>
        </motion.div>
      </div>

      {/* Floating decorative card */}
      <motion.div
        className="absolute bottom-12 right-8 md:bottom-16 md:right-16 hidden lg:block w-44 h-56 rounded-2xl overflow-hidden z-20 glass shadow-lg"
        initial={{ opacity: 0, x: 30 }}
        animate={{ opacity: 1, x: 0 }}
        transition={{ duration: 1, delay: 0.8 }}
        style={{ y: useTransform(scrollY, [0, 600], [0, -80]) }}
      >
        <img
          src="https://images.unsplash.com/photo-1591561954557-26941169b49e?q=80&w=600&auto=format&fit=crop"
          alt="Детали"
          className="w-full h-full object-cover opacity-80"
        />
        <div className="absolute bottom-0 left-0 right-0 p-3 bg-gradient-to-t from-black/40 to-transparent">
          <p className="text-white text-xs font-medium">Новая подборка</p>
        </div>
      </motion.div>
    </section>
  );
}
