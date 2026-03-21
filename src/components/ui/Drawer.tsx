import { motion, AnimatePresence } from 'framer-motion';
import { X } from 'lucide-react';

interface DrawerProps {
  isOpen: boolean;
  onClose: () => void;
  title?: string;
  children: React.ReactNode;
  side?: 'right' | 'left';
}

export function Drawer({ isOpen, onClose, title, children, side = 'right' }: DrawerProps) {
  const isRight = side === 'right';

  return (
    <AnimatePresence>
      {isOpen && (
        <div className="fixed inset-0 z-[80]">
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.2 }}
            className="absolute inset-0 bg-graphite/20 backdrop-blur-sm"
            onClick={onClose}
          />
          <motion.div
            initial={{ x: isRight ? '100%' : '-100%' }}
            animate={{ x: 0 }}
            exit={{ x: isRight ? '100%' : '-100%' }}
            transition={{ duration: 0.35, ease: [0.16, 1, 0.3, 1] }}
            className={`absolute top-0 ${isRight ? 'right-0' : 'left-0'} h-full w-full max-w-md glass-strong shadow-2xl`}
          >
            <div className="h-full flex flex-col">
              {title && (
                <div className="flex items-center justify-between p-5 border-b border-border-lighter">
                  <h3 className="text-lg font-semibold text-graphite">{title}</h3>
                  <button
                    onClick={onClose}
                    className="w-8 h-8 rounded-xl flex items-center justify-center text-ash hover:text-graphite hover:bg-surface transition-all"
                  >
                    <X className="w-4 h-4" />
                  </button>
                </div>
              )}
              <div className="flex-1 overflow-y-auto p-5">
                {children}
              </div>
            </div>
          </motion.div>
        </div>
      )}
    </AnimatePresence>
  );
}
