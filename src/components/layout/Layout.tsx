import { Navbar } from './Navbar';
import { Footer } from './Footer';
import { ToastContainer } from '../ui/ToastContainer';
import { IcyBackground } from './IcyBackground';
import { ThemeFlipCorner } from '../ui/ThemeFlipCorner';

interface LayoutProps {
  children: React.ReactNode;
}

export function Layout({ children }: LayoutProps) {
  return (
    <div className="relative isolate flex flex-col min-h-screen">
      <ThemeFlipCorner />
      <IcyBackground />
      <Navbar />
      <main className="flex-grow pb-20 md:pb-0 relative z-10">
        {children}
      </main>
      <Footer />
      <ToastContainer />
    </div>
  );
}
