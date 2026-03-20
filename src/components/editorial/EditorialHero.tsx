import React from 'react';
import { motion, useScroll, useTransform } from 'framer-motion';

export function EditorialHero() {
  const { scrollY } = useScroll();
  const yImage = useTransform(scrollY, [0, 1000], [0, 200]);
  const yText = useTransform(scrollY, [0, 1000], [0, -100]);

  return (
    <section className="relative h-[90vh] md:h-[80vh] w-full overflow-hidden bg-milk mx-auto max-w-screen-2xl">
      {/* Background Layer with Parallax */}
      <motion.div 
        className="absolute top-0 right-0 w-full md:w-[70%] h-full z-0"
        style={{ y: yImage }}
      >
        <div className="relative w-full h-full">
          <img 
            src="https://images.unsplash.com/photo-1490481651871-ab68de25d43d?q=80&w=2070&auto=format&fit=crop" 
            alt="Fashion Hero" 
            className="w-full h-full object-cover object-[60%_20%]"
          />
          <div className="absolute inset-0 bg-gradient-to-r from-milk via-milk/40 to-transparent"></div>
        </div>
      </motion.div>

      {/* Foreground Typographic Layer */}
      <div className="container mx-auto px-6 max-w-7xl h-full relative z-10 flex items-center">
        <motion.div 
          className="max-w-xl md:max-w-2xl"
          style={{ y: yText }}
        >
          <motion.div
            initial={{ opacity: 0, y: 30 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 1, ease: [0.16, 1, 0.3, 1], delay: 0.2 }}
          >
            <span className="block text-sm font-bold tracking-[0.2em] text-dusty-blue-dark mb-4 uppercase">
              Curated Collection '26
            </span>
            <h1 className="text-5xl md:text-7xl lg:text-8xl font-serif text-graphite leading-[0.9] mb-8">
              Digital<br/>
              <span className="italic text-ash/80">Gallery</span><br/>
              Of Style.
            </h1>
            <p className="text-lg text-ash max-w-md leading-relaxed mb-10">
              Discover unique pieces from independent designers, carefully selected for the modern aesthetic.
            </p>
            
            <motion.button 
              className="group relative inline-flex items-center gap-4 text-sm font-medium tracking-widest uppercase overflow-hidden"
              whileHover="hover"
            >
              <span className="text-graphite relative z-10">Explore the edit</span>
              <motion.div 
                className="w-12 h-[1px] bg-graphite relative z-10"
                variants={{
                  hover: { width: 80, backgroundColor: '#38BDF8' }
                }}
                transition={{ duration: 0.4, ease: "easeOut" }}
              />
            </motion.button>
          </motion.div>
        </motion.div>
      </div>
      
      {/* Floating abstract decorative element */}
      <motion.div 
        className="absolute bottom-10 right-10 md:bottom-20 md:right-20 hidden md:block w-48 h-64 border border-graphite/10 p-2 z-20 backdrop-blur-sm bg-white/10"
        initial={{ opacity: 0, x: 50 }}
        animate={{ opacity: 1, x: 0 }}
        transition={{ duration: 1.2, delay: 0.8 }}
        style={{ y: useTransform(scrollY, [0, 800], [0, -150]) }}
      >
        <img 
          src="https://images.unsplash.com/photo-1591561954557-26941169b49e?q=80&w=1974&auto=format&fit=crop" 
          alt="Detail" 
          className="w-full h-full object-cover grayscale opacity-80"
        />
      </motion.div>
    </section>
  );
}
