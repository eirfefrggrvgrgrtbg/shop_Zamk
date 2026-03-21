import { useToast } from '../../contexts/ToastContext';
import { X, Check, AlertCircle, Info } from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';

const icons = {
  success: Check,
  error: AlertCircle,
  info: Info,
};

const colors = {
  success: 'bg-success/10 border-success/30 text-success',
  error: 'bg-error/10 border-error/30 text-error',
  info: 'bg-primary/10 border-primary/30 text-primary',
};

export function ToastContainer() {
  const { toasts, removeToast } = useToast();

  return (
    <div className="fixed bottom-6 right-6 z-[100] flex flex-col gap-3 max-w-sm">
      <AnimatePresence>
        {toasts.map(toast => {
          const Icon = icons[toast.type];
          return (
            <motion.div
              key={toast.id}
              initial={{ opacity: 0, y: 16, scale: 0.96 }}
              animate={{ opacity: 1, y: 0, scale: 1 }}
              exit={{ opacity: 0, y: -8, scale: 0.96 }}
              transition={{ duration: 0.3, ease: [0.16, 1, 0.3, 1] }}
              className={`glass-strong rounded-2xl px-4 py-3 flex items-center gap-3 shadow-lg border ${colors[toast.type]}`}
            >
              <div className={`w-8 h-8 rounded-xl flex items-center justify-center shrink-0 ${colors[toast.type]}`}>
                <Icon className="w-4 h-4" />
              </div>
              <p className="text-sm text-graphite flex-1">{toast.message}</p>
              <button
                onClick={() => removeToast(toast.id)}
                className="text-ash hover:text-graphite transition-colors shrink-0"
              >
                <X className="w-4 h-4" />
              </button>
            </motion.div>
          );
        })}
      </AnimatePresence>
    </div>
  );
}
