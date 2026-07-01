import { NavLink } from 'react-router-dom';
import { User, ShoppingBag, Heart, RotateCcw, Star } from 'lucide-react';
import { cn } from '../../lib/utils';

const links = [
  { to: '/account', label: 'Профиль', icon: User, end: true },
  { to: '/orders', label: 'Заказы', icon: ShoppingBag },
  { to: '/favorites', label: 'Избранное', icon: Heart },
  { to: '/orders', label: 'Возвраты', icon: RotateCcw },
  { to: '/reviews', label: 'Отзывы', icon: Star },
];

export function AccountNav() {
  return (
    <nav className="flex flex-wrap gap-2 mb-8">
      {links.map(({ to, label, icon: Icon, end }) => (
        <NavLink
          key={`${to}-${label}`}
          to={to}
          end={end}
          className={({ isActive }) =>
            cn(
              'inline-flex items-center gap-2 rounded-full px-4 py-2 text-[13px] font-medium transition-colors border',
              isActive
                ? 'bg-graphite text-white border-graphite dark:bg-white dark:text-black dark:border-white'
                : 'bg-white/60 dark:bg-white/5 text-graphite/70 dark:text-white/70 border-border-lighter dark:border-white/10 hover:bg-white dark:hover:bg-white/10'
            )
          }
        >
          <Icon className="w-4 h-4" />
          {label}
        </NavLink>
      ))}
    </nav>
  );
}
