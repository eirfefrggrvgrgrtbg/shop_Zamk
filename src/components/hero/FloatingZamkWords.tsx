/**
 * FloatingZamkWords — atmospheric ЗАМК particles floating around the hero
 * Premium editorial aesthetic: organic, slow, elegant
 *
 * ── CONFIG PARAMETERS ────────────────────────────────────────────────────────
 */
const WORD_COUNT    = 22;
const MIN_OPACITY   = 0.06;
const MAX_OPACITY   = 0.22;
const FONT_FAMILY   = "'Inter', 'Helvetica Neue', sans-serif";
const MIN_DURATION  = 18;
const MAX_DURATION  = 40;
/** ─────────────────────────────────────────────────────────────────────────── */

import { motion, useMotionValue, useSpring } from 'framer-motion';
import { useEffect, useRef } from 'react';

// Разнообразные варианты текста — от крупных акцентов до мелкой пыли
const WORD_VARIANTS = [
  { text: 'ЗАМК',      size: 11, weight: 500, spacing: '0.25em' },
  { text: 'ЗАМК',      size: 16, weight: 700, spacing: '0.15em' },
  { text: 'ЗАМК',      size: 22, weight: 800, spacing: '0.12em' },
  { text: 'З·А·М·К',  size: 10, weight: 400, spacing: '0.3em' },
  { text: 'ЗАМК!',     size: 26, weight: 800, spacing: '0.08em' },
  { text: 'З',         size: 32, weight: 700, spacing: '0' },
  { text: 'ЗА·МК',    size: 13, weight: 600, spacing: '0.2em' },
  { text: 'ЗАМК',      size: 9,  weight: 400, spacing: '0.35em' },
  { text: 'З А М К',  size: 14, weight: 300, spacing: '0.05em' },
  { text: 'ЗАМК',      size: 20, weight: 800, spacing: '0.1em' },
];

interface WordDef {
  id: number;
  x: number; // percent
  y: number; // percent
  variant: typeof WORD_VARIANTS[number];
  opacity: number;
  duration: number;
  delay: number;
  driftX: number;
  driftY: number;
  rotation: number;
  parallaxFactor: number;
}

// Deterministic seeded random
function sr(seed: number, offset: number): number {
  return ((Math.sin(seed * 9301 + offset * 7927 + 1) + 1) / 2);
}

function buildWords(): WordDef[] {
  return Array.from({ length: WORD_COUNT }, (_, i) => {
    // Органичное распределение: вокруг центральной зоны, но не внутри неё
    // Центральная "мёртвая зона" где стоит знак вопроса (~30% от центра)
    const angle = sr(i, 1) * Math.PI * 2;
    const minRadius = 28;
    const maxRadius = 48;
    const radius = minRadius + sr(i, 2) * (maxRadius - minRadius);

    // Добавляем случайный сдвиг для органичности
    const jitterX = (sr(i, 11) - 0.5) * 15;
    const jitterY = (sr(i, 12) - 0.5) * 15;

    return {
      id: i,
      x: 50 + Math.cos(angle) * radius + jitterX,
      y: 50 + Math.sin(angle) * radius + jitterY,
      variant: WORD_VARIANTS[i % WORD_VARIANTS.length],
      opacity: MIN_OPACITY + sr(i, 4) * (MAX_OPACITY - MIN_OPACITY),
      duration: MIN_DURATION + sr(i, 5) * (MAX_DURATION - MIN_DURATION),
      delay: sr(i, 6) * -20,
      driftX: (sr(i, 7) - 0.5) * 50,  // плавные длинные дрейфы
      driftY: (sr(i, 8) - 0.5) * 50,
      rotation: (sr(i, 9) - 0.5) * 20, // слабое вращение
      parallaxFactor: 10 + sr(i, 10) * 30,
    };
  });
}

const WORDS = buildWords();

