export interface ProductStatusConfig {
  label: string;
  badgeClass: string;
  dotClass: string;
  description: string;
}

export const PRODUCT_STATUS_MAP: Record<string, ProductStatusConfig> = {
  draft: {
    label: 'Черновик',
    badgeClass: 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300 border-slate-200 dark:border-slate-700',
    dotClass: 'bg-slate-400',
    description: 'Товар сохранен продавцом и не отправлен на проверку',
  },
  pending_moderation: {
    label: 'Ожидает модерации',
    badgeClass: 'bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300 border-amber-200 dark:border-amber-800/50',
    dotClass: 'bg-amber-500 animate-pulse',
    description: 'Товар отправлен продавцом и ожидает проверки модератором',
  },
  in_review: {
    label: 'На проверке',
    badgeClass: 'bg-indigo-50 text-indigo-700 dark:bg-indigo-950/40 dark:text-indigo-300 border-indigo-200 dark:border-indigo-800/50',
    dotClass: 'bg-indigo-500 animate-pulse',
    description: 'Товар находится в процессе активной проверки',
  },
  approved: {
    label: 'Одобрен',
    badgeClass: 'bg-blue-50 text-blue-700 dark:bg-blue-950/40 dark:text-blue-300 border-blue-200 dark:border-blue-800/50',
    dotClass: 'bg-blue-500',
    description: 'Товар прошел модерацию и готов к публикации',
  },
  rejected: {
    label: 'Требуются исправления',
    badgeClass: 'bg-rose-50 text-rose-700 dark:bg-rose-950/40 dark:text-rose-300 border-rose-200 dark:border-rose-800/50',
    dotClass: 'bg-rose-500',
    description: 'Товар возвращен продавцу с замечаниями модератора',
  },
  published: {
    label: 'Опубликован',
    badgeClass: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300 border-emerald-200 dark:border-emerald-800/50',
    dotClass: 'bg-emerald-500',
    description: 'Товар доступен покупателям в каталоге маркетплейса',
  },
  hidden: {
    label: 'Скрыт',
    badgeClass: 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300 border-gray-200 dark:border-gray-700',
    dotClass: 'bg-gray-400',
    description: 'Публикация товара временно приостановлена',
  },
  blocked: {
    label: 'Заблокирован',
    badgeClass: 'bg-red-100 text-red-800 dark:bg-red-950/60 dark:text-red-300 border-red-300 dark:border-red-800',
    dotClass: 'bg-red-600',
    description: 'Товар заблокирован администратором за нарушения',
  },
  out_of_stock: {
    label: 'Нет в наличии',
    badgeClass: 'bg-orange-50 text-orange-700 dark:bg-orange-950/40 dark:text-orange-300 border-orange-200 dark:border-orange-800/50',
    dotClass: 'bg-orange-500',
    description: 'Остатки всех вариантов товара равны нулю',
  },
  archived: {
    label: 'Архив',
    badgeClass: 'bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400 border-zinc-200 dark:border-zinc-700',
    dotClass: 'bg-zinc-400',
    description: 'Товар архивирован и снят с продажи',
  },
};

export function getProductStatusConfig(status: string): ProductStatusConfig {
  return PRODUCT_STATUS_MAP[status] || {
    label: status,
    badgeClass: 'bg-gray-100 text-gray-800 border-gray-200',
    dotClass: 'bg-gray-400',
    description: 'Статус товара',
  };
}
