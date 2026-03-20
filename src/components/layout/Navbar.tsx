import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { ShoppingBag, Heart, User, Search } from 'lucide-react';
import { cn } from '../../lib/utils';
import { Button } from '../ui/Button';

export function Navbar() {
  const [isScrolled, setIsScrolled] = useState(false);

  useEffect(() => {
    const handleScroll = () => {
      setIsScrolled(window.scrollY > 10);
    };
    window.addEventListener('scroll', handleScroll);
    return () => window.removeEventListener('scroll', handleScroll);
  }, []);

  return (
    <header
      className={cn(
        'fixed top-0 left-0 right-0 z-50 transition-all duration-300 border-b',
        isScrolled
          ? 'bg-milk/80 backdrop-blur-md border-border-soft py-3'
          : 'bg-transparent border-transparent py-5'
      )}
    >
      <div className="container mx-auto px-6 max-w-7xl flex items-center justify-between">
        {/* Left: Navigation (Desktop) */}
        <nav className="hidden md:flex items-center gap-8 text-sm font-medium">
          <Link to="/shop" className="hover:text-dusty-blue transition-colors">New Arrivals</Link>
          <Link to="/shop" className="hover:text-dusty-blue transition-colors">Brands</Link>
          <Link to="/shop" className="hover:text-dusty-blue transition-colors">Clothing</Link>
          <Link to="/shop" className="hover:text-dusty-blue transition-colors">Accessories</Link>
        </nav>

        {/* Center: Logo */}
        <div className="flex-1 md:flex-none text-center">
          <Link to="/" className="text-2xl font-serif font-bold tracking-tight">ZAMK</Link>
        </div>

        {/* Right: Actions */}
        <div className="flex items-center justify-end gap-2 md:gap-4">
          <Button variant="ghost" size="icon" className="hidden sm:inline-flex">
            <Search className="w-5 h-5" />
          </Button>
          <Link to="/profile">
            <Button variant="ghost" size="icon" className="hidden sm:inline-flex">
              <User className="w-5 h-5" />
            </Button>
          </Link>
          <Link to="/profile">
            <Button variant="ghost" size="icon">
              <Heart className="w-5 h-5" />
            </Button>
          </Link>
          <Button variant="ghost" size="icon" className="relative">
            <ShoppingBag className="w-5 h-5" />
            <span className="absolute top-1 right-1 w-2 h-2 bg-graphite rounded-full"></span>
          </Button>
        </div>
      </div>
    </header>
  );
}
