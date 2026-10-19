import { Share2 } from 'lucide-react';
import { useToast } from '../../contexts/ToastContext';

interface ShareProductButtonProps {
  title: string;
}

async function copyToClipboard(text: string): Promise<void> {
  if (!navigator.clipboard?.writeText) {
    throw new Error('Clipboard API unavailable');
  }
  await navigator.clipboard.writeText(text);
}

export function ShareProductButton({ title }: ShareProductButtonProps) {
  const { showToast } = useToast();

  const handleShare = async () => {
    const url = window.location.href;

    if (typeof navigator.share === 'function') {
      try {
        await navigator.share({ title, url });
      } catch (e) {
        // Отмена системного меню — не ошибка.
        if (e instanceof DOMException && e.name === 'AbortError') return;
        try {
          await copyToClipboard(url);
          showToast('Ссылка скопирована');
        } catch {
          showToast('Не удалось поделиться ссылкой', 'error');
        }
      }
      return;
    }

    try {
      await copyToClipboard(url);
      showToast('Ссылка скопирована');
    } catch {
      showToast('Не удалось скопировать ссылку', 'error');
    }
  };

  return (
    <button
      type="button"
      onClick={handleShare}
      aria-label="Поделиться товаром"
      className="inline-flex items-center gap-1.5 text-xs sm:text-sm text-ash hover:text-graphite dark:hover:text-white transition-colors rounded-md px-2 py-1 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-graphite/40 dark:focus-visible:ring-white/40"
    >
      <Share2 className="w-4 h-4" aria-hidden="true" />
      <span>Поделиться</span>
    </button>
  );
}
