/**
 * FloatingZamkWords — localized studio dust around the question mark
 * 
 * Particles are concentrated near the sign, fading out at distance.
 * Cursor gently disturbs nearby particles (no explosions, no sparkle).
 * Some particles are static (breathing only), some drift slowly.
 */
const PARTICLE_COUNT = 150;  // Больше мелких частиц для плотного облака
const CURSOR_RADIUS  = 120;  // Зона влияния курсора
const CURSOR_FORCE   = 4;
const DUST_PARALLAX  = 25;   // Параллакс пыли сильнее для объема

import { useEffect, useRef } from 'react';

function sr(seed: number, offset: number): number {
  return ((Math.sin(seed * 9301 + offset * 7927 + 1) + 1) / 2);
}

interface Particle {
  // Base position (% of container)
  baseX: number;
  baseY: number;
  // Current offset from cursor disturbance
  distX: number;
  distY: number;
  // Properties
  size: number;
  maxOpacity: number;
  blur: number;
  // Animation
  driftX: number;
  driftY: number;
  driftSpeed: number;
  breatheSpeed: number;
  breathePhase: number;
  isStatic: boolean; // true = breathing only, no drift 
}

function buildParticles(): Particle[] {
  return Array.from({ length: PARTICLE_COUNT }, (_, i) => {
    // Distribute around center with higher density near center
    const angle = sr(i, 1) * Math.PI * 2;
    const rawRadius = sr(i, 2);
    // Skew distribution to be tightly packed around the sign
    const radius = 2 + Math.pow(rawRadius, 1.5) * 22; // 2% to 24% from center — плотно вокруг знака
    
    // Больше динамичных частиц
    const isStatic = sr(i, 20) > 0.75; // Только 25% статичны
    // Every 8th particle is an "accent"
    const isAccent = i % 8 === 0;

    return {
      baseX: 50 + Math.cos(angle) * radius + (sr(i, 11) - 0.5) * 6,
      baseY: 50 + Math.sin(angle) * radius + (sr(i, 12) - 0.5) * 6,
      distX: 0,
      distY: 0,
      size: isAccent ? (1.5 + sr(i, 3) * 2) : (0.8 + sr(i, 3) * 1.5), // Сильно уменьшили пыль
      maxOpacity: isAccent ? (0.50 + sr(i, 4) * 0.3) : (0.30 + sr(i, 4) * 0.25), // Яркость компенсирует размер
      blur: isAccent ? (1 + sr(i, 5) * 2) : sr(i, 5) * 1.5,
      driftX: isStatic ? 0 : (sr(i, 7) - 0.5) * 25, // Дрейф менее размашистый
      driftY: isStatic ? 0 : (sr(i, 8) - 0.5) * 20,
      driftSpeed: 0.02 + sr(i, 9) * 0.04, // very slow: 0.02 – 0.06 cycles/sec
      breatheSpeed: 0.1 + sr(i, 10) * 0.25,
      breathePhase: sr(i, 13) * Math.PI * 2,
      isStatic,
    };
  });
}

