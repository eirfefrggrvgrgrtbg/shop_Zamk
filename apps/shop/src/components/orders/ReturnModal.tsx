import { useState } from 'react';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { useToast } from '../../contexts/ToastContext';

interface ReturnItemInput {
  orderItemId: string;
  maxQuantity: number;
  productName: string;
}

interface ReturnModalProps {
  isOpen: boolean;
  onClose: () => void;
  orderId: string;
  item: ReturnItemInput;
  onSuccess: () => void;
}

const RETURN_REASONS = [
  { value: 'wrong_item', label: 'Получен не тот товар' },
  { value: 'damaged', label: 'Товар повреждён' },
  { value: 'defective', label: 'Товар неисправен' },
  { value: 'not_as_described', label: 'Не соответствует описанию' },
  { value: 'changed_mind', label: 'Передумал' },
  { value: 'other', label: 'Другое' },
];

export function ReturnModal({ isOpen, onClose, orderId, item, onSuccess }: ReturnModalProps) {
  const [reason, setReason] = useState(RETURN_REASONS[0].value);
  const [comment, setComment] = useState('');
  const [quantity, setQuantity] = useState(1);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const { showToast } = useToast();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      setIsSubmitting(true);
      const { createReturn } = await import('@zamk/api-client/src/customer');
      await createReturn(orderId, {
        reason,
        comment,
        items: [{
          orderItemId: item.orderItemId,
          quantity: quantity,
          reason: reason,
          condition: 'new' // simplified default condition
        }]
      });
      showToast('Запрос на возврат отправлен', 'success');
      onSuccess();
      onClose();
    } catch (error: any) {
      showToast(error.message || 'Ошибка при создании возврата', 'error');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Оформление возврата">
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <p className="text-sm text-graphite-light dark:text-white/70 mb-2">
            Товар: <span className="font-medium text-graphite dark:text-white">{item.productName}</span>
          </p>
        </div>
        
        {item.maxQuantity > 1 && (
          <div>
            <label className="block text-sm font-medium text-graphite dark:text-white mb-1">Количество</label>
            <select 
              value={quantity} 
              onChange={(e) => setQuantity(Number(e.target.value))}
              className="w-full bg-ice dark:bg-white/5 border border-border-lighter dark:border-white/10 rounded-lg p-2.5 text-graphite dark:text-white focus:outline-none focus:border-graphite dark:focus:border-white transition-colors"
            >
              {Array.from({ length: item.maxQuantity }, (_, i) => i + 1).map(q => (
                <option key={q} value={q}>{q}</option>
              ))}
            </select>
          </div>
        )}

        <div>
          <label className="block text-sm font-medium text-graphite dark:text-white mb-1">Причина возврата</label>
          <select 
            value={reason} 
            onChange={(e) => setReason(e.target.value)}
            className="w-full bg-ice dark:bg-white/5 border border-border-lighter dark:border-white/10 rounded-lg p-2.5 text-graphite dark:text-white focus:outline-none focus:border-graphite dark:focus:border-white transition-colors"
          >
            {RETURN_REASONS.map(r => (
              <option key={r.value} value={r.value}>{r.label}</option>
            ))}
          </select>
        </div>

        <div>
          <label className="block text-sm font-medium text-graphite dark:text-white mb-1">Комментарий (опционально)</label>
          <textarea 
            value={comment} 
            onChange={(e) => setComment(e.target.value)}
            placeholder="Опишите проблему подробнее..."
            className="w-full bg-ice dark:bg-white/5 border border-border-lighter dark:border-white/10 rounded-lg p-3 text-graphite dark:text-white placeholder-ash-light dark:placeholder-white/30 focus:outline-none focus:border-graphite dark:focus:border-white transition-colors resize-none h-24"
          />
        </div>

        <div className="pt-4 flex gap-3">
          <Button type="button" variant="secondary" onClick={onClose} className="flex-1" disabled={isSubmitting}>Отмена</Button>
          <Button type="submit" variant="primary" className="flex-1" disabled={isSubmitting}>
            {isSubmitting ? 'Отправка...' : 'Отправить запрос'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
