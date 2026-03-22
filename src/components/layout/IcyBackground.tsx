import { useTheme } from '../../contexts/ThemeContext';

export function IcyBackground() {
  const { isDark } = useTheme();
  return (
    <>
      <div 
        className={`fixed inset-0 z-0 pointer-events-none bg-cover bg-center bg-no-repeat transition-opacity duration-1000 ${isDark ? 'opacity-0' : 'opacity-40'}`}
        style={{ backgroundImage: "url('/bg.webp')", backgroundColor: "#f5f5f5" }}
      />
      <div
        className={`fixed inset-0 z-0 pointer-events-none bg-cover bg-center bg-no-repeat transition-opacity duration-1000 ${isDark ? 'opacity-[0.05]' : 'opacity-0'}`}
        style={{ backgroundImage: "url('/bg_black.jpg')", filter: "contrast(1.2)" }}
      />
      <div 
        className={`fixed inset-0 z-0 pointer-events-none transition-opacity duration-1000 ${isDark ? 'opacity-100' : 'opacity-0'}`}
        style={{ background: "radial-gradient(circle at 50% 0%, rgba(15,15,15,0.4) 0%, rgba(5,5,5,1) 80%, #000000 100%)" }}
      />
    </>
  );
}
