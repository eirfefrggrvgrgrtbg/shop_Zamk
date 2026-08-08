import React, { useEffect, useRef } from 'react';

interface FilterPopoverProps {
  isOpen: boolean;
  onClose: () => void;
  onReset: () => void;
  onApply: () => void;
  children: React.ReactNode;
  widthClass?: string;
}

export const FilterPopover: React.FC<FilterPopoverProps> = ({
  isOpen,
  onClose,
  onReset,
  onApply,
  children,
  widthClass = 'w-64',
}) => {
  const popoverRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isOpen) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };

    // Use pointerdown on document to close when clicking outside.
    // We use setTimeout(0) to let React's synthetic events (checkbox onChange)
    // fire BEFORE we close the popover. Without this delay, mousedown on a
    // checkbox fires the document listener before React updates state.
    const handlePointerDown = (e: PointerEvent) => {
      if (popoverRef.current && !popoverRef.current.contains(e.target as Node)) {
        setTimeout(() => onClose(), 0);
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    document.addEventListener('pointerdown', handlePointerDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
      document.removeEventListener('pointerdown', handlePointerDown);
    };
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  return (
    <div
      ref={popoverRef}
      className={`absolute left-0 top-full mt-2 ${widthClass} bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-2xl shadow-xl z-50 p-4 text-xs animate-in fade-in zoom-in-95`}
    >
      <div className="space-y-3">
        {children}
      </div>

      {/* Popover Footer Buttons */}
      <div className="flex items-center justify-between pt-3 mt-3 border-t border-gray-100 dark:border-gray-700">
        <button
          type="button"
          data-testid="filter-reset-btn"
          onClick={onReset}
          className="text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white font-medium hover:underline px-1 py-0.5"
        >
          Сбросить
        </button>
        <button
          type="button"
          data-testid="filter-apply-btn"
          onClick={onApply}
          className="px-4 py-1.5 bg-black hover:bg-gray-800 dark:bg-white dark:hover:bg-gray-200 text-white dark:text-black font-bold rounded-xl transition-colors shadow-sm text-xs"
        >
          Готово
        </button>
      </div>
    </div>
  );
};
