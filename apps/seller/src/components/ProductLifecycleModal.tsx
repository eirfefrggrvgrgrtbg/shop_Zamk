import { useEffect } from 'react';
import { cn } from '../lib/utils';

export interface ProductLifecycleModalProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  body: string;
  confirmLabel: string;
  cancelLabel?: string;
  isDestructive?: boolean;
  isLoading?: boolean;
  confirmTestId?: string;
  cancelTestId?: string;
}

export function ProductLifecycleModal({
  isOpen,
  onClose,
  onConfirm,
  title,
  body,
  confirmLabel,
  cancelLabel = 'Отмена',
  isDestructive = false,
  isLoading = false,
  confirmTestId = 'lifecycle-confirm-btn',
  cancelTestId = 'lifecycle-cancel-btn',
}: ProductLifecycleModalProps) {
  useEffect(() => {
    if (!isOpen) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !isLoading) {
        onClose();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose, isLoading]);

  if (!isOpen) return null;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="lifecycle-modal-title"
      data-testid="lifecycle-modal"
      className="fixed inset-0 z-50 flex items-center justify-center p-4 overflow-y-auto"
    >
      {/* Backdrop */}
      <div
        data-testid="lifecycle-modal-backdrop"
        className="fixed inset-0 bg-black/40 backdrop-blur-xs transition-opacity animate-in fade-in duration-150"
        onClick={() => {
          if (!isLoading) onClose();
        }}
        aria-hidden="true"
      />

      {/* Modal Dialog Card */}
      <div className="relative w-full max-w-md rounded-xl border border-gray-200 dark:border-white/10 bg-white dark:bg-gray-900 p-6 shadow-2xl transition-all animate-in zoom-in-95 duration-150">
        <h3
          id="lifecycle-modal-title"
          className="text-lg font-semibold tracking-tight text-gray-900 dark:text-white"
        >
          {title}
        </h3>
        <p className="mt-2 text-sm leading-relaxed text-gray-600 dark:text-gray-300">
          {body}
        </p>

        <div className="mt-6 flex items-center justify-end gap-3">
          <button
            type="button"
            data-testid={cancelTestId}
            disabled={isLoading}
            onClick={onClose}
            className="inline-flex h-9 items-center justify-center rounded-lg border border-gray-200 bg-white px-4 text-sm font-medium text-gray-700 shadow-xs hover:bg-gray-50 hover:text-gray-900 disabled:opacity-50 dark:border-white/10 dark:bg-white/5 dark:text-gray-300 dark:hover:bg-white/10 dark:hover:text-white transition-colors cursor-pointer"
          >
            {cancelLabel}
          </button>
          <button
            type="button"
            data-testid={confirmTestId}
            disabled={isLoading}
            onClick={onConfirm}
            className={cn(
              "inline-flex h-9 items-center justify-center rounded-lg px-4 text-sm font-medium text-white shadow-xs transition-colors disabled:opacity-50 cursor-pointer",
              isDestructive
                ? "bg-red-600 hover:bg-red-700 focus-visible:ring-red-600"
                : "bg-gray-900 hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-100"
            )}
          >
            {isLoading ? 'Выполняется...' : confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
