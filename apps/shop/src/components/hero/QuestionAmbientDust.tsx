import { useEffect, useRef } from 'react';

const PARTICLE_COUNT = 35; // Very few particles, minimal and elegant

interface Particle {
  baseX: number; // percentage width
  baseY: number; // percentage height
  size: number;
  maxOpacity: number;
  driftX: number; // pixels
  driftY: number; // pixels
  driftSpeedX: number;
  driftSpeedY: number;
  phaseX: number;
  phaseY: number;
  breathePhase: number;
  breatheSpeed: number;
  blur: number;
}

export function QuestionAmbientDust() {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const mist1Ref = useRef<HTMLDivElement>(null);
  const mist2Ref = useRef<HTMLDivElement>(null);
  const mist3Ref = useRef<HTMLDivElement>(null);
  const particlesRef = useRef<Particle[]>([]);

  useEffect(() => {
    // Generate particles only once
    particlesRef.current = Array.from({ length: PARTICLE_COUNT }, () => {
      // Distribute only around the question mark contour
      const isTop = Math.random() > 0.35; // 65% in the upper curve
      let cx = 0, cy = 0;
      if (isTop) {
        // Upper curve (right-leaning)
        cx = 0.45 + Math.random() * 0.45; // 45% - 90% Width
        cy = 0.10 + Math.random() * 0.45; // 10% - 55% Height
      } else {
        // Bottom dot area
        cx = 0.40 + Math.random() * 0.25; // 40% - 65% Width
        cy = 0.70 + Math.random() * 0.20; // 70% - 90% Height
      }

      return {
        baseX: cx,
        baseY: cy,
        size: Math.random() > 0.85 ? 2.5 + Math.random() * 1.5 : 1 + Math.random() * 1.2, // mostly 1-2.2px, rare 2.5-4px
        maxOpacity: Math.random() * 0.3 + 0.1, // extremely faint: 0.1 - 0.4
        driftX: 10 + Math.random() * 20, // drift radius 10-30px
        driftY: 10 + Math.random() * 25, 
        driftSpeedX: 0.1 + Math.random() * 0.15, // ultra-slow cycle (cycles per sec)
        driftSpeedY: 0.1 + Math.random() * 0.15,
        phaseX: Math.random() * Math.PI * 2,
        phaseY: Math.random() * Math.PI * 2,
        breathePhase: Math.random() * Math.PI * 2,
        breatheSpeed: 0.2 + Math.random() * 0.3, // slow fade pulse
        blur: Math.random() > 0.8 ? 1.0 + Math.random() * 1.5 : 0, // rare blurred depth
      };
    });
  }, []);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    let w = 0, h = 0;
    const resize = () => {
      const parent = canvas.parentElement;
      if (!parent) return;
      w = parent.clientWidth;
      h = parent.clientHeight;
      const dpr = window.devicePixelRatio || 1;
      canvas.width = w * dpr;
      canvas.height = h * dpr;
      canvas.style.width = `${w}px`;
      canvas.style.height = `${h}px`;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    };
    resize();
    window.addEventListener('resize', resize);

    const t0 = performance.now();
    let raf = 0;

    const animate = () => {
      if (!w || !h) resize();
      const t = (performance.now() - t0) / 1000;
      
      // Animate dust particles
      ctx.clearRect(0, 0, w, h);
      
      particlesRef.current.forEach(p => {
        const dx = Math.sin(t * p.driftSpeedX + p.phaseX) * p.driftX;
        // Float slightly upwards too using simple offset modulo if needed, but pure sin/cos drift is safer and softer
        const dy = Math.cos(t * p.driftSpeedY + p.phaseY) * p.driftY;

        const x = p.baseX * w + dx;
        const y = p.baseY * h + dy;

        // Breathing opacity
        const breathe = Math.sin(t * p.breatheSpeed + p.breathePhase);
        const opacity = p.maxOpacity * (0.6 + 0.4 * breathe);

        ctx.save();
        ctx.globalAlpha = Math.max(0, opacity);
        if (p.blur > 0) ctx.filter = `blur(${p.blur}px)`;
        // Soft white / off-white tone
        ctx.fillStyle = '#f8fafc';
        ctx.beginPath();
        ctx.arc(x, y, p.size / 2, 0, Math.PI * 2);
        ctx.fill();
        ctx.restore();
      });

      // Animate mist layers directly via DOM refs (smoother and independent of CSS keyframes)
      // Mist movement is EXTREMELY slow
      if (mist1Ref.current) {
        const mx = Math.sin(t * 0.05) * 15;
        const my = Math.cos(t * 0.06) * 15;
        mist1Ref.current.style.transform = `translate(${mx}px, ${my}px)`;
      }
      if (mist2Ref.current) {
        const mx = Math.cos(t * 0.04) * -20;
        const my = Math.sin(t * 0.05) * 10;
        mist2Ref.current.style.transform = `translate(${mx}px, ${my}px)`;
      }
      if (mist3Ref.current) {
        const mx = Math.sin(t * 0.03 + 1) * 25;
        const my = Math.cos(t * 0.04 + 2) * -15;
        mist3Ref.current.style.transform = `translate(${mx}px, ${my}px)`;
      }

      raf = requestAnimationFrame(animate);
    };

    raf = requestAnimationFrame(animate);
    return () => {
      cancelAnimationFrame(raf);
      window.removeEventListener('resize', resize);
    };
  }, []);

  return (
    <div 
      className="absolute inset-0 pointer-events-none" 
      style={{ zIndex: 0 }}
      aria-hidden="true"
    >
      {/* 
        Mist Layers 
        Using pure div blobs with brutal filter blur & very low opacity 
        so there are ZERO visible hard edges. 
      */}
      
      {/* Upper mist - covers the top curve */}
      <div 
        ref={mist1Ref}
        className="absolute top-[10%] left-[30%] w-[50%] h-[50%] rounded-[50%]"
        style={{ 
          background: 'radial-gradient(circle at center, rgba(245, 248, 255, 0.05) 0%, transparent 60%)', 
          filter: 'blur(45px)', 
        }} 
      />

      {/* Center ambient mass */}
      <div 
        ref={mist2Ref}
        className="absolute top-[30%] left-[20%] w-[60%] h-[40%] rounded-[50%]"
        style={{ 
          background: 'radial-gradient(ellipse at center, rgba(235, 240, 250, 0.06) 0%, transparent 70%)', 
          filter: 'blur(55px)', 
        }} 
      />

      {/* Bottom dot mist */}
      <div 
        ref={mist3Ref}
        className="absolute bottom-[5%] left-[40%] w-[40%] h-[35%] rounded-[50%]"
        style={{ 
          background: 'radial-gradient(ellipse at center, rgba(220, 230, 245, 0.04) 0%, transparent 65%)', 
          filter: 'blur(40px)', 
        }} 
      />

      {/* Particles Canvas */}
      <canvas 
        ref={canvasRef} 
        className="absolute inset-0 z-10" 
      />
    </div>
  );
}
