/**
 * QuestionHero3D — premium interactive question mark with atmospheric layers
 *
 * Layers (back to front):
 * 1. Atmospheric shadow (wide, very faint)
 * 2. Soft haze behind the sign (не glow, а мягкое облако)
 * 3. Question mark PNG with CSS 3D tilt
 * 4. Contact shadow (tight, under the object)
 *
 * All layers respond to mouse with multi-layer parallax + lerp smoothing
 */
const IDLE_AMPLITUDE  = 0.5;   // Значительно уменьшили движения
const IDLE_SPEED      = 0.2;   // Стало более плавным
const MOUSE_TILT      = 2;     // degrees — знак почти не крутится, только чуть-чуть
const MOUSE_SHIFT     = 4;     // px — знак почти не съезжает
const SHADOW_PARALLAX = 2;     // px
const HAZE_PARALLAX   = 4;     // px
const LIGHT_PARALLAX  = -12;   // px — свет двигается в противовес знаку
const LERP            = 0.04;

import { useRef, useEffect, useCallback, useState } from 'react';
import pngUrl from '../../assets/question-hero.png';
import { QuestionAmbientDust } from './QuestionAmbientDust';
import { ZamkTypographyAura } from './ZamkTypographyAura';

export function QuestionHero3D() {
  const containerRef = useRef<HTMLDivElement>(null);
  const imgRef = useRef<HTMLDivElement>(null);
  const hazeRef = useRef<HTMLDivElement>(null);
  const contactRef = useRef<HTMLDivElement>(null);
  const atmoRef = useRef<HTMLDivElement>(null);
  const lightRef = useRef<HTMLDivElement>(null);
  const mouseRef = useRef({ x: 0, y: 0 });
  const smooth = useRef({ x: 0, y: 0, rx: 0, ry: 0 });
  const rafRef = useRef(0);
  const [loaded, setLoaded] = useState(false);

  const onMove = useCallback((e: MouseEvent) => {
    const el = containerRef.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    mouseRef.current = {
      x: Math.max(-1, Math.min(1, (e.clientX - r.left - r.width / 2) / (r.width / 2))),
      y: Math.max(-1, Math.min(1, (e.clientY - r.top - r.height / 2) / (r.height / 2))),
    };
  }, []);

  const onLeave = useCallback(() => { mouseRef.current = { x: 0, y: 0 }; }, []);

  useEffect(() => {
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseleave', onLeave);
    return () => {
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseleave', onLeave);
    };
  }, [onMove, onLeave]);

  useEffect(() => {
    const t0 = performance.now();
    const animate = () => {
      const t = (performance.now() - t0) / 1000;
      const mx = mouseRef.current.x;
      const my = mouseRef.current.y;

      // Smooth lerp
      smooth.current.x += (mx - smooth.current.x) * LERP;
      smooth.current.y += (my - smooth.current.y) * LERP;
      const sx = smooth.current.x;
      const sy = smooth.current.y;

      // Idle sway
      const idleY = Math.sin(t * IDLE_SPEED * Math.PI * 2) * IDLE_AMPLITUDE;
      const idleX = Math.cos(t * IDLE_SPEED * Math.PI * 2 * 0.7) * IDLE_AMPLITUDE * 0.3;

      // === Знак вопроса: tilt + translate ===
      const tiltX = -sy * MOUSE_TILT + idleX;
      const tiltY = sx * MOUSE_TILT + idleY;
      const shiftX = sx * MOUSE_SHIFT;
      const shiftY = sy * MOUSE_SHIFT * 0.6;
      smooth.current.rx += (tiltX - smooth.current.rx) * LERP;
      smooth.current.ry += (tiltY - smooth.current.ry) * LERP;

      if (imgRef.current) {
        imgRef.current.style.transform =
          `perspective(900px) translate(${shiftX}px, ${shiftY}px) rotateX(${smooth.current.rx}deg) rotateY(${smooth.current.ry}deg)`;
      }

      // === Дымка: parallax сильнее ===
      if (hazeRef.current) {
        hazeRef.current.style.transform =
          `translate(calc(-50% + ${sx * HAZE_PARALLAX}px), calc(-50% + ${sy * HAZE_PARALLAX}px))`;
      }

      // === Тени: parallax слабее ===
      if (contactRef.current) {
        contactRef.current.style.transform =
          `translateX(calc(-50% + ${sx * SHADOW_PARALLAX}px))`;
      }
      if (atmoRef.current) {
        atmoRef.current.style.transform =
          `translate(calc(-50% + ${sx * SHADOW_PARALLAX * 0.5}px), ${sy * SHADOW_PARALLAX * 0.3}px)`;
      }
      if (lightRef.current) {
        lightRef.current.style.transform =
          `translate(calc(-50% + ${sx * LIGHT_PARALLAX}px), calc(-50% + ${sy * LIGHT_PARALLAX}px))`;
      }

      rafRef.current = requestAnimationFrame(animate);
    };
    rafRef.current = requestAnimationFrame(animate);
    return () => cancelAnimationFrame(rafRef.current);
  }, []);

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
      {/* 1. Atmospheric shadow — широкая, очень слабая */}
      <div
        ref={atmoRef}
        aria-hidden="true"
        style={{
          position: 'absolute',
          bottom: '-5%',
          left: '50%',
          transform: 'translateX(-50%)',
          width: '90%',
          height: '60px',
          borderRadius: '50%',
          background: 'radial-gradient(ellipse at center, rgba(20,25,40,0.10) 0%, rgba(20,25,40,0.04) 40%, transparent 70%)',
          filter: 'blur(30px)',
          pointerEvents: 'none',
          zIndex: 0,
        }}
      />

      {/* 2. Направленный студийный свет за знаком (Spotlight) */}
      <div
        ref={lightRef}
        aria-hidden="true"
        style={{
          position: 'absolute',
          top: '40%',
          left: '50%',
          transform: 'translate(-50%, -50%)',
          width: '70%',
          height: '70%',
          borderRadius: '50%',
          background: 'radial-gradient(ellipse at center, rgba(230,240,255,0.4) 0%, rgba(200,215,245,0.15) 30%, transparent 65%)',
          filter: 'blur(45px)',
          pointerEvents: 'none',
          zIndex: 0,
        }}
      />

      {/* 2b. Soft haze — более выраженное облако за знаком для объёма */}
      <div
        ref={hazeRef}
        aria-hidden="true"
        style={{
          position: 'absolute',
          top: '48%',
          left: '50%',
          transform: 'translate(-50%, -50%)',
          width: '80%',
          height: '75%',
          borderRadius: '42% 58% 48% 52% / 55% 42% 58% 45%',
          background: 'radial-gradient(ellipse at 50% 42%, rgba(200,208,225,0.45) 0%, rgba(215,220,232,0.20) 30%, rgba(225,228,238,0.08) 55%, transparent 70%)',
          filter: 'blur(35px)',
          pointerEvents: 'none',
          zIndex: 0,
        }}
      />
      {/* 2b. Направленный свет сверху-справа — добавляет объём */}
      <div
        aria-hidden="true"
        style={{
          position: 'absolute',
          top: '15%',
          right: '10%',
          width: '50%',
          height: '40%',
          borderRadius: '50%',
          background: 'radial-gradient(ellipse at center, rgba(240,242,248,0.30) 0%, transparent 60%)',
          filter: 'blur(30px)',
          pointerEvents: 'none',
          zIndex: 0,
        }}
      />

      {/* 3. Contact shadow — плотная, прямо под знаком */}
      <div
        ref={contactRef}
        aria-hidden="true"
        style={{
          position: 'absolute',
          bottom: '3%',
          left: '50%',
          transform: 'translateX(-50%)',
          width: '45%',
          height: '20px',
          borderRadius: '50%',
          background: 'radial-gradient(ellipse at center, rgba(15,15,30,0.30) 0%, rgba(15,15,30,0.12) 50%, transparent 80%)',
          filter: 'blur(12px)',
          pointerEvents: 'none',
          zIndex: 0,
        }}
      />

      {/* 3.5. Ambient Dust System (Particles & ultra-soft mist) */}
      <QuestionAmbientDust />

      {/* 3.6. Typographic Aura ("ЗАМК" instances wrapping the shape) */}
      <ZamkTypographyAura />

      {/* 4. Выпуклый знак вопроса: обёртка с трансформацией */}
      <div
        ref={imgRef}
        style={{
          width: '115%',
          maxWidth: '780px',
          marginLeft: '-7.5%',
          marginTop: '-5%',
          position: 'relative',
          willChange: 'transform',
          zIndex: 1,
          pointerEvents: 'none',
          userSelect: 'none',
          // Внешние объемные тени
          filter: [
            'drop-shadow(-4px -4px 8px rgba(255, 255, 255, 0.5))',
            'drop-shadow(6px 12px 16px rgba(0,0,0,0.25))',
            'drop-shadow(15px 30px 40px rgba(0,0,0,0.15))',
            'drop-shadow(30px 60px 80px rgba(0,0,0,0.10))',
          ].join(' '),
        }}
      >
        {/* Само изображение */}
        <img
          src={pngUrl}
          alt="ZAMK Question Mark"
          onLoad={() => setLoaded(true)}
          style={{
            width: '100%',
            height: 'auto',
            objectFit: 'contain',
            transition: loaded ? 'none' : 'opacity 0.6s ease',
            opacity: loaded ? 1 : 0,
            filter: 'contrast(1.03) brightness(1.02) saturate(1.05)',
            display: 'block',
          }}
          draggable={false}
        />

        {/* Слой с внутренними тенями (Эффект выпуклости) */}
        <div 
          aria-hidden="true"
          style={{
            position: 'absolute',
            inset: 0,
            WebkitMaskImage: `url(${pngUrl})`,
            WebkitMaskSize: '100% 100%',
            WebkitMaskRepeat: 'no-repeat',
            // Внутренняя тень: свет слева-сверху, сильная тень справа-снизу
            boxShadow: 'inset 12px 18px 24px -5px rgba(255,255,255,0.7), inset -15px -25px 35px -5px rgba(0,0,0,0.6)',
            opacity: loaded ? 0.8 : 0,
            transition: 'opacity 0.6s ease',
          }}
        />
      </div>
    </div>
  );
}
