import { useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { X } from 'lucide-react';

interface DrawerProps {
  isOpen: boolean;
  onClose: () => void;
  title?: string;
  children: React.ReactNode;
  position?: 'left' | 'right';
}

export function Drawer({ isOpen, onClose, title, children, position = 'left' }: DrawerProps) {
  useEffect(() => {
    if (isOpen) {
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = 'unset';
    }
    return () => { document.body.style.overflow = 'unset'; };
  }, [isOpen]);

  const slideDirection = position === 'left' ? -100 : 100;

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={onClose}
            className="fixed inset-0 z-50 bg-graphite/20 backdrop-blur-sm"
          />

          <motion.div
            initial={{ x: `${slideDirection}%` }}
            animate={{ x: 0 }}
            exit={{ x: `${slideDirection}%` }}
            transition={{ type: "spring", damping: 25, stiffness: 200 }}
            className={`fixed top-0 bottom-0 z-50 w-[85vw] sm:w-[400px] glass-panel-strong flex flex-col
              ${position === 'left' ? 'left-0 rounded-r-[2rem]' : 'right-0 rounded-l-[2rem]'}
            `}
          >
            {/* Header */}
            <div className="flex items-center justify-between p-6 border-b border-border-lighter">
              {title && <h2 className="text-xl font-serif font-semibold text-graphite">{title}</h2>}
              <button
                onClick={onClose}
                className="p-2 -mr-2 text-ash hover:text-graphite bg-white/40 hover:bg-white/80 rounded-full transition-all"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {/* Body */}
            <div className="p-6 overflow-y-auto flex-1 scrollbar-hide">
              {children}
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
}