export function FloatingZamkWords() {
  const containerRef = useRef<HTMLDivElement>(null);
  const mouseX = useMotionValue(0);
  const mouseY = useMotionValue(0);
  const springX = useSpring(mouseX, { stiffness: 30, damping: 20 });
  const springY = useSpring(mouseY, { stiffness: 30, damping: 20 });

  useEffect(() => {
    const onMove = (e: MouseEvent) => {
      const el = containerRef.current;
      if (!el) return;
      const rect = el.getBoundingClientRect();
      const cx = rect.left + rect.width / 2;
      const cy = rect.top + rect.height / 2;
      mouseX.set((e.clientX - cx) / (rect.width / 2));
      mouseY.set((e.clientY - cy) / (rect.height / 2));
    };
    const onLeave = () => {
      mouseX.set(0);
      mouseY.set(0);
    };
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseleave', onLeave);
    return () => {
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseleave', onLeave);
    };
  }, [mouseX, mouseY]);

  return (
    <div
      ref={containerRef}
      aria-hidden="true"
      style={{
        position: 'absolute',
        top: '50%',
        left: '50%',
        transform: 'translate(-50%, -50%)',
        width: '160%',
        height: '160%',
        pointerEvents: 'none',
        zIndex: 2,
        overflow: 'visible',
      }}
    >
      {WORDS.map((w) => (
        <ParticleWord key={w.id} word={w} springX={springX} springY={springY} />
      ))}
    </div>
  );
}

// Отдельный компонент для каждого слова — для оптимизации ре-рендеров
function ParticleWord({
  word: w,
  springX,
  springY,
}: {
  word: WordDef;
  springX: ReturnType<typeof useSpring>;
  springY: ReturnType<typeof useSpring>;
}) {
  const ref = useRef<HTMLSpanElement>(null);

  // Подписка на spring значения для parallax без ре-рендера
  useEffect(() => {
    const unsX = springX.on('change', (latestX) => {
      if (!ref.current) return;
      const latestY = springY.get();
      const tx = latestX * w.parallaxFactor;
      const ty = latestY * w.parallaxFactor;
      ref.current.style.transform = `translate(${tx}px, ${ty}px)`;
    });
    const unsY = springY.on('change', (latestY) => {
      if (!ref.current) return;
      const latestX = springX.get();
      const tx = latestX * w.parallaxFactor;
      const ty = latestY * w.parallaxFactor;
      ref.current.style.transform = `translate(${tx}px, ${ty}px)`;
    });
    return () => { unsX(); unsY(); };
  }, [springX, springY, w.parallaxFactor]);

  return (
    <div
      style={{
        position: 'absolute',
        top: `${w.y}%`,
        left: `${w.x}%`,
      }}
    >
      <span ref={ref} style={{ display: 'inline-block', willChange: 'transform' }}>
        <motion.span
          style={{
            display: 'inline-block',
            fontSize: w.variant.size,
            fontFamily: FONT_FAMILY,
            fontWeight: w.variant.weight,
            letterSpacing: w.variant.spacing,
            color: 'rgba(34, 52, 79, 1)',
            opacity: w.opacity,
            whiteSpace: 'nowrap',
            userSelect: 'none',
          }}
          animate={{
            y: [0, w.driftY, -w.driftY * 0.6, 0],
            x: [0, w.driftX, -w.driftX * 0.7, 0],
            rotate: [0, w.rotation, -w.rotation * 0.5, 0],
          }}
          transition={{
            y: { duration: w.duration, delay: w.delay, repeat: Infinity, ease: 'easeInOut' },
            x: { duration: w.duration * 1.3, delay: w.delay, repeat: Infinity, ease: 'easeInOut' },
            rotate: { duration: w.duration * 1.6, delay: w.delay, repeat: Infinity, ease: 'easeInOut' },
          }}
        >
          {w.variant.text}
        </motion.span>
      </span>
    </div>
  );
}
