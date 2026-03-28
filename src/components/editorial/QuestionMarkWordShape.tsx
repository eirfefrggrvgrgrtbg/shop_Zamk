import React, { useMemo } from 'react';
import { motion } from 'framer-motion';

import questionMarkImg from '../../assets/question-hero.png';

export interface QuestionMarkWordShapeProps {
  word?: string;
  density?: number;
  scale?: number;
  opacity?: number;
  className?: string;
  themeAware?: boolean;
}

export const QuestionMarkWordShape: React.FC<QuestionMarkWordShapeProps> = ({
  word = 'ЗАМК',
  density = 100,
  scale = 1,
  opacity = 1,
  className = '',
  themeAware = true,
}) => {
  const themeClasses = themeAware
    ? 'text-[#333333]/80 dark:text-[#cccccc]/80'
    : 'text-black/80 dark:text-white/80';

  const particles = useMemo(() => {
    const list: any[] = [];
    const count = Math.floor(70 * (density / 100)); // Чуть больше частиц для плотности

    for (let i = 0; i < count; i++) {
      const angle = Math.random() * Math.PI * 2;
      // Делаем разброс частиц шире, чтобы они летали во всей области
      const radius = 35 + Math.random() * 60;
      const baseX = Math.cos(angle) * radius;
      const baseY = Math.sin(angle) * radius;

      const randomDuration = 15 + Math.random() * 25; // Более медленный, парящий эффект
      const randomDelay = Math.random() * -30;

      const sizeRandom = Math.random();
      let sizeClass = 'text-xs md:text-sm';
      if (sizeRandom > 0.85) sizeClass = 'text-lg md:text-2xl font-black blur-[1px]'; // некоторые крупные не в фокусе
      else if (sizeRandom > 0.5) sizeClass = 'text-sm md:text-base font-bold';
      else if (sizeRandom < 0.15) sizeClass = 'text-[10px] blur-[2px] opacity-50'; // мелкие частицы на заднем фоне

      list.push({
        id: `particle-${i}`,
        baseX,
        baseY,
        duration: randomDuration,
        delay: randomDelay,
        sizeClass,
        // Сложная 8-образная или круговая орбита
        animateX: [baseX, baseX + Math.random() * 40 - 20, baseX - Math.random() * 40 + 20, baseX],
        animateY: [baseY, baseY - Math.random() * 40 + 20, baseY + Math.random() * 40 - 20, baseY],
        animateRotate: [0, Math.random() * 180, Math.random() * -180, 0],
        opacityAnim: [0, Math.random() * 0.8 + 0.2, 0] // Плавное появление и исчезновение
      });
    }
    return list;
  }, [density]);

  return (
    <div
      className={`relative w-full h-full min-h-[500px] flex items-center justify-center overflow-hidden ${className}`}
      style={{ transform: `scale(${scale})`, opacity }}
    >
      {/* Слой с парящими словами */}
      <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
        {particles.map((p) => (
          <motion.div
            key={p.id}
            className={`absolute select-none ${themeClasses} ${p.sizeClass} z-0`}
            initial={{ x: `${p.baseX}%`, y: `${p.baseY}%`, opacity: 0 }}
            animate={{
              x: p.animateX.map((val: number) => `${val}vw`),
              y: p.animateY.map((val: number) => `${val}vh`),
              rotate: p.animateRotate,
              opacity: p.opacityAnim,
            }}
            transition={{
              duration: p.duration,
              repeat: Infinity,
              ease: "easeInOut",
              delay: p.delay,
            }}
          >
            {word}
          </motion.div>
        ))}
      </div>

      {/* 
        Центральное 3D изображение.
        Магия CSS-блендинга:
        - В светлой теме background уходит благодаря mix-blend-multiply.
        - В темной теме картинка инвертируется (dark:invert) и background уходит благодаря dark:mix-blend-screen.
      */}
      <motion.div
        className="relative z-10 w-full max-w-lg lg:max-w-xl aspect-square flex items-center justify-center pointer-events-none"
        initial={{ scale: 0.85, opacity: 0 }}
        animate={{ scale: 1, opacity: 1 }}
        transition={{ duration: 1.5, ease: 'easeOut' }}
      >
        <img
          src={questionMarkImg}
          alt="ZAMK Question Mark"
          className="w-full h-full object-contain mix-blend-multiply dark:invert dark:mix-blend-screen"
          style={{ 
            filter: themeAware ? '' : 'grayscale(100%)'
          }}
        />
      </motion.div>
    </div>
  );
};
