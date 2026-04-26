import { useEffect, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { Heart, House, LayoutGrid, Menu, Search, ShoppingBag, User } from 'lucide-react';
import { useCart } from '../../contexts/CartContext';
import { useFavorites } from '../../contexts/FavoritesContext';
import { useAuth } from '../../contexts/AuthContext';
import { useSearch } from '../../contexts/SearchContext';
import { Drawer } from '../ui/Drawer';
import { Input } from '../ui/Input';
import { ProfileMenu } from '../auth/ProfileMenu';
import { Sidebar } from './Sidebar';

export function Navbar() {
  const [isScrolled, setIsScrolled] = useState(false);
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);
  const { totalItems } = useCart();
  const { favorites } = useFavorites();
  const { isAuthenticated, openAuthModal } = useAuth();
  const { openSearch } = useSearch();
  const location = useLocation();

  useEffect(() => {
    const handleScroll = () => setIsScrolled(window.scrollY > 20);
    window.addEventListener('scroll', handleScroll);
    return () => window.removeEventListener('scroll', handleScroll);
  }, []);

  const navLinks = [
    { to: '/catalog', label: 'Каталог' },
    { to: '/brands', label: 'Бренды' },
    { to: '/seller-dashboard', label: 'Продавцам' },
  ];

  return (
    <>
      <header className={`fixed w-full z-50 transition-all duration-700 ease-out left-0 right-0 ${
        isScrolled ? 'top-2 md:top-3' : 'top-3 md:top-5'
      }`}>
        <div className="max-w-[1400px] mx-auto px-4 sm:px-6">
          <div className={`h-[60px] md:h-[68px] rounded-full flex items-center justify-between transition-all duration-700 px-5 md:px-7 ${
            isScrolled
                ? 'bg-white/60 dark:bg-black/40 backdrop-blur-2xl border border-white/50 dark:border-white/10 shadow-[0_4px_24px_rgba(100,130,170,0.14),0_1px_0_rgba(255,255,255,0.8)_inset] dark:shadow-[0_4px_24px_rgba(0,0,0,0.4),0_1px_0_rgba(255,255,255,0.05)_inset]'
                : 'bg-white/20 dark:bg-black/20 backdrop-blur-xl border border-white/30 dark:border-white/5 shadow-[0_2px_16px_rgba(120,155,190,0.10),0_1px_0_rgba(255,255,255,0.6)_inset] dark:shadow-[0_2px_16px_rgba(0,0,0,0.2),0_1px_0_rgba(255,255,255,0.02)_inset]'
            }`}>

            {/* Mobile burger */}
            <button
              className="md:hidden p-2 -ml-1 text-graphite/70 dark:text-white/70 hover:text-graphite dark:hover:text-white transition-colors"
              onClick={() => setIsMobileMenuOpen(true)}
            >
              <Menu className="w-[18px] h-[18px]" />
            </button>

            {/* Left nav links */}
            <nav className="hidden md:flex items-center gap-8 lg:gap-10 flex-1">
              {navLinks.map((link) => (
                <Link
                  key={link.to}
                  to={link.to}
                  className={`text-[13.5px] font-medium transition-all duration-300 tracking-[0.04em] uppercase ${
                    location.pathname === link.to
                      ? 'text-graphite dark:text-white'
                      : 'text-graphite/50 dark:text-white/50 hover:text-graphite/80 dark:hover:text-white/80'
                  }`}
                >
                  {link.label}
                </Link>
              ))}
            </nav>

            {/* Center logo */}
            <Link
              to="/"
              className="absolute left-1/2 -translate-x-1/2 flex items-center justify-center"
            >
              <span className={`font-serif text-[34px] md:text-[40px] font-medium tracking-[0.06em] transition-colors duration-500 ${
                isScrolled ? 'text-graphite dark:text-white' : 'text-graphite/85 dark:text-white/85'
              }`}>
                ЗАМК
              </span>
            </Link>

            {/* Right actions */}
            <div className="flex items-center justify-end gap-0.5 sm:gap-1 flex-1">
              <button
                onClick={openSearch}
                className="hidden sm:flex p-2.5 text-graphite/50 hover:text-graphite dark:text-white/50 dark:hover:text-white transition-colors rounded-full hover:bg-white/40 dark:hover:bg-white/10"
                aria-label="Поиск"
              >
                <Search className="w-[17px] h-[17px]" />
              </button>
              
              {isAuthenticated ? (
                <ProfileMenu />
              ) : (
                <button
                  onClick={() => openAuthModal('login')}
                  className="hidden sm:flex p-2.5 text-[13px] text-graphite/50 hover:text-graphite dark:text-white/50 dark:hover:text-white transition-colors font-medium tracking-[0.04em] rounded-full hover:bg-white/40 dark:hover:bg-white/10 uppercase"
                  aria-label="Вход"
                >
                  Вход
                </button>
              )}

              <Link
                to="/favorites"
                className="hidden sm:flex p-2.5 text-graphite/50 hover:text-graphite dark:text-white/50 dark:hover:text-white transition-colors relative rounded-full hover:bg-white/40 dark:hover:bg-white/10"
                aria-label="Избранное"
              >
                <Heart className="w-[17px] h-[17px]" />
                {favorites.length > 0 && (
                  <span className="absolute top-2 right-2 w-1.5 h-1.5 bg-primary dark:bg-white rounded-full ring-2 ring-white/80 dark:ring-[#111214]/80" />
                )}
              </Link>
              <Link
                to="/cart"
                className="p-2.5 text-graphite/50 hover:text-graphite dark:text-white/50 dark:hover:text-white transition-colors relative rounded-full hover:bg-white/40 dark:hover:bg-white/10"
                aria-label="Корзина"
              >
                <ShoppingBag className="w-[17px] h-[17px]" />
                {totalItems > 0 && (
                  <span className="absolute top-1.5 right-1.5 bg-primary dark:bg-white text-white dark:text-black text-[9px] font-bold w-[16px] h-[16px] rounded-full flex items-center justify-center shadow-sm">
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
          <div onClick={() => { setIsMobileMenuOpen(false); openSearch(); }}>
            <Input placeholder="Поиск" isSearch className="bg-white/50 pointer-events-none" readOnly />
          </div>
          <nav className="flex flex-col gap-4">
            <Link to="/" className="text-lg font-medium py-2 border-b border-border-lighter" onClick={() => setIsMobileMenuOpen(false)}>
              Главная
            </Link>
            <Link to="/catalog" className="text-lg font-medium py-2 border-b border-border-lighter" onClick={() => setIsMobileMenuOpen(false)}>
              Каталог
            </Link>
            <Link to="/collections" className="text-lg font-medium py-2 border-b border-border-lighter" onClick={() => setIsMobileMenuOpen(false)}>
              Подборки
            </Link>
            <Link to="/brands" className="text-lg font-medium py-2 border-b border-border-lighter" onClick={() => setIsMobileMenuOpen(false)}>
              Бренды
            </Link>
            <Link to="/seller-dashboard" className="text-lg font-medium py-2 border-b border-border-lighter" onClick={() => setIsMobileMenuOpen(false)}>
              Продавцам
            </Link>
            <Link to="/favorites" className="text-lg font-medium py-2 border-b border-border-lighter flex justify-between items-center" onClick={() => setIsMobileMenuOpen(false)}>
              Избранное
              {favorites.length > 0 && <span className="bg-primary/20 text-primary text-xs px-2 py-1 rounded-full">{favorites.length}</span>}
            </Link>

            {isAuthenticated ? (
              <Link to="/profile" className="text-lg font-medium py-2 border-b border-border-lighter" onClick={() => setIsMobileMenuOpen(false)}>
                Профиль
              </Link>
            ) : (
              <button
                className="text-lg font-medium py-2 border-b border-border-lighter text-left"
                onClick={() => {
                  setIsMobileMenuOpen(false);
                  openAuthModal('login');
                }}
              >
                Вход / Регистрация
              </button>
            )}
          </nav>
        </div>
      </Drawer>

      {/* Mobile bottom nav */}
      <div className="md:hidden fixed bottom-4 left-4 right-4 z-50">
        <div className="bg-white/90 dark:bg-[#111214]/90 border border-white/70 dark:border-white/10 rounded-full flex justify-between items-center px-5 py-3 shadow-[0_12px_36px_rgba(110,140,170,0.18)] dark:shadow-[0_12px_36px_rgba(0,0,0,0.5)] backdrop-blur-xl">
          <Link to="/" className={`p-2 rounded-full transition-colors ${location.pathname === '/' ? 'text-graphite dark:text-white bg-ice dark:bg-white/10' : 'text-ash dark:text-gray-400'}`}>
            <House className="w-5 h-5" />
          </Link>
          <button onClick={openSearch} className={`p-2 rounded-full transition-colors ${location.pathname === '/catalog' ? 'text-primary dark:text-white bg-primary/10 dark:bg-white/10' : 'text-ash dark:text-gray-400'}`}>
            <Search className="w-5 h-5" />
          </button>
          <Link to="/favorites" className={`relative p-2 rounded-full transition-colors ${location.pathname === '/favorites' ? 'text-primary dark:text-white bg-primary/10 dark:bg-white/10' : 'text-ash dark:text-gray-400'}`}>
            <Heart className="w-5 h-5" />
            {favorites.length > 0 && <span className="absolute top-1.5 right-1.5 w-1.5 h-1.5 bg-primary dark:bg-white rounded-full ring-2 ring-white dark:ring-[#111214]" />}
          </Link>
          <Link to="/cart" className={`relative p-2 rounded-full transition-colors ${location.pathname === '/cart' ? 'text-primary dark:text-white bg-primary/10 dark:bg-white/10' : 'text-ash dark:text-gray-400'}`}>
            <ShoppingBag className="w-5 h-5" />
            {totalItems > 0 && <span className="absolute -top-0.5 -right-0.5 bg-primary dark:bg-white text-white dark:text-black text-[10px] font-bold w-4 h-4 rounded-full flex items-center justify-center">{totalItems}</span>}
          </Link>
          
          {isAuthenticated ? (
            <Link to="/profile" className={`p-2 rounded-full transition-colors ${location.pathname === '/profile' ? 'text-primary dark:text-white bg-primary/10 dark:bg-white/10' : 'text-ash dark:text-gray-400'}`}>
              <User className="w-5 h-5" />
            </Link>
          ) : (
            <button 
              onClick={() => openAuthModal('login')}
              className={`p-2 rounded-full transition-colors text-ash dark:text-gray-400`}
            >
              <User className="w-5 h-5" />
            </button>
          )}
        </div>
      </div>

      {/* Sidebar */}
      <Sidebar isOpen={isSidebarOpen} onClose={() => setIsSidebarOpen(false)} />
    </>
  );
}
