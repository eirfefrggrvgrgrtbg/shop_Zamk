import { useState, useEffect } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { ShoppingBag, Heart, User, Search, Menu, X, Home, Grid3x3 } from 'lucide-react';
import { cn } from '../../lib/utils';
import { useCart } from '../../contexts/CartContext';
import { motion, AnimatePresence } from 'framer-motion';

const navLinks = [
  { to: '/catalog', label: 'Каталог' },
  { to: '/brands', label: 'Бренды' },
  { to: '/catalog?filter=new', label: 'Новинки' },
  { to: '/about', label: 'О нас' },
];

export function Navbar() {
  const [isScrolled, setIsScrolled] = useState(false);
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const { totalItems } = useCart();
  const location = useLocation();

  useEffect(() => {
    const handleScroll = () => setIsScrolled(window.scrollY > 10);
    window.addEventListener('scroll', handleScroll);
    return () => window.removeEventListener('scroll', handleScroll);
  }, []);

  useEffect(() => {
    setIsMobileMenuOpen(false);
  }, [location]);

  return (
    <>
      {/* Desktop & Mobile Header */}
      <header
        className={cn(
          'fixed top-0 left-0 right-0 z-50 transition-all duration-300',
          isScrolled
            ? 'glass-strong py-3 shadow-sm'
            : 'bg-milk/50 backdrop-blur-sm py-4'
        )}
      >
        <div className="container mx-auto px-4 sm:px-6 max-w-7xl flex items-center justify-between">
          {/* Left: Hamburger (mobile) + Navigation (Desktop) */}
          <div className="flex items-center gap-6">
            <button
              className="md:hidden w-10 h-10 rounded-xl flex items-center justify-center text-graphite hover:bg-surface transition-all"
              onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)}
            >
              {isMobileMenuOpen ? <X className="w-5 h-5" /> : <Menu className="w-5 h-5" />}
            </button>

            <nav className="hidden md:flex items-center gap-7">
              {navLinks.map(link => (
                <Link
                  key={link.to}
                  to={link.to}
                  className={cn(
                    "text-sm font-medium transition-colors hover:text-primary",
                    location.pathname === link.to ? 'text-primary' : 'text-graphite-light'
                  )}
                >
                  {link.label}
                </Link>
              ))}
            </nav>
          </div>

          {/* Center: Logo */}
          <Link to="/" className="text-2xl font-serif font-bold tracking-tight text-graphite">
            ZAMK
          </Link>

          {/* Right: Actions */}
          <div className="flex items-center gap-1 sm:gap-2">
            <Link to="/catalog" className="hidden sm:flex w-10 h-10 rounded-xl items-center justify-center text-ash hover:text-graphite hover:bg-surface transition-all">
              <Search className="w-5 h-5" />
            </Link>
            <Link to="/profile" className="hidden sm:flex w-10 h-10 rounded-xl items-center justify-center text-ash hover:text-graphite hover:bg-surface transition-all">
              <User className="w-5 h-5" />
            </Link>
            <Link to="/favorites" className="w-10 h-10 rounded-xl flex items-center justify-center text-ash hover:text-graphite hover:bg-surface transition-all">
              <Heart className="w-5 h-5" />
            </Link>
            <Link to="/cart" className="relative w-10 h-10 rounded-xl flex items-center justify-center text-ash hover:text-graphite hover:bg-surface transition-all">
              <ShoppingBag className="w-5 h-5" />
              {totalItems > 0 && (
                <span className="absolute -top-0.5 -right-0.5 w-5 h-5 bg-primary text-white text-[10px] font-bold rounded-full flex items-center justify-center">
                  {totalItems}
                </span>
              )}
            </Link>
          </div>
        </div>
      </header>

      {/* Mobile Menu Overlay */}
      <AnimatePresence>
        {isMobileMenuOpen && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 z-40 bg-graphite/20 backdrop-blur-sm md:hidden"
            onClick={() => setIsMobileMenuOpen(false)}
          >
            <motion.div
              initial={{ x: '-100%' }}
              animate={{ x: 0 }}
              exit={{ x: '-100%' }}
              transition={{ duration: 0.3, ease: [0.16, 1, 0.3, 1] }}
              className="absolute top-0 left-0 h-full w-72 glass-strong shadow-2xl pt-20 px-6"
              onClick={e => e.stopPropagation()}
            >
              <nav className="flex flex-col gap-2">
                {navLinks.map(link => (
                  <Link
                    key={link.to}
                    to={link.to}
                    className={cn(
                      "px-4 py-3 rounded-xl text-base font-medium transition-all",
                      location.pathname === link.to
                        ? 'bg-primary/10 text-primary'
                        : 'text-graphite hover:bg-surface'
                    )}
                  >
                    {link.label}
                  </Link>
                ))}
                <hr className="my-3 border-border-lighter" />
                <Link to="/delivery" className="px-4 py-3 rounded-xl text-sm text-ash hover:bg-surface transition-all">
                  Доставка и возврат
                </Link>
                <Link to="/help" className="px-4 py-3 rounded-xl text-sm text-ash hover:bg-surface transition-all">
                  Помощь
                </Link>
                <Link to="/contacts" className="px-4 py-3 rounded-xl text-sm text-ash hover:bg-surface transition-all">
                  Контакты
                </Link>
              </nav>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Mobile Bottom Tab Bar */}
      <div className="fixed bottom-0 left-0 right-0 z-50 md:hidden glass-strong border-t border-border-lighter">
        <nav className="flex items-center justify-around py-2 px-4">
          {[
            { to: '/', icon: Home, label: 'Главная' },
            { to: '/catalog', icon: Grid3x3, label: 'Каталог' },
            { to: '/favorites', icon: Heart, label: 'Избранное' },
            { to: '/cart', icon: ShoppingBag, label: 'Корзина', badge: totalItems },
            { to: '/profile', icon: User, label: 'Профиль' },
          ].map(item => {
            const isActive = location.pathname === item.to;
            return (
              <Link
                key={item.to}
                to={item.to}
                className={cn(
                  "flex flex-col items-center gap-0.5 py-1 px-3 rounded-xl transition-all relative",
                  isActive ? 'text-primary' : 'text-ash'
                )}
              >
                <item.icon className="w-5 h-5" />
                <span className="text-[10px] font-medium">{item.label}</span>
                {item.badge && item.badge > 0 && (
                  <span className="absolute -top-1 right-0 w-4 h-4 bg-primary text-white text-[8px] font-bold rounded-full flex items-center justify-center">
                    {item.badge}
                  </span>
                )}
              </Link>
            );
          })}
        </nav>
      </div>
    </>
  );
}
