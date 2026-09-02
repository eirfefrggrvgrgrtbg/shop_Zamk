import { useEffect } from 'react';
import { createPortal } from 'react-dom';
import { motion, AnimatePresence } from 'framer-motion';
import { X } from 'lucide-react';
import { lockBodyScroll, unlockBodyScroll } from './scrollLock';

interface DrawerProps {
  isOpen: boolean;
  onClose: () => void;
  title?: string;
  children: React.ReactNode;
  position?: 'left' | 'right';
  widthClassName?: string;
  hideHeader?: boolean;
}

export function Drawer({
  isOpen,
  onClose,
  title,
  children,
  position = 'left',
  widthClassName = 'w-[90vw] sm:w-[460px]',
  hideHeader = false,
}: DrawerProps) {
  useEffect(() => {
    if (isOpen) {
      lockBodyScroll();

      const handleKeyDown = (e: KeyboardEvent) => {
        if (e.key === 'Escape') {
          onClose();
        }
      };
      window.addEventListener('keydown', handleKeyDown);

      return () => {
        unlockBodyScroll();
        window.removeEventListener('keydown', handleKeyDown);
      };
    }
  }, [isOpen, onClose]);

  if (typeof document === 'undefined') return null;

  const slideDirection = position === 'left' ? -100 : 100;

  return createPortal(
    <AnimatePresence>
      {isOpen && (
        <div className="fixed inset-0 z-[100] overflow-hidden" data-testid="drawer-portal">
          {/* Backdrop: Stable static non-animated div without blur to eliminate Safari layer compositing dropout */}
          <div
            onClick={onClose}
            className="fixed inset-0 bg-graphite/40 dark:bg-black/60"
            data-testid="drawer-backdrop"
          />

          {/* Drawer Panel */}
          <motion.div
            initial={{ x: `${slideDirection}%` }}
            animate={{ x: 0 }}
            exit={{ x: `${slideDirection}%` }}
            transition={{ type: 'spring', damping: 25, stiffness: 200 }}
            className={`fixed top-0 bottom-0 ${widthClassName} bg-white dark:bg-[#111214] border-border-lighter dark:border-white/10 flex flex-col shadow-2xl z-[101]
              ${position === 'left' ? 'left-0 rounded-r-[1.5rem] border-r' : 'right-0 rounded-l-[1.5rem] border-l'}
            `}
            data-testid="drawer-panel"
          >
            {/* Header */}
            {!hideHeader && (
              <div className="flex items-center justify-between p-5 border-b border-border-lighter dark:border-white/10 flex-shrink-0">
                {title ? (
                  <h2 className="text-lg font-semibold text-graphite dark:text-white tracking-tight">{title}</h2>
                ) : (
                  <div />
                )}
                <button
                  type="button"
                  onClick={onClose}
                  className="p-2 -mr-2 text-ash hover:text-graphite dark:hover:text-white bg-ice/50 dark:bg-white/5 hover:bg-ice dark:hover:bg-white/10 rounded-full transition-all"
                  aria-label="Закрыть"
                >
                  <X className="w-5 h-5" />
                </button>
              </div>
            )}

            {/* Body */}
            <div className="p-5 overflow-y-auto flex-1 scrollbar-hide">
              {children}
            </div>
          </motion.div>
        </div>
      )}
    </AnimatePresence>,
    document.body
  );
}
