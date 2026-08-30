import { useState, useRef } from 'react';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { useToast } from '../../contexts/ToastContext';
import { uploadReturnEvidence, deleteReturnEvidence } from '@zamk/api-client/src/customer';
import {
  RETURN_REASONS,
  REQUIRED_EVIDENCE_REASONS,
  SUCCESS_TOAST_MESSAGE,
  type EvidenceItem,
  canSubmitReturn,
  buildCreateReturnPayload,
  mapReturnErrorMessage,
} from './returnModalState';

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

export function ReturnModal({ isOpen, onClose, orderId, item, onSuccess }: ReturnModalProps) {
  const [reason, setReason] = useState(RETURN_REASONS[0].value);
  const [comment, setComment] = useState('');
  const [quantity, setQuantity] = useState(1);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isCleaningUp, setIsCleaningUp] = useState(false);
  const [evidence, setEvidence] = useState<EvidenceItem[]>([]);
  const [isUploading, setIsUploading] = useState(false);
  const [deletingIds, setDeletingIds] = useState<string[]>([]);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const { showToast } = useToast();

  const isRequired = REQUIRED_EVIDENCE_REASONS.includes(reason);

  const canSubmit = canSubmitReturn({
    reason,
    comment,
    evidence,
    isSubmitting: isSubmitting || isCleaningUp,
    isUploading,
  });

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!e.target.files || e.target.files.length === 0) return;
    const file = e.target.files[0];

    if (evidence.length >= 6) {
      showToast('Максимум 6 фотографий', 'error');
      return;
    }

    try {
      setIsUploading(true);
      const res = await uploadReturnEvidence(file);
      setEvidence((prev) => [...prev, res]);
    } catch (error: any) {
      showToast('Ошибка загрузки фото', 'error');
    } finally {
      setIsUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  const removeEvidence = async (id: string) => {
    if (deletingIds.includes(id) || isSubmitting || isCleaningUp) return;
    try {
      setDeletingIds((prev) => [...prev, id]);
      await deleteReturnEvidence(id);
      setEvidence((prev) => prev.filter((e) => e.id !== id));
    } catch (error: any) {
      showToast('Не удалось удалить фото. Попробуйте еще раз.', 'error');
    } finally {
      setDeletingIds((prev) => prev.filter((item) => item !== id));
    }
  };

  const handleCancelOrClose = async () => {
    if (isSubmitting || isCleaningUp) return;
    if (evidence.length > 0) {
      try {
        setIsCleaningUp(true);
        const results = await Promise.allSettled(
          evidence.map((ev) => deleteReturnEvidence(ev.id))
        );
        const failedIds: string[] = [];
        results.forEach((res, index) => {
          if (res.status === 'rejected') {
            failedIds.push(evidence[index].id);
          }
        });

        if (failedIds.length > 0) {
          showToast(
            'Некоторые загруженные фото не удалось удалить с сервера.',
            'error'
          );
          setEvidence((prev) => prev.filter((e) => failedIds.includes(e.id)));
        } else {
          setEvidence([]);
        }
      } catch (err) {
        showToast('Ошибка при очистке загруженных фото', 'error');
      } finally {
        setIsCleaningUp(false);
      }
    }
    onClose();
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmit) return;
    try {
      setIsSubmitting(true);
      const { createReturn } = await import('@zamk/api-client/src/customer');
      const payload = buildCreateReturnPayload(
        { orderItemId: item.orderItemId, quantity },
        reason,
        comment,
        evidence
      );
      await createReturn(orderId, payload as any);
      showToast(SUCCESS_TOAST_MESSAGE, 'success');
      // Reset local state to ensure no bound evidence IDs are reused if reopened
      setEvidence([]);
      setComment('');
      setQuantity(1);
      onSuccess();
      onClose();
    } catch (error: any) {
      showToast(mapReturnErrorMessage(error.message), 'error');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Modal isOpen={isOpen} onClose={handleCancelOrClose} title="Заявка на возврат">
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
              disabled={isSubmitting || isCleaningUp}
              className="w-full bg-ice dark:bg-white/5 border border-border-lighter dark:border-white/10 rounded-lg p-2.5 text-graphite dark:text-white focus:outline-none focus:border-graphite dark:focus:border-white transition-colors"
            >
              {Array.from({ length: item.maxQuantity }, (_, i) => i + 1).map((q) => (
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
            disabled={isSubmitting || isCleaningUp}
            className="w-full bg-ice dark:bg-white/5 border border-border-lighter dark:border-white/10 rounded-lg p-2.5 text-graphite dark:text-white focus:outline-none focus:border-graphite dark:focus:border-white transition-colors"
          >
            {RETURN_REASONS.map((r) => (
              <option key={r.value} value={r.value}>{r.label}</option>
            ))}
          </select>
        </div>

        <div>
          <label className="block text-sm font-medium text-graphite dark:text-white mb-1">Фотографии ({evidence.length}/6)</label>
          <p className="text-xs text-graphite-light dark:text-white/50 mb-2">
            {isRequired ? 'Обязательно загрузите от 2 до 6 фото товара.' : 'Вы можете загрузить до 6 фото товара.'}
          </p>
          <div className="flex gap-2 flex-wrap">
            {evidence.map((ev) => (
              <div key={ev.id} className="relative w-16 h-16 rounded-lg overflow-hidden border border-border-lighter">
                <img src={ev.url} className="object-cover w-full h-full" alt="" />
                <button
                  type="button"
                  onClick={() => removeEvidence(ev.id)}
                  disabled={deletingIds.includes(ev.id) || isSubmitting || isCleaningUp}
                  className="absolute top-1 right-1 bg-black/50 text-white rounded-full w-5 h-5 flex items-center justify-center text-xs"
                >
                  &times;
                </button>
              </div>
            ))}
            {evidence.length < 6 && (
              <button
                type="button"
                onClick={() => fileInputRef.current?.click()}
                className="w-16 h-16 rounded-lg border-2 border-dashed border-border-lighter dark:border-white/20 flex flex-col items-center justify-center text-graphite-light dark:text-white/50 hover:bg-black/5 transition-colors"
                disabled={isUploading || isSubmitting || isCleaningUp}
              >
                <span className="text-xl leading-none mb-1">+</span>
              </button>
            )}
          </div>
          <input
            type="file"
            ref={fileInputRef}
            onChange={handleFileUpload}
            accept="image/jpeg,image/png,image/webp"
            className="hidden"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-graphite dark:text-white mb-1">
            Опишите проблему
          </label>
          <p className="text-xs text-graphite-light dark:text-white/50 mb-2">
            Напишите, что именно не так с товаром.
          </p>
          <textarea
            value={comment}
            onChange={(e) => setComment(e.target.value)}
            disabled={isSubmitting || isCleaningUp}
            placeholder="Например: на правом рукаве разрыв около шва..."
            className="w-full bg-ice dark:bg-white/5 border border-border-lighter dark:border-white/10 rounded-lg p-3 text-graphite dark:text-white placeholder-ash-light dark:placeholder-white/30 focus:outline-none focus:border-graphite dark:focus:border-white transition-colors resize-none h-24"
          />
        </div>

        <div className="pt-4 flex gap-3">
          <Button type="button" variant="secondary" onClick={handleCancelOrClose} className="flex-1" disabled={isSubmitting || isCleaningUp}>Отмена</Button>
          <Button type="submit" variant="primary" className="flex-1" disabled={!canSubmit}>
            {isSubmitting ? 'Отправка...' : isCleaningUp ? 'Очистка...' : 'Отправить заявку'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
