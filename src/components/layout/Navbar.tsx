import { useEffect, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { Heart, House, Menu, Search, ShoppingBag, User } from 'lucide-react';
import { useCart } from '../../contexts/CartContext';
import { useFavorites } from '../../contexts/FavoritesContext';
import { Drawer } from '../ui/Drawer';
import { Input } from '../ui/Input';

export function Navbar() {
  const [isScrolled, setIsScrolled] = useState(false);
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const { totalItems } = useCart();
  const { favorites } = useFavorites();
  const location = useLocation();

  useEffect(() => {
    const handleScroll = () => setIsScrolled(window.scrollY > 16);
    window.addEventListener('scroll', handleScroll);
    return () => window.removeEventListener('scroll', handleScroll);
  }, []);

  const navLinks = [
    { to: '/catalog', label: 'Каталог' },
    { to: '/brands', label: 'Бренды' },
    { to: '/new', label: 'Новинки' },
    { to: '/about', label: 'О проекте' },
  ];

  return (
    <>
      <header className={`fixed top-0 w-full z-50 transition-all duration-500 ${isScrolled ? 'pt-4' : 'pt-6 md:pt-8'}`}>
        <div className="container mx-auto px-4 md:px-6">
          <div className={`mx-auto max-w-7xl rounded-[2.8rem] px-6 md:px-8 h-16 md:h-[74px] flex items-center justify-between transition-all duration-500 ${isScrolled ? 'bg-white/88 backdrop-blur-xl border border-border-lighter shadow-[0_20px_60px_rgba(130,158,186,0.14)]' : 'bg-white/70 backdrop-blur-lg border border-border-lighter/70 shadow-[0_12px_42px_rgba(130,158,186,0.08)]'}`}>
            <button className="md:hidden p-2 -ml-2 text-graphite hover:text-primary transition-colors" onClick={() => setIsMobileMenuOpen(true)}>
              <Menu className="w-5 h-5" />
            </button>

            <nav className="hidden md:flex items-center gap-8 lg:gap-12 flex-1">
              {navLinks.map((link) => (
                <Link key={link.to} to={link.to} className="text-sm font-medium text-graphite/80 hover:text-graphite transition-colors tracking-wide">
                  {link.label}
                </Link>
              ))}
            </nav>

            <Link to="/" className="absolute left-1/2 -translate-x-1/2 flex items-center justify-center">
              <span className="font-serif text-3xl font-bold tracking-tighter text-graphite">ZAMK</span>
            </Link>

            <div className="flex items-center justify-end gap-2 sm:gap-4 flex-1">
              <button className="hidden sm:flex p-2 text-graphite/80 hover:text-graphite transition-colors" aria-label="Поиск по архиву">
                <Search className="w-5 h-5" />
              </button>
              <Link to="/profile" className="hidden sm:flex p-2 text-graphite/80 hover:text-graphite transition-colors" aria-label="Профиль">
                <User className="w-5 h-5" />
              </Link>
              <Link to="/favorites" className="hidden sm:flex p-2 text-graphite/80 hover:text-graphite transition-colors relative" aria-label="Избранное">
                <Heart className="w-5 h-5" />
                {favorites.length > 0 && <span className="absolute top-1.5 right-1.5 w-2 h-2 bg-primary rounded-full ring-2 ring-white" />}
              </Link>
              <Link to="/cart" className="p-2 text-graphite/80 hover:text-graphite transition-colors relative" aria-label="Корзина">
                <ShoppingBag className="w-5 h-5" />
                {totalItems > 0 && (
                  <span className="absolute top-1 right-1 bg-graphite text-white text-[10px] font-bold w-4 h-4 rounded-full flex items-center justify-center ring-2 ring-white">
                    {totalItems}
                  </span>
                )}
              </Link>
            </div>
          </div>
        </div>
      </header>

      <Drawer isOpen={isMobileMenuOpen} onClose={() => setIsMobileMenuOpen(false)} title="Меню">
        <div className="flex flex-col gap-6 py-4">
          <Input placeholder="Поиск по архиву" isSearch className="bg-white/50" />

          <nav className="flex flex-col gap-4">
            <Link to="/" className="text-lg font-medium py-2 border-b border-border-lighter" onClick={() => setIsMobileMenuOpen(false)}>
              Главная
            </Link>
            <Link to="/catalog" className="text-lg font-medium py-2 border-b border-border-lighter" onClick={() => setIsMobileMenuOpen(false)}>
              Каталог
            </Link>
            <Link to="/brands" className="text-lg font-medium py-2 border-b border-border-lighter" onClick={() => setIsMobileMenuOpen(false)}>
              Бренды
            </Link>
            <Link to="/new" className="text-lg font-medium py-2 border-b border-border-lighter" onClick={() => setIsMobileMenuOpen(false)}>
              Новинки
            </Link>
            <Link to="/favorites" className="text-lg font-medium py-2 border-b border-border-lighter flex justify-between items-center" onClick={() => setIsMobileMenuOpen(false)}>
              Избранное
              {favorites.length > 0 && <span className="bg-primary/20 text-primary text-xs px-2 py-1 rounded-full">{favorites.length}</span>}
            </Link>
            <Link to="/profile" className="text-lg font-medium py-2 border-b border-border-lighter" onClick={() => setIsMobileMenuOpen(false)}>
              Профиль
            </Link>
            <Link to="/about" className="text-lg font-medium py-2 border-b border-border-lighter" onClick={() => setIsMobileMenuOpen(false)}>
              О проекте
            </Link>
          </nav>
        </div>
      </Drawer>

      <div className="md:hidden fixed bottom-5 left-4 right-4 z-50">
        <div className="bg-white/92 border border-border-lighter rounded-full flex justify-between items-center px-5 py-3 shadow-[0_14px_40px_rgba(130,158,186,0.18)] backdrop-blur-lg">
          <Link to="/" className={`p-2 rounded-full ${location.pathname === '/' ? 'text-graphite bg-milk' : 'text-ash'}`}>
            <House className="w-5 h-5" />
          </Link>
          <Link to="/catalog" className={`p-2 rounded-full ${location.pathname === '/catalog' ? 'text-primary bg-primary/10' : 'text-ash'}`}>
            <Search className="w-5 h-5" />
          </Link>
          <Link to="/favorites" className={`relative p-2 rounded-full ${location.pathname === '/favorites' ? 'text-primary bg-primary/10' : 'text-ash'}`}>
            <Heart className="w-5 h-5" />
            {favorites.length > 0 && <span className="absolute top-1.5 right-1.5 w-2 h-2 bg-primary rounded-full ring-2 ring-white" />}
          </Link>
          <Link to="/cart" className={`relative p-2 rounded-full ${location.pathname === '/cart' ? 'text-primary bg-primary/10' : 'text-ash'}`}>
            <ShoppingBag className="w-5 h-5" />
            {totalItems > 0 && <span className="absolute -top-0.5 -right-0.5 bg-primary text-white text-[10px] font-bold w-4 h-4 rounded-full flex items-center justify-center">{totalItems}</span>}
          </Link>
          <Link to="/profile" className={`p-2 rounded-full ${location.pathname === '/profile' ? 'text-primary bg-primary/10' : 'text-ash'}`}>
            <User className="w-5 h-5" />
          </Link>
        </div>
      </div>
    </>
  );
}
