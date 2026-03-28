/**
 * QuestionHero3D — premium interactive question mark for ZAMK hero section
 * Uses the Blender PNG render with CSS 3D transforms for mouse-follow tilt
 * and idle sway animation. No WebGL needed.
 *
 * ── CONFIG PARAMETERS ────────────────────────────────────────────────────────
 */
const IDLE_AMPLITUDE  = 2.5;   // degrees of idle sway
const IDLE_SPEED      = 0.4;   // cycles per second
const MOUSE_STRENGTH  = 8;     // max tilt degrees from mouse
const LERP_FACTOR     = 0.06;  // smoothing speed
const SHADOW_OPACITY  = 0.25;
/** ─────────────────────────────────────────────────────────────────────────── */

import { useRef, useEffect, useCallback, useState } from 'react';
import pngUrl from '../../assets/question-hero.png';

export function QuestionHero3D() {
  const containerRef = useRef<HTMLDivElement>(null);
  const imgRef = useRef<HTMLImageElement>(null);
  const mouseRef = useRef({ x: 0, y: 0 });
  const currentRot = useRef({ x: 0, y: 0 });
  const rafRef = useRef<number>(0);
  const [loaded, setLoaded] = useState(false);

  // Track mouse position relative to container center, normalized -1..1
  const handleMouseMove = useCallback((e: MouseEvent) => {
    const el = containerRef.current;
    if (!el) return;
    const rect = el.getBoundingClientRect();
    const cx = rect.left + rect.width / 2;
    const cy = rect.top + rect.height / 2;
    mouseRef.current = {
      x: Math.max(-1, Math.min(1, (e.clientX - cx) / (rect.width / 2))),
      y: Math.max(-1, Math.min(1, (e.clientY - cy) / (rect.height / 2))),
    };
  }, []);

  const handleMouseLeave = useCallback(() => {
    mouseRef.current = { x: 0, y: 0 };
  }, []);

  useEffect(() => {
    window.addEventListener('mousemove', handleMouseMove);
    const el = containerRef.current;
    if (el) el.addEventListener('mouseleave', handleMouseLeave);
    return () => {
      window.removeEventListener('mousemove', handleMouseMove);
      if (el) el.removeEventListener('mouseleave', handleMouseLeave);
    };
  }, [handleMouseMove, handleMouseLeave]);

  // Animation loop: idle sway + mouse follow with smooth lerp
  useEffect(() => {
    let startTime = performance.now();

    const animate = () => {
      const t = (performance.now() - startTime) / 1000;
      const img = imgRef.current;
      if (!img) {
        rafRef.current = requestAnimationFrame(animate);
        return;
      }

      // Idle sway
      const idleY = Math.sin(t * IDLE_SPEED * Math.PI * 2) * IDLE_AMPLITUDE;
      const idleX = Math.cos(t * IDLE_SPEED * Math.PI * 2 * 0.7) * IDLE_AMPLITUDE * 0.3;

      // Target = idle + mouse
      const targetX = -mouseRef.current.y * MOUSE_STRENGTH + idleX;
      const targetY = mouseRef.current.x * MOUSE_STRENGTH + idleY;

      // Smooth lerp
      currentRot.current.x += (targetX - currentRot.current.x) * LERP_FACTOR;
      currentRot.current.y += (targetY - currentRot.current.y) * LERP_FACTOR;

      // Apply CSS 3D transform
      img.style.transform = `
        perspective(800px)
        rotateX(${currentRot.current.x}deg)
        rotateY(${currentRot.current.y}deg)
        scale(${loaded ? 1 : 0.95})
      `;

      rafRef.current = requestAnimationFrame(animate);
    };

    rafRef.current = requestAnimationFrame(animate);
    return () => cancelAnimationFrame(rafRef.current);
  }, [loaded]);

  return (
    <div
      ref={containerRef}
      className="w-full h-full"
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        position: 'relative',
      }}
    >
      {/* Контактная тень */}
      <div
        aria-hidden="true"
        style={{
          position: 'absolute',
          bottom: '2%',
          left: '50%',
          transform: 'translateX(-50%)',
          width: '55%',
          height: '30px',
          borderRadius: '50%',
          background: `radial-gradient(ellipse at center, rgba(20,20,35,${SHADOW_OPACITY}) 0%, transparent 70%)`,
          filter: 'blur(18px)',
          pointerEvents: 'none',
          zIndex: 0,
        }}
      />
      {/* Изображение знака вопроса */}
      <img
        ref={imgRef}
        src={pngUrl}
        alt="ZAMK Question Mark"
        onLoad={() => setLoaded(true)}
        style={{
          width: '100%',
          maxWidth: '630px',
          height: 'auto',
          objectFit: 'contain',
          willChange: 'transform',
          transition: loaded ? 'none' : 'opacity 0.6s ease, transform 0.6s ease',
          opacity: loaded ? 1 : 0,
          filter: 'drop-shadow(0 8px 24px rgba(0,0,0,0.15))',
          zIndex: 1,
          pointerEvents: 'none',
          userSelect: 'none',
        }}
        draggable={false}
      />
    </div>
  );
}
