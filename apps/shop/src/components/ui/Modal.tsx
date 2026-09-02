import { useEffect } from 'react';
import { createPortal } from 'react-dom';
import { X } from 'lucide-react';
import { lockBodyScroll, unlockBodyScroll } from './scrollLock';

interface ModalProps {
  isOpen: boolean;
  onClose: () => void;
  title?: string;
  children: React.ReactNode;
  maxWidth?: 'sm' | 'md' | 'lg' | 'xl' | '2xl' | '3xl' | '4xl' | '5xl' | 'wide';
}

const maxWidthMap: Record<string, string> = {
  sm: 'max-w-sm',
  md: 'max-w-md',
  lg: 'max-w-lg',
  xl: 'max-w-xl',
  '2xl': 'max-w-2xl',
  '3xl': 'max-w-3xl',
  '4xl': 'max-w-4xl',
  '5xl': 'max-w-5xl',
  wide: 'max-w-[1050px]',
};

export function Modal({ isOpen, onClose, title, children, maxWidth = 'lg' }: ModalProps) {
  // Use shared scroll lock to prevent scroll jumps and layout repaints
  useEffect(() => {
    if (isOpen) {
      lockBodyScroll();

      const handleKeyDown = (e: KeyboardEvent) => {
        if (e.key === 'Escape') {
          e.stopPropagation();
          onClose();
        }
      };
      window.addEventListener('keydown', handleKeyDown, { capture: true });

      return () => {
        unlockBodyScroll();
        window.removeEventListener('keydown', handleKeyDown, { capture: true });
      };
    }
  }, [isOpen, onClose]);

  if (typeof document === 'undefined') return null;
  const portalRoot = document.getElementById('modal-root') || document.body;
  const maxWClass = maxWidthMap[maxWidth] || maxWidthMap.lg;

  return createPortal(
    <div
      className={`fixed inset-0 z-[200] overflow-hidden transition-all duration-150 ${
        isOpen
          ? 'opacity-100 pointer-events-auto visible'
          : 'opacity-0 pointer-events-none invisible select-none'
      }`}
      aria-hidden={!isOpen}
      data-testid="modal-portal"
    >
      {/* Backdrop: Stable semi-transparent overlay without blur filters */}
      <div
        onClick={onClose}
        className="fixed inset-0 z-[200] bg-graphite/40 dark:bg-black/60 transition-opacity duration-150"
        data-testid="modal-backdrop"
      />

      {/* Modal Content */}
      <div className="fixed inset-0 z-[201] flex items-center justify-center p-4 pointer-events-none">
        <div
          className={`pointer-events-auto w-full ${maxWClass} bg-white dark:bg-[#111214] border border-border-lighter dark:border-white/20 rounded-2xl overflow-hidden flex flex-col max-h-[90vh] shadow-2xl relative z-[202] transition-transform duration-150 ${
            isOpen ? 'scale-100' : 'scale-[0.99]'
          }`}
          data-testid="modal-panel"
        >
          {/* Header */}
          <div className="flex items-center justify-between p-5 border-b border-border-lighter dark:border-white/20 flex-shrink-0">
            {title && <h2 className="text-lg font-semibold text-graphite dark:text-white tracking-tight">{title}</h2>}
            <button
              type="button"
              onClick={onClose}
              className="p-2 -mr-2 text-ash hover:text-graphite dark:hover:text-white bg-ice/50 dark:bg-white/5 hover:bg-ice dark:hover:bg-white/10 rounded-full transition-all"
              aria-label="Закрыть"
              tabIndex={isOpen ? 0 : -1}
            >
              <X className="w-5 h-5" />
            </button>
          </div>

          {/* Body */}
          <div className="p-5 overflow-y-auto scrollbar-hide flex-1">
            {isOpen ? children : null}
          </div>
        </div>
      </div>
    </div>,
    portalRoot
  );
}
