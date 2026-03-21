import { Link } from 'react-router-dom';
import { Send } from 'lucide-react';

export function Footer() {
  return (
    <footer className="bg-white border-t border-border-lighter pt-16 pb-24 md:pb-8 mt-auto">
      <div className="container mx-auto px-4 sm:px-6 max-w-7xl">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-8 md:gap-12 mb-12">
          {/* Brand */}
          <div className="col-span-2 md:col-span-1 space-y-4">
            <h3 className="font-serif text-2xl font-bold text-graphite">ZAMK</h3>
            <p className="text-sm text-ash leading-relaxed max-w-xs">
              Кураторский fashion-магазин. Избранные бренды, продуманные подборки, бережная доставка.
            </p>
          </div>

          {/* Shop */}
          <div>
            <h4 className="font-semibold text-sm text-graphite mb-4">Магазин</h4>
            <ul className="space-y-2.5 text-sm text-ash">
              <li><Link to="/catalog?filter=new" className="hover:text-primary transition-colors">Новинки</Link></li>
              <li><Link to="/brands" className="hover:text-primary transition-colors">Бренды</Link></li>
              <li><Link to="/catalog" className="hover:text-primary transition-colors">Одежда</Link></li>
              <li><Link to="/catalog" className="hover:text-primary transition-colors">Сумки</Link></li>
              <li><Link to="/catalog" className="hover:text-primary transition-colors">Обувь</Link></li>
              <li><Link to="/catalog" className="hover:text-primary transition-colors">Аксессуары</Link></li>
            </ul>
          </div>

          {/* Info */}
          <div>
            <h4 className="font-semibold text-sm text-graphite mb-4">Информация</h4>
            <ul className="space-y-2.5 text-sm text-ash">
              <li><Link to="/about" className="hover:text-primary transition-colors">О нас</Link></li>
              <li><Link to="/delivery" className="hover:text-primary transition-colors">Доставка и возврат</Link></li>
              <li><Link to="/help" className="hover:text-primary transition-colors">Помощь</Link></li>
              <li><Link to="/contacts" className="hover:text-primary transition-colors">Контакты</Link></li>
              <li><Link to="/privacy" className="hover:text-primary transition-colors">Политика конфиденциальности</Link></li>
            </ul>
          </div>

          {/* Newsletter */}
          <div className="col-span-2 md:col-span-1">
            <h4 className="font-semibold text-sm text-graphite mb-4">Рассылка</h4>
            <p className="text-sm text-ash mb-4">Подпишитесь на новости о новых коллекциях и эксклюзивных предложениях.</p>
            <div className="flex items-center gap-2">
              <input
                type="email"
                placeholder="Ваш email"
                className="flex-1 h-10 rounded-xl border border-border-soft bg-white px-3 text-sm focus:outline-none focus:ring-2 focus:ring-primary/30 focus:border-primary/50 transition-all"
              />
              <button className="h-10 w-10 rounded-xl bg-primary text-white flex items-center justify-center hover:bg-primary-hover transition-colors shrink-0">
                <Send className="w-4 h-4" />
              </button>
            </div>
          </div>
        </div>

        <div className="pt-8 border-t border-border-lighter flex flex-col md:flex-row items-center justify-between text-xs text-ash gap-3">
          <p>&copy; {new Date().getFullYear()} ZAMK. Все права защищены.</p>
          <div className="flex gap-4">
            <Link to="/privacy" className="hover:text-primary transition-colors">Политика конфиденциальности</Link>
            <Link to="/privacy" className="hover:text-primary transition-colors">Условия использования</Link>
          </div>
        </div>
      </div>
    </footer>
  );
}
