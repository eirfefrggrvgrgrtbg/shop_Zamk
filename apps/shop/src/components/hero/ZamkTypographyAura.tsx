import { useEffect, useRef } from 'react';

/**
 * ZamkTypographyAura
 * A purely CSS/DOM-based typographic aura that arranges the word "ЗАМК" 
 * around the 3D question mark shape to provide a sense of volume and premium graphic design.
 */

interface TypoElement {
  id: number;
  top: string;
  left: string;
  rotate: number;
  scale: number;
  opacity: number;
  blur: number;
  speed: number;
  phase: number;
}

// Meticulously hand-placed coordinates around the question mark contour
const TYPO_MAP: TypoElement[] = [
  // --- TOP ARCH (Left to Right) ---
  { id: 1, top: '22%', left: '32%', rotate: -35, scale: 0.5, opacity: 0.08, blur: 3, speed: 0.5, phase: 0 }, // Far back left
  { id: 2, top: '12%', left: '42%', rotate: -15, scale: 0.7, opacity: 0.15, blur: 1, speed: 0.7, phase: 1.2 }, // Closer top-left
  { id: 3, top: '8%',  left: '55%', rotate: 5,   scale: 0.9, opacity: 0.35, blur: 0, speed: 1.0, phase: 2.5 },  // Front top-right
  { id: 4, top: '15%', left: '72%', rotate: 28,  scale: 0.6, opacity: 0.12, blur: 2, speed: 0.6, phase: 3.1 }, // Back right
  { id: 5, top: '28%', left: '80%', rotate: 45,  scale: 0.45, opacity: 0.05, blur: 4, speed: 0.4, phase: 4.5 }, // Very far right depth

  // --- MIDDLE SPINE (Curves inwards) ---
  { id: 6, top: '42%', left: '72%', rotate: 18,  scale: 0.8, opacity: 0.25, blur: 0, speed: 0.9, phase: 1.5 }, // Front middle right
  { id: 7, top: '55%', left: '58%', rotate: -8,  scale: 0.55, opacity: 0.10, blur: 2, speed: 0.5, phase: 5.2 },  // Back middle

  // --- BOTTOM DOT ---
  { id: 8, top: '75%', left: '38%', rotate: -20, scale: 0.65, opacity: 0.18, blur: 1, speed: 0.8, phase: 2.1 }, // Back left of dot
  { id: 9, top: '85%', left: '58%', rotate: 12,  scale: 0.9, opacity: 0.40, blur: 0, speed: 1.1, phase: 0.8 }, // Front right of dot
  { id: 10, top: '92%', left: '48%', rotate: -5,  scale: 0.4, opacity: 0.06, blur: 3, speed: 0.4, phase: 3.7 }, // Far bottom depth

  // --- ADDITIONAL FILLERS FOR DEEP DEPTH ---
  { id: 11, top: '5%',  left: '48%', rotate: -5,  scale: 0.35, opacity: 0.04, blur: 5, speed: 0.3, phase: 1.1 }, // Extremely deep top
  { id: 12, top: '65%', left: '45%', rotate: 15,  scale: 0.4, opacity: 0.05, blur: 4, speed: 0.5, phase: 4.8 }, // Deep space between spine and dot
];

export function ZamkTypographyAura() {
  const containerRef = useRef<HTMLDivElement>(null);
  const elementsRef = useRef<Array<HTMLDivElement | null>>([]);

  useEffect(() => {
    let rafId = 0;
    const t0 = performance.now();

    const animate = () => {
      const t = (performance.now() - t0) / 1000;

      TYPO_MAP.forEach((item, index) => {
        const el = elementsRef.current[index];
        if (!el) return;

        // Very slow and subtle floating & breathing
        // Each element breathes opacity and drifts by 1-3 pixels
        const floatY = Math.sin(t * item.speed + item.phase) * (3 * item.scale);
        const floatX = Math.cos(t * item.speed * 0.8 + item.phase) * (2 * item.scale);
        
        // Minor rotation sway
        const rotSway = Math.sin(t * item.speed * 0.5 + item.phase) * 2;

        el.style.transform = `translate(-50%, -50%) translate(${floatX}px, ${floatY}px) rotate(${item.rotate + rotSway}deg) scale(${item.scale})`;
        
        // Gentle opacity pulse (mostly stays near base opacity)
        const opacityPulse = item.opacity * (0.8 + 0.2 * Math.sin(t * item.speed * 1.5 + item.phase));
        el.style.opacity = opacityPulse.toString();
      });

      rafId = requestAnimationFrame(animate);
    };

    rafId = requestAnimationFrame(animate);
    return () => cancelAnimationFrame(rafId);
  }, []);

  return (
    <div 
      ref={containerRef}
      className="absolute inset-0 pointer-events-none select-none"
      style={{ zIndex: 0 }}
      aria-hidden="true"
    >
      {TYPO_MAP.map((item, i) => (
        <div
          key={item.id}
          ref={(el) => { elementsRef.current[i] = el; }}
          className="absolute font-serif font-black tracking-widest text-[#1a1c23] dark:text-[#f8fafc]"
          style={{
            top: item.top,
            left: item.left,
            willChange: 'transform, opacity',
            // Base styles, overwritten dynamically by JS but good fallback
            transform: `translate(-50%, -50%) rotate(${item.rotate}deg) scale(${item.scale})`,
            opacity: item.opacity,
            filter: item.blur > 0 ? `blur(${item.blur}px)` : 'none',
            fontSize: '2rem', // Base size, scaled by CSS transform
          }}
        >
          ЗАМК
        </div>
      ))}
    </div>
  );
}
