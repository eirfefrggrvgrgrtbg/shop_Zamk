import { Eye, LayoutList } from 'lucide-react';
import { useProductStudio } from '../../contexts/ProductStudioContext';
import { cn } from '../../lib/utils';

export function ProductStudioViewToggle() {
  const { viewMode, setViewMode } = useProductStudio();

  return (
    <div
      role="tablist"
      aria-label="Режим отображения Product Studio"
      className="inline-flex items-center rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-100/80 dark:bg-slate-800/80 p-1 text-xs"
    >
      <button
        type="button"
        role="tab"
        id="studio-tab-visual"
        aria-selected={viewMode === 'visual'}
        aria-controls="studio-workspace-visual"
        tabIndex={viewMode === 'visual' ? 0 : -1}
        onClick={() => setViewMode('visual')}
        data-testid="studio-view-toggle-visual"
        className={cn(
          "inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500",
          viewMode === 'visual'
            ? "bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs"
            : "text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white"
        )}
      >
        <Eye className="w-3.5 h-3.5" aria-hidden="true" />
        <span>Визуально</span>
      </button>

      <button
        type="button"
        role="tab"
        id="studio-tab-form"
        aria-selected={viewMode === 'form'}
        aria-controls="studio-workspace-form"
        tabIndex={viewMode === 'form' ? 0 : -1}
        onClick={() => setViewMode('form')}
        data-testid="studio-view-toggle-form"
        className={cn(
          "inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500",
          viewMode === 'form'
            ? "bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs"
            : "text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white"
        )}
      >
        <LayoutList className="w-3.5 h-3.5" aria-hidden="true" />
        <span>Форма</span>
      </button>
    </div>
  );
}
