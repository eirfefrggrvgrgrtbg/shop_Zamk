import { motion } from 'framer-motion';
import { ArrowRight, Play } from 'lucide-react';
import { Link } from 'react-router-dom';

export function EditorialHero() {
  return (
    <section className="relative min-h-[90vh] flex items-center pt-24 pb-16 overflow-hidden">
      <div className="container mx-auto px-4 sm:px-6 relative z-10">
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-12 lg:gap-8 items-center">
          
          {/* Text Content - Asymmetric left align */}
          <div className="lg:col-span-7 xl:col-span-6 relative z-20">
            <motion.div
              initial={{ opacity: 0, x: -30 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ duration: 0.8, ease: [0.16, 1, 0.3, 1] }}
            >
              <div className="inline-flex items-center gap-3 px-4 py-2 rounded-full glass-panel mb-8 border border-primary/20 bg-white/30">
                <span className="w-2 h-2 rounded-full bg-primary animate-pulse"></span>
                <span className="text-xs font-semibold tracking-widest uppercase text-graphite">Мягкий андеграунд</span>
              </div>
              
              <h1 className="text-5xl sm:text-6xl md:text-7xl lg:text-8xl font-serif font-bold text-graphite leading-[1.05] tracking-tight mb-8">
                Кураторский<br />
                <span className="text-transparent bg-clip-text bg-gradient-to-r from-graphite to-primary">архив</span>
                <br />стиля.
              </h1>
              
              <p className="text-lg md:text-xl text-graphite-light max-w-lg mb-10 leading-relaxed font-light">
                Цифровая витрина для независимых брендов и альтернативной эстетики. Открываем новые имена вне массового ритейла.
              </p>
              
              <div className="flex flex-col sm:flex-row gap-4">
                <Link 
                  to="/catalog" 
                  className="inline-flex items-center justify-center gap-2 h-14 px-8 rounded-full bg-graphite text-white hover:bg-primary transition-all duration-300 font-medium group shadow-lg shadow-graphite/20 hover:shadow-primary/30"
                >
                  Смотреть каталог
                  <ArrowRight className="w-4 h-4 group-hover:translate-x-1 transition-transform" />
                </Link>
                <Link 
                  to="/brands" 
                  className="inline-flex items-center justify-center gap-2 h-14 px-8 rounded-full glass-panel text-graphite hover:bg-white/60 transition-colors font-medium"
                >
                  <Play className="w-4 h-4 text-primary" />
                  Узнать о нас
                </Link>
              </div>
            </motion.div>
          </div>

          {/* Image & Glass Layers - Right side */}
          <div className="lg:col-span-5 xl:col-span-6 relative">
            <motion.div
              initial={{ opacity: 0, scale: 0.95 }}
              animate={{ opacity: 1, scale: 1 }}
              transition={{ duration: 1, delay: 0.2, ease: [0.16, 1, 0.3, 1] }}
              className="relative w-full aspect-[4/5] sm:aspect-square lg:aspect-[3/4] max-w-lg mx-auto"
            >
              {/* Main Image in Capsule */}
              <div className="absolute inset-0 capsule p-3 rotate-2 hover:rotate-0 transition-transform duration-700">
                <div className="w-full h-full rounded-[2rem] overflow-hidden relative">
                  <img 
                    src="https://images.unsplash.com/photo-1615529182904-14819c35db37?q=80&w=2160&auto=format&fit=crop" 
                    alt="Альтернативная мода" 
                    className="w-full h-full object-cover"
                  />
                  <div className="absolute inset-0 bg-gradient-to-b from-transparent via-transparent to-graphite/40"></div>
                </div>
              </div>

              {/* Floating Glass Tab 1 */}
              <motion.div 
                className="absolute top-12 -left-6 md:-left-12 glass-panel p-4 rounded-3xl max-w-[200px] border border-white/60 shadow-xl"
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.8, delay: 0.5, ease: "easeOut" }}
              >
                <p className="text-[10px] font-bold text-ash tracking-widest uppercase mb-1">Фокус сезона</p>
                <p className="text-sm font-medium text-graphite leading-tight">Технологичный крой и воздушные фактуры</p>
              </motion.div>

              {/* Floating Glass Tab 2 */}
              <motion.div 
                className="absolute bottom-16 -right-4 md:-right-8 glass-panel-strong p-4 rounded-full flex items-center gap-4"
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.8, delay: 0.7, ease: "easeOut" }}
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
    </section>
  );
}
