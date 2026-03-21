export function IcyBackground() {
  return (
    <>
      <div className="fixed inset-0 z-[-2] bg-milk pointer-events-none overflow-hidden">
        <div className="absolute inset-0 bg-[radial-gradient(circle_at_12%_12%,rgba(255,255,255,0.95),transparent_34%),radial-gradient(circle_at_88%_12%,rgba(189,214,238,0.24),transparent_32%),radial-gradient(circle_at_76%_74%,rgba(214,228,244,0.3),transparent_46%),linear-gradient(180deg,#fbfcff_0%,#f3f8ff_58%,#f8fbff_100%)]" />
        <div className="absolute inset-0 mist-grid opacity-35" />

        <div className="absolute top-[-16%] left-[-14%] w-[56vw] h-[54vw] rounded-full bg-[#d6e7f7]/55 blur-[120px]" />
        <div className="absolute top-[18%] right-[-12%] w-[48vw] h-[46vw] rounded-full bg-[#e4eef9]/70 blur-[110px]" />
        <div className="absolute bottom-[-18%] left-[18%] w-[50vw] h-[45vw] rounded-full bg-white/80 blur-[130px]" />

        <div className="absolute top-[20%] left-[14%] h-44 w-44 rounded-full border border-white/70 bg-white/35" />
        <div className="absolute bottom-[15%] right-[16%] h-56 w-56 rounded-full border border-white/60 bg-[#eaf3ff]/42" />
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
