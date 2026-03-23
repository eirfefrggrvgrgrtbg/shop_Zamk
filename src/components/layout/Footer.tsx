import { useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowRight, Instagram, Check } from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';

export function Footer() {
  const [email, setEmail] = useState('');
  const [isSubscribed, setIsSubscribed] = useState(false);

  const handleSubscribe = (e: React.FormEvent) => {
    e.preventDefault();
    if (email) {
      setIsSubscribed(true);
      setEmail('');
      setTimeout(() => setIsSubscribed(false), 5000);
    }
  };

  return (
    <footer className="relative mt-20 pb-24 md:pb-12 z-10 pt-20 border-t border-border-lighter/70">
      <div className="container mx-auto px-4 sm:px-6">
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-12 gap-12 lg:gap-8 mb-16">
          
          {/* Brand Col */}
          <div className="lg:col-span-5 pr-4">
            <Link to="/" className="inline-block mb-6">
              <span className="font-serif text-3xl font-bold tracking-tighter text-graphite">ЗАМК</span>
            </Link>
            <p className="text-sm text-graphite-light leading-relaxed max-w-sm mb-8">
              Цифровая витрина для независимых брендов и альтернативной эстетики. Кураторский подход к современной моде.
            </p>
            <div className="flex gap-4">
              <a 
                href="https://instagram.com" 
                target="_blank" 
                rel="noopener noreferrer" 
                className="w-10 h-10 rounded-full border border-border-lighter bg-white flex items-center justify-center text-graphite hover:text-primary transition-all shadow-sm hover:shadow"
                aria-label="Instagram"
              >
                <Instagram className="w-4 h-4" />
              </a>
              <a 
                href="https://t.me/telegram" 
                target="_blank" 
                rel="noopener noreferrer" 
                className="h-10 px-5 rounded-full border border-border-lighter bg-white flex items-center justify-center text-xs font-semibold text-graphite hover:text-primary transition-all uppercase tracking-widest shadow-sm hover:shadow"
              >
                Канал
              </a>
            </div>
          </div>

          {/* Links Col 1 */}
          <div className="lg:col-span-2">
            <h4 className="text-xs font-bold text-graphite uppercase tracking-widest mb-6 border-b border-border-lighter pb-3">Магазин</h4>
            <ul className="space-y-4">
              <li><Link to="/catalog" className="text-sm text-ash hover:text-primary transition-colors">Каталог</Link></li>
              <li><Link to="/collections" className="text-sm text-ash hover:text-primary transition-colors">Подборки</Link></li>
              <li><Link to="/brands" className="text-sm text-ash hover:text-primary transition-colors">Бренды</Link></li>
              <li><Link to="/catalog?collection=archive" className="text-sm text-ash hover:text-primary transition-colors">Архив</Link></li>
            </ul>
          </div>

          {/* Links Col 2 */}
          <div className="lg:col-span-2">
            <h4 className="text-xs font-bold text-graphite uppercase tracking-widest mb-6 border-b border-border-lighter pb-3">Информация</h4>
            <ul className="space-y-4">
              <li><Link to="/about" className="text-sm text-ash hover:text-primary transition-colors">О витрине</Link></li>
              <li><Link to="/returns" className="text-sm text-ash hover:text-primary transition-colors">Возврат и обмен</Link></li>
              <li><Link to="/help" className="text-sm text-ash hover:text-primary transition-colors">Вопросы и ответы</Link></li>
              <li><Link to="/contacts" className="text-sm text-ash hover:text-primary transition-colors">Контакты</Link></li>
            </ul>
          </div>

          {/* Newsletter Col */}
          <div className="lg:col-span-3">
            <h4 className="text-xs font-bold text-graphite uppercase tracking-widest mb-6 border-b border-border-lighter pb-3">Рассылка</h4>
            <p className="text-sm text-ash mb-4">Узнавайте первыми о новых кураторских подборках и лимитированных релизах.</p>
            <div className="relative">
              <form className="relative" onSubmit={handleSubscribe}>
                <input 
                  type="email" 
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="Ваша эл. почта" 
                  pattern=".*@.*"
                  title="Почта должна содержать символ @"
                  className="w-full h-12 bg-white/40 backdrop-blur-md rounded-full border border-border-soft px-5 text-sm outline-none focus:border-primary/50 transition-colors"
                  required
                />
                <button 
                  type="submit" 
                  className="absolute right-1 top-1 bottom-1 w-10 bg-graphite text-white rounded-full flex items-center justify-center hover:bg-primary transition-colors"
                >
                  <ArrowRight className="w-4 h-4" />
                </button>
              </form>
              
              {/* Soft Notification */}
              <AnimatePresence>
                {isSubscribed && (
                  <motion.div
                    initial={{ opacity: 0, y: -10, scale: 0.95 }}
                    animate={{ opacity: 1, y: 0, scale: 1 }}
                    exit={{ opacity: 0, y: 5, scale: 0.95 }}
                    className="absolute -bottom-14 left-0 right-0 bg-white/80 backdrop-blur-md border border-white px-4 py-2.5 rounded-2xl shadow-[0_4px_20px_rgba(100,130,170,0.12)] flex items-center gap-3 z-20"
                  >
                    <div className="w-5 h-5 rounded-full bg-graphite/5 flex items-center justify-center text-graphite flex-shrink-0">
                      <Check className="w-3 h-3" />
                    </div>
                    <span className="text-[12.5px] font-medium text-graphite">Вы успешно подписались</span>
                  </motion.div>
                )}
              </AnimatePresence>
            </div>
          </div>
        </div>

        {/* Bottom */}
        <div className="flex flex-col md:flex-row items-center justify-between pt-8 border-t border-border-lighter gap-4">
          <p className="text-xs text-ash-light">© 2026 ЗАМК. Все права защищены.</p>
          <div className="flex gap-6">
            <Link to="/privacy" className="text-xs text-ash-light hover:text-graphite transition-colors">Политика конфиденциальности</Link>
          </div>
        </div>
      </div>
    </footer>
  );
}
