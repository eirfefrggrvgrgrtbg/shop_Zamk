import { useState } from 'react';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { useToast } from '../../contexts/ToastContext';
import { Star } from 'lucide-react';

interface ReviewModalProps {
  isOpen: boolean;
  onClose: () => void;
  orderId: string;
  orderItemId: string;
  productName: string;
  onSuccess: () => void;
}

export function ReviewModal({ isOpen, onClose, orderId, orderItemId, productName, onSuccess }: ReviewModalProps) {
  const [rating, setRating] = useState(5);
  const [title, setTitle] = useState('');
  const [comment, setComment] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const { showToast } = useToast();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      setIsSubmitting(true);
      const { createReview } = await import('@zamk/api-client/src/customer');
      await createReview(orderId, orderItemId, {
        rating,
        title,
        comment
      });
      showToast('Отзыв отправлен на модерацию', 'success');
      onSuccess();
      onClose();
    } catch (error: any) {
      showToast(error.message || 'Ошибка при добавлении отзыва', 'error');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Оставить отзыв">
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <p className="text-sm text-graphite-light dark:text-white/70 mb-2">
            Оцените товар: <span className="font-medium text-graphite dark:text-white">{productName}</span>
          </p>
        </div>
        
        <div className="flex gap-2">
          {[1, 2, 3, 4, 5].map((star) => (
            <button
              key={star}
              type="button"
              onClick={() => setRating(star)}
              className="focus:outline-none transition-transform hover:scale-110"
            >
              <Star className={`w-8 h-8 ${star <= rating ? 'fill-amber-400 text-amber-400' : 'fill-gray-200 text-gray-200 dark:fill-white/10 dark:text-white/10'}`} />
            </button>
          ))}
        </div>

        <div>
          <label className="block text-sm font-medium text-graphite dark:text-white mb-1">Заголовок отзыва (опционально)</label>
          <input 
            type="text"
            value={title} 
            onChange={(e) => setTitle(e.target.value)}
            placeholder="Краткое впечатление"
            className="w-full bg-ice dark:bg-white/5 border border-border-lighter dark:border-white/10 rounded-lg p-2.5 text-graphite dark:text-white placeholder-ash-light dark:placeholder-white/30 focus:outline-none focus:border-graphite dark:focus:border-white transition-colors"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-graphite dark:text-white mb-1">Комментарий (опционально)</label>
          <textarea 
            value={comment} 
            onChange={(e) => setComment(e.target.value)}
            placeholder="Что вам понравилось или не понравилось?"
            className="w-full bg-ice dark:bg-white/5 border border-border-lighter dark:border-white/10 rounded-lg p-3 text-graphite dark:text-white placeholder-ash-light dark:placeholder-white/30 focus:outline-none focus:border-graphite dark:focus:border-white transition-colors resize-none h-24"
          />
        </div>

        <div className="pt-4 flex gap-3">
          <Button type="button" variant="secondary" onClick={onClose} className="flex-1" disabled={isSubmitting}>Отмена</Button>
          <Button type="submit" variant="primary" className="flex-1" disabled={isSubmitting}>
            {isSubmitting ? 'Отправка...' : 'Отправить отзыв'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
