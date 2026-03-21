import { useEffect, useState } from 'react';

export function IcyBackground() {
  const [mousePos, setMousePos] = useState({ x: 0, y: 0 });

  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      // Smoothly follow mouse for a subtle interactive glow
      requestAnimationFrame(() => {
        setMousePos({
          x: (e.clientX / window.innerWidth) * 100,
          y: (e.clientY / window.innerHeight) * 100,
        });
      });
    };

    window.addEventListener('mousemove', handleMouseMove);
    return () => window.removeEventListener('mousemove', handleMouseMove);
  }, []);

  return (
    <>
      <div className="fixed inset-0 z-[-2] bg-milk pointer-events-none overflow-hidden">
        <div className="absolute inset-0 bg-[radial-gradient(circle_at_8%_8%,rgba(255,255,255,0.95),transparent_28%),radial-gradient(circle_at_85%_16%,rgba(201,226,255,0.45),transparent_34%),radial-gradient(circle_at_64%_78%,rgba(170,214,255,0.3),transparent_38%),linear-gradient(165deg,#f7fbff_0%,#edf5ff_52%,#f5f9ff_100%)]" />

        <div className="absolute inset-0 mist-grid opacity-55" />

        <div 
          className="absolute top-[-10%] left-[-12%] w-[55vw] h-[55vh] rounded-full bg-primary/18 blur-[128px] opacity-75"
        />
        <div 
          className="absolute bottom-[-12%] right-[-8%] w-[58vw] h-[58vh] rounded-full bg-[#a7d6ff]/45 blur-[150px] opacity-60"
        />

        <div className="absolute inset-0 bg-[radial-gradient(circle_at_40%_28%,rgba(255,255,255,0.7),transparent_44%)]" />

        <div 
          className="absolute w-[40vw] h-[40vw] rounded-full bg-white/55 blur-[100px] transition-all duration-[1800ms] ease-out"
          style={{ 
            left: `${mousePos.x}%`, 
            top: `${mousePos.y}%`,
            transform: 'translate(-50%, -50%)',
          }}
        />

        <div className="absolute top-[24%] left-[14%] h-36 w-36 rounded-full border border-white/50 bg-white/28 backdrop-blur-lg" />
        <div className="absolute bottom-[18%] right-[20%] h-48 w-48 rounded-full border border-white/50 bg-[#d9edff]/36 backdrop-blur-lg" />
      </div>

      <svg id="noise-overlay" xmlns="http://www.w3.org/2000/svg">
        <filter id="noiseFilter">
          <feTurbulence 
            type="fractalNoise" 
            baseFrequency="0.9" 
            numOctaves="2" 
            stitchTiles="stitch" 
          />
          <feColorMatrix type="matrix" values="1 0 0 0 0, 0 1 0 0 0, 0 0 1 0 0, 0 0 0 0.15 0" />
        </filter>
        <rect width="100%" height="100%" filter="url(#noiseFilter)" />
      </svg>
    </>
  );
}
