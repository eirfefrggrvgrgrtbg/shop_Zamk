import React, { useState, useEffect } from 'react';
import { X } from 'lucide-react';
import type { AdminAuctionLot } from '@zamk/api-client/src/types';
import { createAdminLot, updateAdminLot } from '@zamk/api-client/src/admin';
import { HelpTooltip } from '../HelpTooltip';

interface LotModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  auctionId: string;
  lot?: AdminAuctionLot | null;
}

export function LotModal({ isOpen, onClose, onSuccess, auctionId, lot }: LotModalProps) {
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [startPriceRubles, setStartPriceRubles] = useState('100');
  const [bidStepRubles, setBidStepRubles] = useState('100');
  const [canRelaunch, setCanRelaunch] = useState(false);
  const [canMoveToDirectSale, setCanMoveToDirectSale] = useState(false);
  const [directSalePriceRubles, setDirectSalePriceRubles] = useState('');
  const [adminNote, setAdminNote] = useState('');

  useEffect(() => {
    if (lot) {
      setTitle(lot.title);
      setDescription(lot.description || '');
      setStartPriceRubles((lot.startPriceCents / 100).toString());
      setBidStepRubles((lot.bidStepCents / 100).toString());
      setCanRelaunch(lot.canRelaunch || false);
      setCanMoveToDirectSale(lot.canMoveToDirectSale || false);
      setDirectSalePriceRubles(lot.directSalePriceCents ? (lot.directSalePriceCents / 100).toString() : '');
      setAdminNote(lot.adminNote || '');
    } else {
      setTitle('');
      setDescription('');
      setStartPriceRubles('100');
      setBidStepRubles('100');
      setCanRelaunch(false);
      setCanMoveToDirectSale(false);
      setDirectSalePriceRubles('');
      setAdminNote('');
    }
  }, [lot, isOpen]);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    const startPriceCents = Math.round(parseFloat(startPriceRubles) * 100);
    const bidStepCents = Math.round(parseFloat(bidStepRubles) * 100);
    const directSalePriceCents = directSalePriceRubles ? Math.round(parseFloat(directSalePriceRubles) * 100) : null;

    if (isNaN(startPriceCents) || startPriceCents <= 0) {
      setError('Начальная цена должна быть больше нуля.');
      return;
    }
    if (isNaN(bidStepCents) || bidStepCents <= 0) {
      setError('Шаг ставки должен быть больше нуля.');
      return;
    }

    try {
      setIsSubmitting(true);
      const data: Partial<AdminAuctionLot> = {
        title,
        description,
        startPriceCents,
        bidStepCents,
        canRelaunch,
        canMoveToDirectSale,
        directSalePriceCents: directSalePriceCents as any,
        adminNote,
      };

      if (lot) {
        await updateAdminLot(lot.id, data);
      } else {
        await createAdminLot(auctionId, data);
      }

      onSuccess();
    } catch (err) {
      setError('Не удалось сохранить.');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto">
      <div className="flex items-center justify-center min-h-screen px-4 pt-4 pb-20 text-center sm:block sm:p-0">
        <div className="fixed inset-0 transition-opacity" aria-hidden="true" onClick={onClose}>
          <div className="absolute inset-0 bg-gray-500 opacity-75"></div>
        </div>

        <span className="hidden sm:inline-block sm:align-middle sm:h-screen" aria-hidden="true">&#8203;</span>

        <div className="inline-block align-bottom bg-white rounded-lg text-left overflow-hidden shadow-xl transform transition-all sm:my-8 sm:align-middle sm:max-w-lg sm:w-full">
          <div className="bg-white px-4 pt-5 pb-4 sm:p-6 sm:pb-4">
            <div className="flex justify-between items-center mb-4 border-b pb-2">
              <h3 className="text-lg leading-6 font-medium text-gray-900">
                {lot ? 'Редактировать лот' : 'Добавить лот'}
              </h3>
              <button onClick={onClose} className="text-gray-400 hover:text-gray-500">
                <X className="h-6 w-6" />
              </button>
            </div>

            {error && (
              <div className="mb-4 bg-red-50 text-red-700 p-3 rounded-md text-sm">
                {error}
              </div>
            )}

            <form id="lot-form" onSubmit={handleSubmit} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700">Название <span className="text-red-500">*</span></label>
                <input
                  type="text"
                  required
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700">Описание</label>
                <textarea
                  rows={3}
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
                />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700">Старт. цена (₽) <span className="text-red-500">*</span></label>
                  <input
                    type="number"
                    min="1"
                    required
                    value={startPriceRubles}
                    onChange={(e) => setStartPriceRubles(e.target.value)}
                    className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700">Шаг ставки (₽) <span className="text-red-500">*</span></label>
                  <input
                    type="number"
                    min="1"
                    required
                    value={bidStepRubles}
                    onChange={(e) => setBidStepRubles(e.target.value)}
                    className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
                  />
                </div>
              </div>

              <div className="space-y-2 mt-4">
                <div className="flex items-center">
                  <input
                    id="canRelaunch"
                    type="checkbox"
                    checked={canRelaunch}
                    onChange={(e) => setCanRelaunch(e.target.checked)}
                    className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
                  />
                  <label htmlFor="canRelaunch" className="ml-2 flex items-center text-sm text-gray-900">
                    Разрешить перезапуск
                    <HelpTooltip content="Если лот не продан, его можно будет запустить в другом аукционе." />
                  </label>
                </div>
                
                <div className="flex items-center">
                  <input
                    id="canMoveToDirectSale"
                    type="checkbox"
                    checked={canMoveToDirectSale}
                    onChange={(e) => setCanMoveToDirectSale(e.target.checked)}
                    className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
                  />
                  <label htmlFor="canMoveToDirectSale" className="ml-2 flex items-center text-sm text-gray-900">
                    Разрешить прямую продажу
                    <HelpTooltip content="Можно перевести лот в обычный магазин для прямой покупки." />
                  </label>
                </div>
              </div>

              {canMoveToDirectSale && (
                <div>
                  <label className="block text-sm font-medium text-gray-700">Цена прямой продажи (₽)</label>
                  <input
                    type="number"
                    min="1"
                    value={directSalePriceRubles}
                    onChange={(e) => setDirectSalePriceRubles(e.target.value)}
                    className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
                  />
                </div>
              )}

              <div>
                <label className="block text-sm font-medium text-gray-700">Внутренняя заметка админа</label>
                <textarea
                  rows={2}
                  value={adminNote}
                  onChange={(e) => setAdminNote(e.target.value)}
                  className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
                  placeholder="Не видна пользователям..."
                />
              </div>

            </form>
          </div>
          <div className="bg-gray-50 px-4 py-3 sm:px-6 sm:flex sm:flex-row-reverse">
            <button
              type="submit"
              form="lot-form"
              disabled={isSubmitting}
              className="w-full inline-flex justify-center rounded-md border border-transparent shadow-sm px-4 py-2 bg-indigo-600 text-base font-medium text-white hover:bg-indigo-700 focus:outline-none sm:ml-3 sm:w-auto sm:text-sm disabled:opacity-50"
            >
              {isSubmitting ? 'Сохраняем…' : 'Сохранить'}
            </button>
            <button
              type="button"
              onClick={onClose}
              className="mt-3 w-full inline-flex justify-center rounded-md border border-gray-300 shadow-sm px-4 py-2 bg-white text-base font-medium text-gray-700 hover:bg-gray-50 focus:outline-none sm:mt-0 sm:ml-3 sm:w-auto sm:text-sm"
            >
              Отмена
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
