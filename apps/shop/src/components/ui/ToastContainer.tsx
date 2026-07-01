import { AnimatePresence, motion } from 'framer-motion';
import { useToast } from '../../contexts/ToastContext';
import { CheckCircle2, XCircle, Info, X } from 'lucide-react';

export function ToastContainer() {
  const { toasts, removeToast } = useToast();

  const icons = {
    success: <CheckCircle2 className="w-5 h-5 text-success" />,
    error: <XCircle className="w-5 h-5 text-error" />,
    info: <Info className="w-5 h-5 text-primary" />,
  };

  return (
    <div className="fixed bottom-24 md:bottom-6 right-0 md:right-6 z-50 flex flex-col gap-3 px-4 md:px-0 pointer-events-none w-full md:w-auto max-w-sm">
      <AnimatePresence>
        {toasts.map((toast) => (
          <motion.div
            key={toast.id}
            initial={{ opacity: 0, y: 50, scale: 0.9 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, scale: 0.9, transition: { duration: 0.2 } }}
            className={`pointer-events-auto flex items-start gap-4 p-4 rounded-2xl glass-panel shadow-lg ${
              toast.type === 'error' ? 'border-error/30' : ''
            }`}
          >
            <div className="shrink-0 mt-0.5">{icons[toast.type]}</div>
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium text-graphite mb-1">{toast.message}</p>
            </div>
            <button
              onClick={() => removeToast(toast.id)}
              className="shrink-0 p-1 text-ash hover:text-graphite hover:bg-white/50 rounded-full transition-colors"
            >
              <X className="w-4 h-4" />
            </button>
          </motion.div>
        ))}
      </AnimatePresence>
    </div>
  );
}
