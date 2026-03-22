import React, { useState } from 'react';
import { useTheme } from '../../contexts/ThemeContext';
import { flushSync } from 'react-dom';

export function ThemeFlipCorner() {
  const { isDark, setTheme } = useTheme();
  const [isHovered, setIsHovered] = useState(false);

  const toggleTheme = () => {
    const nextTheme = isDark ? 'light' : 'dark';

    if (!document.startViewTransition || window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      setTheme(nextTheme);
      return;
    }

    document.startViewTransition(() => {
      flushSync(() => {
        setTheme(nextTheme);
      });
    });
  };

  const size = isHovered ? 82 : 56; 

  return (
    <button 
      className="fixed top-0 left-0 z-[100] cursor-pointer outline-none focus:outline-none bg-transparent border-0 p-0 transition-all duration-700 ease-[cubic-bezier(0.16,1,0.3,1)]"
      style={{ width: size, height: size }}
      onClick={toggleTheme}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
      aria-label="Переключить тему"
      title="Сменить оформление"
    >
      <div 
        className="absolute inset-0 transition-colors duration-700 pointer-events-none"
        style={{
          clipPath: 'polygon(0 0, 100% 0, 0 100%)',
          background: isDark ? '#EEF4FA' : '#050505',
          boxShadow: isDark 
            ? 'inset 8px 8px 24px rgba(124,156,191,0.5), inset 2px 2px 6px rgba(124,156,191,0.7)' 
            : 'inset 8px 8px 24px rgba(0,0,0,0.8), inset 2px 2px 6px rgba(0,0,0,1)'
        }} 
      />
      
      <div 
         className="absolute inset-0 transition-all duration-700 pointer-events-none"
         style={{
            filter: isDark 
              ? 'drop-shadow(-4px -4px 12px rgba(0,0,0,0.8)) drop-shadow(4px 4px 10px rgba(0,0,0,0.7)) drop-shadow(2px 2px 4px rgba(0,0,0,0.5))' 
              : 'drop-shadow(-4px -4px 16px rgba(100,130,170,0.5)) drop-shadow(4px 4px 14px rgba(100,130,170,0.4)) drop-shadow(2px 2px 6px rgba(100,130,170,0.2))'
         }}
      >
        <div 
          className="absolute inset-0 transition-all duration-700 pointer-events-none"
          style={{
            clipPath: 'polygon(100% 0, 0 100%, 100% 100%)',
            backdropFilter: 'blur(30px) saturate(1.4)',
            WebkitBackdropFilter: 'blur(30px) saturate(1.4)',
            background: isDark
               ? 'linear-gradient(135deg, transparent 48.5%, rgba(255,255,255,0.7) 50%, rgba(255,255,255,0.05) 54%, rgba(0,0,0,0.3) 75%, rgba(255,255,255,0.2) 98%, rgba(255,255,255,0.55) 100%)'
               : 'linear-gradient(135deg, transparent 48.5%, rgba(255,255,255,1) 50%, rgba(255,255,255,0.5) 55%, rgba(255,255,255,0.1) 80%, rgba(255,255,255,0.85) 98%, rgba(255,255,255,1) 100%)',
            borderRight: isDark ? '1px solid rgba(255,255,255,0.2)' : '1px solid rgba(255,255,255,0.7)',
            borderBottom: isDark ? '1px solid rgba(255,255,255,0.2)' : '1px solid rgba(255,255,255,0.7)',
          }}
        />
      </div>
    </button>
  );
}