export function IcyBackground() {
  return (
    <div 
      className="fixed inset-0 z-0 pointer-events-none bg-cover bg-center bg-no-repeat opacity-[0.4]" 
      style={{ backgroundImage: "url('/bg.webp')", backgroundColor: "#f5f5f5" }} 
    />
  );
}
