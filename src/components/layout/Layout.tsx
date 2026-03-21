import { Navbar } from './Navbar';
import { Footer } from './Footer';
import { ToastContainer } from '../ui/ToastContainer';
import { IcyBackground } from './IcyBackground';

interface LayoutProps {
  children: React.ReactNode;
}

export function Layout({ children }: LayoutProps) {
  return (
    <div className="relative isolate flex flex-col min-h-screen">
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
