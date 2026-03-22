import { useState, useRef, useEffect } from 'react';
import { useAuth } from '../../contexts/AuthContext';
import { Link } from 'react-router-dom';
import { LogOut, Settings, ShoppingBag, User } from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';

export function ProfileMenu() {
  const { user, logout } = useAuth();
  const [isOpen, setIsOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  // Закрытие по клику вне меню
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  return (
    <div className="relative" ref={menuRef}>
      <button 
        onClick={() => setIsOpen(!isOpen)}
        className="flex items-center gap-2 p-1.5 rounded-full hover:bg-white/40 transition-colors"
      >
        <div className="w-[26px] h-[26px] sm:w-[30px] sm:h-[30px] rounded-full bg-graphite/5 text-graphite flex items-center justify-center text-[12px] sm:text-[13px] font-medium border border-border-soft">
          {user?.name?.charAt(0).toUpperCase() || 'U'}
        </div>
      </button>

      <AnimatePresence>
        {isOpen && (
          <motion.div 
            initial={{ opacity: 0, y: 10, scale: 0.95 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: 10, scale: 0.95 }}
            transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
            className="absolute right-0 mt-4 w-60 bg-white/80 backdrop-blur-2xl rounded-3xl shadow-[0_8px_32px_rgba(100,130,170,0.15),0_1px_0_rgba(255,255,255,0.8)_inset] border border-white/50 py-2 z-50 overflow-hidden"
          >
            <div className="px-5 py-4 border-b border-border-lighter bg-white/30">
              <p className="text-[14px] font-medium text-graphite truncate">{user?.name}</p>
              <p className="text-[12px] text-graphite/50 truncate mt-0.5">{user?.email}</p>
            </div>
            
            <div className="py-2 flex flex-col">
              <Link 
                to="/profile" 
                onClick={() => setIsOpen(false)}
                className="flex items-center gap-3 px-5 py-2.5 text-[13.5px] text-graphite/70 hover:bg-white/60 hover:text-graphite transition-all"
              >
                <User className="w-[15px] h-[15px]" />
                Мой профиль
              </Link>
              <Link 
                to="/profile"  // Или отдельная страница
                onClick={() => setIsOpen(false)}
                className="flex items-center gap-3 px-5 py-2.5 text-[13.5px] text-graphite/70 hover:bg-white/60 hover:text-graphite transition-all"
              >
                <ShoppingBag className="w-[15px] h-[15px]" />
                Мои заказы
              </Link>
              <Link 
                to="/settings" // Предполагаем, что может быть
                onClick={() => setIsOpen(false)}
                className="flex items-center gap-3 px-5 py-2.5 text-[13.5px] text-graphite/70 hover:bg-white/60 hover:text-graphite transition-all"
              >
                <Settings className="w-[15px] h-[15px]" />
                Настройки
              </Link>
            </div>
            
            <div className="pt-2 pb-1 border-t border-border-lighter flex flex-col bg-white/30">
              <button 
                onClick={() => {
                  logout();
                  setIsOpen(false);
                }}
                className="flex items-center gap-3 px-5 py-2.5 text-[13.5px] text-red-500/80 hover:bg-red-50/50 hover:text-red-600 transition-all font-medium"
              >
                <LogOut className="w-[15px] h-[15px]" />
                Выйти
              </button>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}