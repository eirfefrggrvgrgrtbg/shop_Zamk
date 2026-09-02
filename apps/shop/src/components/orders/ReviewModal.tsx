import { useState, useEffect } from 'react';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { useToast } from '../../contexts/ToastContext';
import { Star } from 'lucide-react';
import { validateReviewForm, buildReviewPayload } from './reviewHelpers';

interface ReviewModalProps {
  isOpen: boolean;
  onClose: () => void;
  orderId?: string;
  orderItemId: string;
  productName: string;
  onSuccess: () => void;
}

export function ReviewModal({ isOpen, onClose, orderItemId, productName, onSuccess }: ReviewModalProps) {
  const [rating, setRating] = useState(5);
  const [title, setTitle] = useState('');
  const [comment, setComment] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const { showToast } = useToast();

  const resetForm = () => {
    setRating(5);
    setTitle('');
    setComment('');
  };

  // Reset form whenever modal opens or active target item changes
  useEffect(() => {
    if (isOpen) {
      resetForm();
    }
  }, [isOpen, orderItemId]);

  const handleClose = () => {
    resetForm();
    onClose();
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const validation = validateReviewForm(rating, comment);
    if (!validation.isValid) {
      showToast(validation.error || 'Пожалуйста, проверьте форму отзыва', 'error');
      return;
    }

    try {
      setIsSubmitting(true);
      const { createReview } = await import('@zamk/api-client/src/customer');
      const payload = buildReviewPayload(orderItemId, rating, title, comment);
      await createReview(payload);
      showToast('Отзыв отправлен на модерацию', 'success');
      resetForm();
      onSuccess();
      onClose();
    } catch (error: any) {
      const msg = error?.message || 'Ошибка при добавлении отзыва';
      showToast(msg, 'error');
      if (error?.status === 409 || error?.code === 'duplicate_review' || msg.includes('уже оставили отзыв')) {
        resetForm();
        onSuccess();
        onClose();
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Modal isOpen={isOpen} onClose={handleClose} title="Оставить отзыв">
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <p className="text-sm text-graphite-light dark:text-white/70 mb-2">
            Оцените товар: <span className="font-medium text-graphite dark:text-white">{productName}</span>
          </p>
        </div>
        
        <div className="flex gap-2" role="radiogroup" aria-label="Рейтинг товара">
          {[1, 2, 3, 4, 5].map((star) => (
            <button
              key={star}
              type="button"
              onClick={() => setRating(star)}
              className="focus:outline-none transition-transform hover:scale-110"
              aria-label={`Оценка ${star} из 5`}
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
          <div className="flex justify-between items-center mb-1">
            <label className="block text-sm font-medium text-graphite dark:text-white">Комментарий (опционально)</label>
            <span className={`text-xs ${comment.length > 1000 ? 'text-error font-medium' : 'text-ash dark:text-white/40'}`}>
              {comment.length} / 1000
            </span>
          </div>
          <textarea 
            value={comment} 
            onChange={(e) => setComment(e.target.value)}
            placeholder="Что вам понравилось или не понравилось?"
            className="w-full bg-ice dark:bg-white/5 border border-border-lighter dark:border-white/10 rounded-lg p-3 text-graphite dark:text-white placeholder-ash-light dark:placeholder-white/30 focus:outline-none focus:border-graphite dark:focus:border-white transition-colors resize-none h-24"
          />
        </div>

        <div className="pt-4 flex gap-3">
          <Button type="button" variant="secondary" onClick={handleClose} className="flex-1" disabled={isSubmitting}>Отмена</Button>
          <Button type="submit" variant="primary" className="flex-1" disabled={isSubmitting || comment.length > 1000}>
            {isSubmitting ? 'Отправка...' : 'Отправить отзыв'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
