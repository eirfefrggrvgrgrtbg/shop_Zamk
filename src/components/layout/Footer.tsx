import { Link } from 'react-router-dom';
import { ArrowRight, Instagram } from 'lucide-react';

export function Footer() {
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
              <a href="#" className="w-10 h-10 rounded-full border border-border-lighter bg-white flex items-center justify-center text-graphite hover:text-primary transition-all">
                <Instagram className="w-4 h-4" />
              </a>
              <a href="#" className="h-10 px-4 rounded-full border border-border-lighter bg-white flex items-center justify-center text-xs font-medium text-graphite hover:text-primary transition-all uppercase tracking-widest">
                Канал
              </a>
            </div>
          </div>

          {/* Links Col 1 */}
          <div className="lg:col-span-2">
            <h4 className="text-xs font-bold text-graphite uppercase tracking-widest mb-6 border-b border-border-lighter pb-3">Магазин</h4>
            <ul className="space-y-4">
              <li><Link to="/catalog" className="text-sm text-ash hover:text-primary transition-colors">Каталог</Link></li>
              <li><Link to="/brands" className="text-sm text-ash hover:text-primary transition-colors">Бренды</Link></li>
              <li><Link to="/new" className="text-sm text-ash hover:text-primary transition-colors">Новинки</Link></li>
              <li><Link to="/catalog?sort=sale" className="text-sm text-ash hover:text-primary transition-colors">Скидки</Link></li>
            </ul>
          </div>

          {/* Links Col 2 */}
          <div className="lg:col-span-2">
            <h4 className="text-xs font-bold text-graphite uppercase tracking-widest mb-6 border-b border-border-lighter pb-3">Информация</h4>
            <ul className="space-y-4">
              <li><Link to="/about" className="text-sm text-ash hover:text-primary transition-colors">О нас</Link></li>
              <li><Link to="/delivery" className="text-sm text-ash hover:text-primary transition-colors">Доставка и возврат</Link></li>
              <li><Link to="/help" className="text-sm text-ash hover:text-primary transition-colors">Вопросы и ответы</Link></li>
              <li><Link to="/contacts" className="text-sm text-ash hover:text-primary transition-colors">Контакты</Link></li>
            </ul>
          </div>

          {/* Newsletter Col */}
          <div className="lg:col-span-3">
            <h4 className="text-xs font-bold text-graphite uppercase tracking-widest mb-6 border-b border-border-lighter pb-3">Рассылка</h4>
            <p className="text-sm text-ash mb-4">Узнавайте первыми о новых кураторских подборках и лимитированных релизах.</p>
            <form className="relative" onSubmit={(e) => e.preventDefault()}>
              <input 
                type="email" 
                placeholder="Ваша эл. почта" 
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
