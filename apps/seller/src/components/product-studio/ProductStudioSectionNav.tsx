import {
  PRODUCT_STUDIO_SECTIONS,
  useProductStudio,
} from '../../contexts/ProductStudioContext';
import { cn } from '../../lib/utils';

export function ProductStudioSectionNav() {
  const { activeSection, setActiveSection } = useProductStudio();

  return (
    <nav
      role="tablist"
      aria-label="Секции формы товара"
      data-testid="product-studio-section-nav"
      className="flex items-center gap-1 border-b border-gray-200 dark:border-white/10 overflow-x-auto no-scrollbar py-1"
    >
      {PRODUCT_STUDIO_SECTIONS.map((section) => {
        const isActive = activeSection === section.id;
        return (
          <button
            key={section.id}
            type="button"
            role="tab"
            id={`studio-section-tab-${section.id}`}
            aria-selected={isActive}
            aria-controls={`studio-section-panel-${section.id}`}
            tabIndex={isActive ? 0 : -1}
            onClick={() => setActiveSection(section.id)}
            data-testid={`studio-section-btn-${section.id}`}
            className={cn(
              "px-3 py-2 text-xs sm:text-sm font-medium rounded-lg whitespace-nowrap transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500",
              isActive
                ? "bg-gray-100 text-gray-900 dark:bg-white/10 dark:text-white font-semibold"
                : "text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white hover:bg-gray-50 dark:hover:bg-white/5"
            )}
          >
            {section.label}
          </button>
        );
      })}
    </nav>
  );
}