export function FloatingZamkWords() {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const mouseRef = useRef({ x: 0.5, y: 0.5, active: false }); // normalized 0-1 within container
  const smoothMouse = useRef({ x: 0, y: 0 }); // normalized -1..1 for parallax
  const particlesRef = useRef<Particle[]>(buildParticles());
  const rafRef = useRef(0);

  // Mouse tracking
  useEffect(() => {
    const onMove = (e: MouseEvent) => {
      const el = containerRef.current;
      if (!el) return;
      const r = el.getBoundingClientRect();
      mouseRef.current = {
        x: (e.clientX - r.left) / r.width,
        y: (e.clientY - r.top) / r.height,
        active: true,
      };
      // For parallax
      smoothMouse.current.x = (e.clientX - r.left - r.width / 2) / (r.width / 2);
      smoothMouse.current.y = (e.clientY - r.top - r.height / 2) / (r.height / 2);
    };
    const onLeave = () => {
      mouseRef.current.active = false;
      smoothMouse.current = { x: 0, y: 0 };
    };
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseleave', onLeave);
    return () => {
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseleave', onLeave);
    };
  }, []);

  // Canvas animation
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    let w = 0, h = 0;
    const resize = () => {
      const parent = canvas.parentElement;
      if (!parent) return;
      const dpr = Math.min(window.devicePixelRatio || 1, 2);
      w = parent.clientWidth;
      h = parent.clientHeight;
      canvas.width = w * dpr;
      canvas.height = h * dpr;
      canvas.style.width = w + 'px';
      canvas.style.height = h + 'px';
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    };
    resize();
    window.addEventListener('resize', resize);

    const t0 = performance.now();
    const smoothParallax = { x: 0, y: 0 };

    const animate = () => {
      if (!w || !h) { resize(); }
      const t = (performance.now() - t0) / 1000;
      const particles = particlesRef.current;
      const mouse = mouseRef.current;

      // Smooth parallax
      smoothParallax.x += (smoothMouse.current.x * DUST_PARALLAX - smoothParallax.x) * 0.03;
      smoothParallax.y += (smoothMouse.current.y * DUST_PARALLAX - smoothParallax.y) * 0.03;

      ctx.clearRect(0, 0, w, h);

      for (const p of particles) {
        // Base position
        let px = (p.baseX / 100) * w;
        let py = (p.baseY / 100) * h;

        // Drift animation
        if (!p.isStatic) {
          px += Math.sin(t * p.driftSpeed * Math.PI * 2) * p.driftX;
          py += Math.cos(t * p.driftSpeed * Math.PI * 2 * 0.8 + 1.3) * p.driftY;
        }

        // Global parallax (dust reacts strongly to mouse)
        px += smoothParallax.x;
        py += smoothParallax.y;

        // Cursor disturbance — gentle push
        if (mouse.active) {
          const mx = mouse.x * w;
          const my = mouse.y * h;
          const dx = px - mx;
          const dy = py - my;
          const dist = Math.sqrt(dx * dx + dy * dy);
          if (dist < CURSOR_RADIUS && dist > 0) {
            const strength = (1 - dist / CURSOR_RADIUS) * CURSOR_FORCE;
            const targetDx = (dx / dist) * strength;
            const targetDy = (dy / dist) * strength;
            p.distX += (targetDx - p.distX) * 0.08;
            p.distY += (targetDy - p.distY) * 0.08;
          } else {
            // Slowly return
            p.distX *= 0.96;
            p.distY *= 0.96;
          }
        } else {
          p.distX *= 0.96;
          p.distY *= 0.96;
        }

        px += p.distX;
        py += p.distY;

        // Breathing opacity
        const breathe = 0.5 + 0.5 * Math.sin(t * p.breatheSpeed * Math.PI * 2 + p.breathePhase);
        const opacity = p.maxOpacity * (0.6 + breathe * 0.4); // Меньше просадка по прозрачности

        // Distance from center — fade out at edges
        const cx = px / w - 0.5;
        const cy = py / h - 0.5;
        const distFromCenter = Math.sqrt(cx * cx + cy * cy) * 2; // 0..1
        // Плавный фейд на краях, чтобы не обрезалось резко
        const edgeFade = Math.max(0, 1 - distFromCenter * 1.5);

        ctx.save();
        ctx.globalAlpha = opacity * edgeFade;
        if (p.blur > 0.5) {
          ctx.filter = `blur(${p.blur}px)`;
        }
        ctx.fillStyle = '#616c80'; // Ещё темнее и контрастнее (ближе к темно-серому/синему)
        ctx.beginPath();
        ctx.arc(px, py, p.size / 2, 0, Math.PI * 2);
        ctx.fill();
        ctx.restore();
      }

      rafRef.current = requestAnimationFrame(animate);
    };

    rafRef.current = requestAnimationFrame(animate);
    return () => {
      cancelAnimationFrame(rafRef.current);
      window.removeEventListener('resize', resize);
    };
  }, []);

  return (
    <div
      ref={containerRef}
      aria-hidden="true"
      style={{
        position: 'absolute',
        inset: '-15%',  // немного выходит за пределы контейнера
        pointerEvents: 'none',
        zIndex: 3,
      }}
    >
      <canvas
        ref={canvasRef}
        style={{ position: 'absolute', inset: 0, pointerEvents: 'none' }}
      />
    </div>
  );
}
