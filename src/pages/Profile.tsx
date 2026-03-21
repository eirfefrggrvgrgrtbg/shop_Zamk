import { Link } from 'react-router-dom';
import { User, Package, MapPin, Heart, Settings, Bell, ChevronRight, LogOut } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { useFavorites } from '../contexts/FavoritesContext';
import { useCart } from '../contexts/CartContext';

const menuSections = [
  {
    items: [
      { icon: Package, label: 'Мои заказы', to: '/profile', badge: '2' },
      { icon: MapPin, label: 'Адреса', to: '/profile' },
      { icon: Heart, label: 'Избранное', to: '/favorites' },
    ],
  },
  {
    items: [
      { icon: Settings, label: 'Настройки', to: '/profile' },
      { icon: Bell, label: 'Уведомления', to: '/profile' },
    ],
  },
];

export function Profile() {
  const { favorites } = useFavorites();
  const { totalItems } = useCart();

  return (
    <div className="min-h-screen bg-milk">
      <div className="container mx-auto px-4 sm:px-6 max-w-2xl py-8">
        <h1 className="text-3xl font-serif text-graphite mb-8">Профиль</h1>

        {/* Avatar & Info */}
        <div className="bg-white rounded-3xl border border-border-lighter p-6 mb-6 flex items-center gap-5">
          <div className="w-20 h-20 rounded-2xl bg-surface flex items-center justify-center shrink-0">
            <User className="w-8 h-8 text-ash" />
          </div>
          <div>
            <h2 className="text-lg font-semibold text-graphite">Анна Прокофьева</h2>
            <p className="text-sm text-ash">Партнер брендов · Монада</p>
            <div className="flex gap-4 mt-2 text-xs text-ash">
              <span>Заказов: 2</span>
              <span>В избранном: {favorites.length}</span>
            </div>
          </div>
        </div>

        {/* Quick stats */}
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-6">
          {[
            { label: 'Заказы', value: '2', to: '/profile' },
            { label: 'Избранное', value: String(favorites.length), to: '/favorites' },
            { label: 'Корзина', value: String(totalItems), to: '/cart' },
            { label: 'Бонусы', value: '850', to: '/profile' },
          ].map(stat => (
            <Link
              key={stat.label}
              to={stat.to}
              className="bg-white rounded-2xl border border-border-lighter p-4 text-center hover:border-primary/30 transition-all"
            >
              <p className="text-2xl font-semibold text-graphite">{stat.value}</p>
              <p className="text-xs text-ash mt-0.5">{stat.label}</p>
            </Link>
          ))}
        </div>

        {/* Menu sections */}
        <div className="space-y-3">
          {menuSections.map((section, si) => (
            <div key={si} className="bg-white rounded-2xl border border-border-lighter overflow-hidden">
              {section.items.map((item, ii) => (
                <Link
                  key={ii}
                  to={item.to}
                  className="flex items-center gap-3 px-5 py-4 hover:bg-surface/50 transition-colors border-b border-border-lighter last:border-0"
                >
                  <div className="w-9 h-9 rounded-xl bg-surface flex items-center justify-center shrink-0">
                    <item.icon className="w-4.5 h-4.5 text-ash" />
                  </div>
                  <span className="text-sm font-medium text-graphite flex-1">{item.label}</span>
                  {item.badge && (
                    <span className="text-xs bg-primary/10 text-primary font-semibold px-2 py-0.5 rounded-full">{item.badge}</span>
                  )}
                  <ChevronRight className="w-4 h-4 text-ash-light" />
                </Link>
              ))}
            </div>
          ))}
        </div>

        {/* My Brands */}
        <div className="mt-8">
          <h3 className="text-lg font-semibold text-graphite mb-4">Мои бренды</h3>
          <div className="grid grid-cols-2 gap-3">
            {['Монада', 'Шталь Эйр'].map(brand => (
              <div key={brand} className="bg-white rounded-2xl border border-border-lighter p-4 text-center">
                <div className="w-14 h-14 rounded-xl bg-surface flex items-center justify-center mx-auto mb-2">
                  <span className="text-lg font-serif text-graphite">{brand[0]}</span>
                </div>
                <p className="text-sm font-medium text-graphite">{brand}</p>
              </div>
            ))}
          </div>
        </div>

        {/* Logout */}
        <button className="flex items-center gap-2.5 mt-8 text-sm text-ash hover:text-error transition-colors">
          <LogOut className="w-4 h-4" />
          Выйти из аккаунта
        </button>
      </div>
    </div>
  );
}
