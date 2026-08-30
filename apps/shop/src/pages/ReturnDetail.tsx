import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { ArrowLeft, MessageSquare, AlertCircle, X } from 'lucide-react';
import { AccountLayout } from '../components/account/AccountLayout';
import { PRODUCT_PLACEHOLDER_IMAGE } from '../api/publicCatalog';
import { ReturnLifecycleProgress } from '../components/orders/ReturnLifecycleProgress';
import { ReturnLogistics } from '../components/orders/ReturnLogistics';
import { getCustomerReturn } from '@zamk/api-client/src/customer';
import {
  type CustomerReturnRecord,
  formatCustomerReturnStatus,
  formatReturnReason,
} from '@zamk/api-client/src/types';

export function ReturnDetail() {
  const { returnId } = useParams<{ returnId: string }>();
  const [returnData, setReturnData] = useState<CustomerReturnRecord | null>(null);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!returnId) return;

    const loadDetail = async () => {
      try {
        setIsLoading(true);
        setError('');
        const data = await getCustomerReturn(returnId);
        setReturnData(data);
      } catch (err: unknown) {
        console.error('Failed to load return details', err);
        setError('Не удалось загрузить информацию о возврате.');
      } finally {
        setIsLoading(false);
      }
    };

    loadDetail();
  }, [returnId]);

  if (isLoading) {
    return (
      <AccountLayout title="Детали возврата">
        <div className="py-20 flex justify-center">
          <div className="animate-spin w-8 h-8 border-2 border-black border-t-transparent rounded-full dark:border-white dark:border-t-transparent" />
        </div>
      </AccountLayout>
    );
  }

  if (error || !returnData) {
    return (
      <AccountLayout title="Детали возврата">
        <div className="p-8 rounded-[1.5rem] bg-white/50 dark:bg-white/5 backdrop-blur-xl border border-white/60 dark:border-white/10 text-center">
          <AlertCircle className="w-12 h-12 text-red-500 mx-auto mb-4" />
          <h2 className="text-xl font-medium text-graphite dark:text-white mb-2">
            {error || 'Возврат не найден'}
          </h2>
          <p className="text-sm text-ash dark:text-white/60 mb-6">
            Проверьте правильность адреса или вернитесь к списку ваших возвратов.
          </p>
          <Link
            to="/returns"
            className="inline-flex items-center gap-2 px-5 py-2.5 rounded-full bg-black text-white dark:bg-white dark:text-black text-xs font-medium hover:opacity-90 transition-opacity"
          >
            <ArrowLeft className="w-4 h-4" />
            Ко всем возвратам
          </Link>
        </div>
      </AccountLayout>
    );
  }

  const orderLabel = returnData.orderNumber
    ? `Возврат по заказу ${returnData.orderNumber}`
    : 'Возврат по заказу';

  const dateStr = new Date(returnData.createdAt).toLocaleDateString('ru-RU', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  });

  const shouldShowLogistics =
    returnData.status === 'approved' || !!returnData.shipment;

  const allEvidence = (returnData.items || []).flatMap((it) => it.evidence || []);

  return (
    <AccountLayout title="Детали возврата">
      <div className="space-y-4 md:space-y-5">
        {/* Navigation & Header */}
        <div>
          <Link
            to="/returns"
            className="inline-flex items-center gap-1.5 text-xs font-medium text-graphite/70 dark:text-white/70 hover:text-graphite dark:hover:text-white transition-colors mb-3"
          >
            <ArrowLeft className="w-4 h-4" />
            Назад к возвратам
          </Link>

          <div className="p-5 md:p-6 rounded-[1.25rem] bg-white/80 dark:bg-white/10 backdrop-blur-xl border border-graphite/10 dark:border-white/15 shadow-sm">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
              <div>
                <div className="flex items-center gap-3 mb-1.5">
                  <span className="text-xs font-medium px-3 py-0.5 rounded-full text-graphite dark:text-white bg-graphite/5 dark:bg-white/10 border border-graphite/10 dark:border-white/20">
                    {formatCustomerReturnStatus(returnData.status)}
                  </span>
                  <span className="text-xs text-graphite/70 dark:text-white/70 font-medium">
                    Оформлен {dateStr}
                  </span>
                </div>
                <h2 className="text-xl md:text-2xl font-serif text-graphite dark:text-white font-normal">
                  {orderLabel}
                </h2>
              </div>
            </div>
          </div>
        </div>

        {/* Progress Indicator */}
        <ReturnLifecycleProgress
          returnStatus={returnData.status}
          shipmentStatus={returnData.shipment?.status}
          adminComment={returnData.adminComment}
        />

        {/* Products Section */}
        <div className="p-5 md:p-6 rounded-[1.25rem] bg-white/80 dark:bg-white/10 backdrop-blur-xl border border-graphite/10 dark:border-white/15 shadow-sm">
          <h3 className="text-base font-semibold text-graphite dark:text-white mb-3 font-sans">
            Товары в возврате
          </h3>

          <div className="divide-y divide-graphite/5 dark:divide-white/10">
            {returnData.items && returnData.items.length > 0 ? (
              returnData.items.map((item, idx) => {
                const variantDetails = [item.variantSize, item.variantColor]
                  .filter(Boolean)
                  .join(' · ');

                return (
                  <div key={item.id || idx} className="py-3 first:pt-0 last:pb-0 flex gap-3.5 items-center">
                    <div className="w-14 h-18 bg-graphite/5 dark:bg-white/10 rounded-lg overflow-hidden flex-shrink-0 border border-graphite/10 dark:border-white/10">
                      <img
                        src={item.productImageUrl || PRODUCT_PLACEHOLDER_IMAGE}
                        alt={item.productTitle || 'Товар'}
                        className="w-full h-full object-cover object-center"
                      />
                    </div>
                    <div className="flex-1 py-0.5 flex flex-col justify-between">
                      <div>
                        <p className="text-sm font-medium text-graphite dark:text-white leading-snug">
                          {item.productTitle || 'Неизвестный товар'}
                        </p>
                        {variantDetails && (
                          <p className="text-xs text-graphite/70 dark:text-white/70 mt-0.5">
                            {variantDetails}
                          </p>
                        )}
                      </div>
                      <p className="text-xs font-medium text-graphite/70 dark:text-white/70 mt-0.5">
                        {item.quantity ?? 1} шт.
                      </p>
                    </div>
                  </div>
                );
              })
            ) : (
              <p className="text-sm text-graphite/70 dark:text-white/70 font-medium">
                Информация о товарах недоступна
              </p>
            )}
          </div>
        </div>

        {/* Claim Details: Reason & Comment */}
        <div className="p-5 md:p-6 rounded-[1.25rem] bg-white/80 dark:bg-white/10 backdrop-blur-xl border border-graphite/10 dark:border-white/15 shadow-sm space-y-3">
          <h3 className="text-base font-semibold text-graphite dark:text-white font-sans">
            Детали заявки
          </h3>

          <div className="space-y-2.5 text-sm">
            <div>
              <span className="text-xs text-graphite/60 dark:text-white/60 block mb-0.5">
                Причина возврата
              </span>
              <span className="font-medium text-graphite dark:text-white">
                {formatReturnReason(returnData.reason)}
              </span>
            </div>

            {returnData.comment && (
              <div>
                <span className="text-xs text-graphite/60 dark:text-white/60 block mb-0.5">
                  Комментарий покупателя
                </span>
                <p className="text-xs md:text-sm text-graphite/90 dark:text-white/90 leading-relaxed bg-graphite/5 dark:bg-white/5 border border-graphite/10 dark:border-white/10 p-3 rounded-xl">
                  {returnData.comment}
                </p>
              </div>
            )}
          </div>
        </div>

        {/* Evidence Section */}
        {allEvidence.length > 0 && (
          <div className="p-5 md:p-6 rounded-[1.25rem] bg-white/80 dark:bg-white/10 backdrop-blur-xl border border-graphite/10 dark:border-white/15 shadow-sm">
            <h3 className="text-base font-semibold text-graphite dark:text-white mb-3 font-sans">
              Фотографии к заявке
            </h3>
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-3">
              {allEvidence.map((ev) => (
                <button
                  key={ev.id}
                  type="button"
                  onClick={() => setPreviewUrl(ev.url)}
                  className="group relative aspect-square rounded-xl overflow-hidden border border-graphite/15 dark:border-white/15 bg-graphite/5 dark:bg-white/5 hover:opacity-90 hover:border-graphite/30 focus:outline-none focus:ring-2 focus:ring-black dark:focus:ring-white transition-all cursor-pointer"
                >
                  <img
                    src={ev.url}
                    alt="Фотография к возврату"
                    className="w-full h-full object-cover object-center group-hover:scale-105 transition-transform duration-300"
                  />
                </button>
              ))}
            </div>
          </div>
        )}

        {/* Logistics Section */}
        {shouldShowLogistics && (
          <ReturnLogistics
            returnId={returnData.id}
            initialShipment={returnData.shipment}
            onShipmentUpdated={(newShipment) => {
              setReturnData((prev) => (prev ? { ...prev, shipment: newShipment } : prev));
            }}
          />
        )}

        {/* Future-Proof Layout Placeholder: Messages/Chat */}
        <div className="p-5 md:p-6 rounded-[1.25rem] bg-white/80 dark:bg-white/10 backdrop-blur-xl border border-graphite/10 dark:border-white/15 shadow-sm">
          <div className="flex items-center gap-3 mb-1.5">
            <div className="w-7 h-7 rounded-full bg-graphite/5 dark:bg-white/10 flex items-center justify-center text-graphite/70 dark:text-white/70">
              <MessageSquare className="w-3.5 h-3.5" />
            </div>
            <h3 className="text-base font-semibold text-graphite dark:text-white font-sans">
              Переписка по возврату
            </h3>
          </div>
          <p className="text-xs text-graphite/70 dark:text-white/70 leading-relaxed pl-10">
            Здесь будет доступна переписка со службой поддержки и продавцом по данному возврату.
          </p>
        </div>
      </div>

      {/* Lightbox Preview */}
      {previewUrl && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm"
          onClick={() => setPreviewUrl(null)}
        >
          <div
            className="relative max-w-4xl max-h-[90vh] bg-transparent rounded-2xl overflow-hidden shadow-2xl flex flex-col items-center"
            onClick={(e) => e.stopPropagation()}
          >
            <button
              type="button"
              onClick={() => setPreviewUrl(null)}
              className="absolute top-3 right-3 z-10 w-9 h-9 rounded-full bg-black/60 hover:bg-black/80 text-white flex items-center justify-center transition-colors"
              aria-label="Закрыть"
            >
              <X className="w-5 h-5" />
            </button>
            <img
              src={previewUrl}
              alt="Увеличенное фото возврата"
              className="max-w-full max-h-[85vh] object-contain rounded-xl shadow-lg"
            />
          </div>
        </div>
      )}
    </AccountLayout>
  );
}
