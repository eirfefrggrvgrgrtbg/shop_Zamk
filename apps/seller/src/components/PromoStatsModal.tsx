import { useEffect, useState } from 'react';
import { getSellerPromotionStats } from '@zamk/api-client/src/seller';
import type { SellerPromoOutcome, SellerPromoStatsDetail, SellerPromotion } from '@zamk/api-client/src/types';
import { SellerModal } from './SellerSurface';

const rub = new Intl.NumberFormat('ru-RU', { style: 'currency', currency: 'RUB', maximumFractionDigits: 0 });
export const formatStatsMoney = (cents: number) => rub.format(cents / 100);

export const OUTCOME_LABELS: Record<SellerPromoOutcome, string> = {
  reserved: 'В резерве, не оплачен',
  paid: 'Оплачен',
  cancelled_after_payment: 'Отменён после оплаты',
  released: 'Не оплачен, код возвращён',
  expired: 'Не оплачен, резерв истёк',
};

/** Share of the global limit taken by paid + reserved usages (the same rule checkout applies). */
export function usageShare(consumed: number, reserved: number, limit: number | null): number | null {
  if (limit == null || limit <= 0) return null;
  return Math.min(100, Math.round(((consumed + reserved) / limit) * 100));
}

function Metric({ label, value, hint, testId }: { label: string; value: string; hint?: string; testId: string }) {
  return (
    <div className="rounded-lg border border-gray-200 p-3" title={hint}>
      <div className="text-xs text-gray-500">{label}</div>
      <div className="mt-1 text-lg font-semibold text-gray-900" data-testid={testId}>{value}</div>
    </div>
  );
}

export function PromoStatsModal({ promo, onClose }: { promo: SellerPromotion | null; onClose: () => void }) {
  const [loaded, setLoaded] = useState<{ id: string; detail?: SellerPromoStatsDetail; error?: string } | null>(null);

  useEffect(() => {
    if (!promo) return;
    let cancelled = false;
    getSellerPromotionStats(promo.id)
      .then((detail) => {
        if (!cancelled) setLoaded({ id: promo.id, detail });
      })
      .catch(() => {
        if (!cancelled) setLoaded({ id: promo.id, error: 'Не удалось загрузить статистику. Попробуйте ещё раз.' });
      });
    return () => {
      cancelled = true;
    };
  }, [promo]);

  const current = promo && loaded && loaded.id === promo.id ? loaded : null;
  const d = current?.detail;
  const share = promo && d ? usageShare(promo.consumedUsageCount, promo.reservedUsageCount, promo.globalUsageLimit) : null;

  return (
    <SellerModal isOpen={!!promo} onClose={onClose} title={promo ? `Статистика: ${promo.code}` : ''} data-testid="promo-stats-modal">
      {!current && <p className="text-sm text-gray-500" data-testid="promo-stats-loading">Загрузка…</p>}
      {current?.error && <p className="text-sm text-red-600" role="alert">{current.error}</p>}
      {promo && d && (
        <div className="space-y-5">
          <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
            <Metric testId="stat-paid" label="Оплаченные заказы" value={String(d.paidOrders)} hint="Заказы с кодом, оплаченные и не отменённые" />
            <Metric testId="stat-sales" label="Продажи с кодом" value={formatStatsMoney(d.salesCents)} hint="Оплаченные товары вашего магазина по заказам с кодом, до учёта возвратов" />
            <Metric testId="stat-discount" label="Скидки за ваш счёт" value={formatStatsMoney(d.sellerDiscountCents)} hint="Сумма скидок, которую профинансировали вы, по оплаченным заказам" />
            <Metric testId="stat-reserved" label="В резерве" value={String(d.reservedOrders)} hint="Заказы ожидают оплаты, код пока удерживается" />
            <Metric testId="stat-cancelled" label="Отменены после оплаты" value={String(d.cancelledAfterPaymentOrders)} />
            <Metric testId="stat-released" label="Не оплачены" value={String(d.releasedOrders)} hint="Оплата не прошла, заказ отменён или резерв истёк" />
            <Metric testId="stat-returned" label="Заказы с возвратом" value={String(d.returnedOrders)} hint="Оплаченные заказы, по которым оформлен завершённый возврат" />
            <Metric
              testId="stat-limit"
              label="Лимит использований"
              value={promo.globalUsageLimit != null ? `${promo.consumedUsageCount + promo.reservedUsageCount} из ${promo.globalUsageLimit}` : 'Без лимита'}
              hint="Оплаченные и удерживаемые использования"
            />
          </div>
          {share != null && (
            <div>
              <div className="h-2 rounded-full bg-gray-100 overflow-hidden">
                <div className="h-2 bg-gray-800" style={{ width: `${share}%` }} />
              </div>
              <div className="mt-1 text-xs text-gray-500">Использовано {share}% общего лимита</div>
            </div>
          )}

          <div>
            <h4 className="text-sm font-semibold text-gray-900 mb-2">Последние заказы с кодом</h4>
            {d.recentOrders.length === 0 ? (
              <p className="text-sm text-gray-500" data-testid="promo-stats-empty">Пока никто не использовал этот код.</p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-sm" data-testid="promo-stats-orders">
                  <thead className="text-xs text-gray-500 uppercase">
                    <tr>
                      <th className="py-2 pr-4">Заказ</th>
                      <th className="py-2 pr-4">Итог</th>
                      <th className="py-2 pr-4 text-right">Продажи</th>
                      <th className="py-2 text-right">Скидка</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100">
                    {d.recentOrders.map((o) => (
                      <tr key={o.orderId} data-testid={`promo-stats-order-${o.outcome}`}>
                        <td className="py-2 pr-4 text-gray-900">
                          {o.orderNumber || o.orderId.slice(0, 8)}
                          <div className="text-xs text-gray-400">{new Date(o.at).toLocaleDateString('ru-RU')}</div>
                        </td>
                        <td className="py-2 pr-4 text-gray-700">
                          {OUTCOME_LABELS[o.outcome]}
                          {o.returned && <span className="ml-1 text-xs text-amber-700">· есть возврат</span>}
                        </td>
                        <td className="py-2 pr-4 text-right">{o.outcome === 'paid' ? formatStatsMoney(o.salesCents) : '—'}</td>
                        <td className="py-2 text-right">{o.outcome === 'paid' ? formatStatsMoney(o.sellerDiscountCents) : '—'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          <p className="text-xs text-gray-500">
            Продажи и скидки считаются только по оплаченным заказам и показаны до учёта возвратов. Это не дополнительная выручка:
            неизвестно, что покупатель приобрёл бы без промокода. Данные о покупателях не показываются.
          </p>
        </div>
      )}
    </SellerModal>
  );
}
