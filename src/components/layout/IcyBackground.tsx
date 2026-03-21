export function IcyBackground() {
  return (
    <>
      <div className="fixed inset-0 z-[-2] pointer-events-none overflow-hidden bg-[#EEF4FA] transition-colors duration-1000">
        <div className="absolute -top-[12%] left-[-5%] h-[46%] w-[58%] rounded-full bg-[#FFFFFF] blur-[110px] opacity-95" />
        <div className="absolute -top-[8%] right-[-8%] h-[40%] w-[48%] rounded-full bg-[#DDEAF7] blur-[105px] opacity-75" />
        <div className="absolute top-[28%] left-[10%] h-[35%] w-[40%] rounded-full bg-[#F8FCFF] blur-[95px] opacity-95" />
        <div className="absolute bottom-[-14%] left-[5%] h-[48%] w-[56%] rounded-full bg-[#D4E3F1] blur-[110px] opacity-70" />
        <div className="absolute bottom-[-20%] right-[-10%] h-[52%] w-[54%] rounded-full bg-[#FFFFFF] blur-[120px] opacity-90" />
      </div>

      <svg id="noise-overlay" xmlns="http://www.w3.org/2000/svg" className="pointer-events-none fixed inset-0 z-[-1] opacity-30 mix-blend-overlay">
        <filter id="noiseFilter">
          <feTurbulence 
            type="fractalNoise" 
            baseFrequency="0.9" 
            numOctaves="2" 
            stitchTiles="stitch" 
          />
          <feColorMatrix type="matrix" values="1 0 0 0 0, 0 1 0 0 0, 0 0 1 0 0, 0 0 0 0.04 0" />
        </filter>
        <rect width="100%" height="100%" filter="url(#noiseFilter)" />
      </svg>
    </>
  );
}
