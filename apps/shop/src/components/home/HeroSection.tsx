import { Link } from 'react-router-dom';
import { motion } from 'framer-motion';
import { QuestionHero3D } from '../hero/QuestionHero3D';
import { HeroShadow } from '../hero/HeroShadow';

export const HeroSection = () => {
  return (
    <section className="relative min-h-[90vh] flex items-center py-20 overflow-hidden">
      <div className="container mx-auto px-4 lg:px-8 relative z-10">
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-12 lg:gap-8 items-center">

          {/* Левая часть: Огромный заголовок-цитата в стиле Editorial */}
          <motion.div
            initial={{ opacity: 0, y: 40 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.8, ease: "easeOut" }}
            className="max-w-[640px] z-20"
          >
            <h1 className="text-5xl md:text-6xl lg:text-[5rem] leading-[1.05] tracking-tight font-serif mb-8 text-black dark:text-white">
              «Будущее <br /> моды <br /> создается в <br /> локальных <br /> студиях, а не <br /> в масс-маркете»
            </h1>

            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ delay: 0.6, duration: 0.8 }}
            >
              <Link
                to="/catalog"
                className="inline-flex items-center justify-center px-8 py-4 bg-black dark:bg-white text-white dark:text-black font-medium text-sm rounded-full transition-transform hover:scale-105 active:scale-95"
              >
                Исследовать архив
              </Link>
            </motion.div>
          </motion.div>

          {/* Правая часть: 3D знак вопроса ZAMK */}
          <motion.div
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            transition={{ duration: 1.2, delay: 0.3, ease: "easeOut" }}
            className="flex justify-center lg:justify-end items-center"
          >
            <div className="relative w-full max-w-[480px] lg:max-w-[520px] aspect-square">
              {/* 3D canvas */}
              <div className="absolute inset-0" style={{ zIndex: 2 }}>
                <QuestionHero3D />
              </div>
              {/* Oval shadow */}
              <HeroShadow />
            </div>
          </motion.div>

        </div>
      </div>
    </section>
  );
};

