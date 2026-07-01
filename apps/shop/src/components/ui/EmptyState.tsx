import { ShoppingBag, Heart, Search } from 'lucide-react';

interface EmptyStateProps {
  icon?: 'cart' | 'heart' | 'search' | 'default';
  title: string;
  description: string;
  action?: React.ReactNode;
}

const iconMap = {
  cart: ShoppingBag,
  heart: Heart,
  search: Search,
  default: ShoppingBag,
};

export function EmptyState({ icon = 'default', title, description, action }: EmptyStateProps) {
  const Icon = iconMap[icon];

  return (
    <div className="flex flex-col items-center justify-center py-20 px-6 text-center">
      <div className="w-20 h-20 rounded-3xl bg-surface flex items-center justify-center mb-6">
        <Icon className="w-8 h-8 text-ash-light" />
      </div>
      <h3 className="text-lg font-semibold text-graphite mb-2">{title}</h3>
      <p className="text-sm text-ash max-w-sm mb-6">{description}</p>
      {action}
    </div>
  );
}
