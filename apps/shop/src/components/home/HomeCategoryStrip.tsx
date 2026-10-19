import { Link } from 'react-router-dom';
import type { Category } from '../../types/catalog';

/** Quick entry into catalog categories. Hidden when there is nothing real to link to. */
export function HomeCategoryStrip({ categories }: { categories: Category[] }) {
  return (
    <nav aria-label="Категории" data-testid="home-category-strip" className="-mx-4 px-4 sm:mx-0 sm:px-0">
      <ul className="flex gap-2.5 overflow-x-auto pb-2 hide-scrollbar sm:flex-wrap sm:overflow-visible">
        <li className="flex-shrink-0">
          <Link
            to="/catalog"
            className="inline-flex h-11 items-center rounded-full bg-graphite px-6 text-sm font-medium text-white transition-transform hover:-translate-y-0.5 dark:bg-white dark:text-black"
          >
            Весь каталог
          </Link>
        </li>
        {categories.map((category) => (
          <li key={category.id} className="flex-shrink-0">
            <Link
              to={`/catalog?categoryId=${encodeURIComponent(category.id)}`}
              className="inline-flex h-11 items-center rounded-full border border-border-soft bg-white/80 px-6 text-sm text-graphite transition-all hover:-translate-y-0.5 hover:border-graphite/40 dark:border-white/15 dark:bg-white/10 dark:text-white/90"
            >
              {category.name}
            </Link>
          </li>
        ))}
      </ul>
    </nav>
  );
}
