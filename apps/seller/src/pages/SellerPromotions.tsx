import React, { useEffect, useState, useCallback } from 'react';
import {
  getSellerPromotions,
  createSellerPromotion,
  updateSellerPromotion,
} from '@zamk/api-client/src/seller';
import type {
  SellerPromotion,
  SellerPromoDiscountType,
  CreateSellerPromotionRequest,
  UpdateSellerPromotionRequest,
} from '@zamk/api-client/src/types';
import {
  Tag,
  Plus,
  AlertCircle,
  Pause,
  Play,
  Edit2,
  CheckCircle2,
  Clock,
  Ban,
  Archive,
} from 'lucide-react';
import { SellerPageFrame, SellerPageHeader } from '../components/SellerPageFrame';
import { SellerSurface, SellerModal } from '../components/SellerSurface';
import { SellerDateTimePicker } from '../components/SellerDateTimePicker';

const currencyFormatter = new Intl.NumberFormat('ru-RU', {
  style: 'currency',
  currency: 'RUB',
  maximumFractionDigits: 0,
});

const STATUS_CONFIG: Record<
  string,
  { label: string; badgeClass: string; icon: React.ComponentType<{ className?: string }> }
> = {
  active: {
    label: 'Активен',
    badgeClass: 'bg-emerald-50 text-emerald-700 border-emerald-200',
    icon: CheckCircle2,
  },
  scheduled: {
    label: 'Запланирован',
    badgeClass: 'bg-blue-50 text-blue-700 border-blue-200',
    icon: Clock,
  },
  paused: {
    label: 'На паузе',
    badgeClass: 'bg-amber-50 text-amber-700 border-amber-200',
    icon: Pause,
  },
  expired: {
    label: 'Истёк',
    badgeClass: 'bg-gray-100 text-gray-700 border-gray-200',
    icon: Archive,
  },
  exhausted: {
    label: 'Исчерпан',
    badgeClass: 'bg-purple-50 text-purple-700 border-purple-200',
    icon: Ban,
  },
};

export function SellerPromotions() {
  const [promotions, setPromotions] = useState<SellerPromotion[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  // Drawer states
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [editPromo, setEditPromo] = useState<SellerPromotion | null>(null);

  // Create Form State
  const [createCode, setCreateCode] = useState('');
  const [createDiscountType, setCreateDiscountType] = useState<SellerPromoDiscountType>('percent');
  const [createDiscountPercent, setCreateDiscountPercent] = useState('');
  const [createDiscountFixedRub, setCreateDiscountFixedRub] = useState('');
  const [createMinOrderSubtotalRub, setCreateMinOrderSubtotalRub] = useState('');
  const [createFirstPaidOnly, setCreateFirstPaidOnly] = useState(false);
  const [createStartsAt, setCreateStartsAt] = useState('');
  const [createEndsAt, setCreateEndsAt] = useState('');
  const [createGlobalLimit, setCreateGlobalLimit] = useState('');
  const [createPerCustomerLimit, setCreatePerCustomerLimit] = useState('1');
  const [createSubmitting, setCreateSubmitting] = useState(false);
  const [createError, setCreateError] = useState('');
  const [createDateError, setCreateDateError] = useState('');

  // Edit Form State (Mutable operational fields only)
  const [editIsActive, setEditIsActive] = useState(true);
  const [editStartsAt, setEditStartsAt] = useState('');
  const [editEndsAt, setEditEndsAt] = useState('');
  const [editGlobalLimit, setEditGlobalLimit] = useState('');
  const [editPerCustomerLimit, setEditPerCustomerLimit] = useState('1');
  const [editSubmitting, setEditSubmitting] = useState(false);
  const [editError, setEditError] = useState('');
  const [editDateError, setEditDateError] = useState('');

  useEffect(() => {
    if (createStartsAt && createEndsAt) {
      if (new Date(createEndsAt) <= new Date(createStartsAt)) {
        setCreateDateError('Дата окончания должна быть позже даты начала.');
        return;
      }
    }
    setCreateDateError('');
  }, [createStartsAt, createEndsAt]);

  useEffect(() => {
    if (editStartsAt && editEndsAt) {
      if (new Date(editEndsAt) <= new Date(editStartsAt)) {
        setEditDateError('Дата окончания должна быть позже даты начала.');
        return;
      }
    }
    setEditDateError('');
  }, [editStartsAt, editEndsAt]);

  const fetchPromotions = useCallback(async () => {
    try {
      const data = await getSellerPromotions();
      setPromotions(data.items || []);
      setError('');
    } catch (err: any) {
      if (err.status === 403) {
        setError('Недостаточно прав для управления промокодами.');
      } else {
        setError('Не удалось загрузить список промокодов.');
      }
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchPromotions();
  }, [fetchPromotions]);

  const resetCreateForm = () => {
    setCreateCode('');
    setCreateDiscountType('percent');
    setCreateDiscountPercent('');
    setCreateDiscountFixedRub('');
    setCreateMinOrderSubtotalRub('');
    setCreateFirstPaidOnly(false);
    setCreateStartsAt('');
    setCreateEndsAt('');
    setCreateGlobalLimit('');
    setCreatePerCustomerLimit('1');
    setCreateError('');
    setCreateDateError('');
  };

  const openEditDrawer = (promo: SellerPromotion) => {
    setEditPromo(promo);
    setEditIsActive(promo.isActive);
    setEditStartsAt(promo.startsAt ? promo.startsAt.substring(0, 16) : '');
    setEditEndsAt(promo.endsAt ? promo.endsAt.substring(0, 16) : '');
    setEditGlobalLimit(promo.globalUsageLimit != null ? String(promo.globalUsageLimit) : '');
    setEditPerCustomerLimit(String(promo.perCustomerUsageLimit || 1));
    setEditError('');
    setEditDateError('');
  };

  const handleCreateSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreateSubmitting(true);
    setCreateError('');

    try {
      const codeClean = createCode.trim().toUpperCase();
      if (!codeClean) {
        throw new Error('Укажите код промокода.');
      }

      const req: CreateSellerPromotionRequest = {
        code: codeClean,
        discountType: createDiscountType,
      };

      if (createDiscountType === 'percent') {
        const pct = parseFloat(createDiscountPercent);
        if (isNaN(pct) || pct <= 0 || pct > 100) {
          throw new Error('Скидка должна быть от 1% до 100%.');
        }
        req.discountValueBps = Math.round(pct * 100);
      } else {
        const fixedRub = parseFloat(createDiscountFixedRub);
        if (isNaN(fixedRub) || fixedRub <= 0) {
          throw new Error('Укажите фиксированную сумму скидки в рублях.');
        }
        req.discountValueFixedCents = Math.round(fixedRub * 100);
      }

      if (createMinOrderSubtotalRub) {
        const minRub = parseFloat(createMinOrderSubtotalRub);
        if (!isNaN(minRub) && minRub > 0) {
          req.minOrderSubtotalCents = Math.round(minRub * 100);
        }
      }

      req.firstPaidOrderOnly = createFirstPaidOnly;

      if (createStartsAt && createEndsAt) {
        if (new Date(createEndsAt) <= new Date(createStartsAt)) {
          throw new Error('Дата окончания должна быть позже даты начала.');
        }
      }

      if (createStartsAt) {
        req.startsAt = new Date(createStartsAt).toISOString();
      }
      if (createEndsAt) {
        req.endsAt = new Date(createEndsAt).toISOString();
      }

      if (createGlobalLimit) {
        const gl = parseInt(createGlobalLimit, 10);
        if (isNaN(gl) || gl <= 0) {
          throw new Error('Общий лимит должен быть положительным числом.');
        }
        req.globalUsageLimit = gl;
      }

      if (createPerCustomerLimit) {
        const pcl = parseInt(createPerCustomerLimit, 10);
        if (isNaN(pcl) || pcl <= 0) {
          throw new Error('Лимит на клиента должен быть положительным числом.');
        }
        req.perCustomerUsageLimit = pcl;
      }

      await createSellerPromotion(req);
      setIsCreateOpen(false);
      resetCreateForm();
      await fetchPromotions();
    } catch (err: any) {
      if (err.data?.code === 'promo_code_duplicate' || err.code === 'promo_code_duplicate') {
        setCreateError('Промокод с таким кодом уже существует. Выберите другой код.');
      } else {
        setCreateError(err.message || err.data?.message || 'Не удалось создать промокод.');
      }
    } finally {
      setCreateSubmitting(false);
    }
  };

  const handleEditSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editPromo) return;
    setEditSubmitting(true);
    setEditError('');

    try {
      const req: UpdateSellerPromotionRequest = {
        isActive: editIsActive,
      };

      if (editStartsAt && editEndsAt) {
        if (new Date(editEndsAt) <= new Date(editStartsAt)) {
          throw new Error('Дата окончания должна быть позже даты начала.');
        }
      }

      if (editStartsAt) {
        req.startsAt = new Date(editStartsAt).toISOString();
      } else {
        req.startsAt = null;
      }

      if (editEndsAt) {
        req.endsAt = new Date(editEndsAt).toISOString();
      } else {
        req.endsAt = null;
      }

      if (editGlobalLimit) {
        const gl = parseInt(editGlobalLimit, 10);
        if (isNaN(gl) || gl <= 0) {
          throw new Error('Общий лимит должен быть положительным числом.');
        }
        req.globalUsageLimit = gl;
      } else {
        req.globalUsageLimit = null;
      }

      if (editPerCustomerLimit) {
        const pcl = parseInt(editPerCustomerLimit, 10);
        if (isNaN(pcl) || pcl <= 0) {
          throw new Error('Лимит на клиента должен быть положительным числом.');
        }
        req.perCustomerUsageLimit = pcl;
      }

      await updateSellerPromotion(editPromo.id, req);
      setEditPromo(null);
      await fetchPromotions();
    } catch (err: any) {
      if (err.data?.code === 'global_limit_below_usage' || err.code === 'global_limit_below_usage') {
        setEditError('Общий лимит не может быть меньше уже зафиксированного использования.');
      } else if (
        err.data?.code === 'customer_limit_below_usage' ||
        err.code === 'customer_limit_below_usage'
      ) {
        setEditError('Лимит на клиента не может быть меньше уже зафиксированного использования покупателя.');
      } else if (err.data?.code === 'invalid_dates' || err.code === 'invalid_dates') {
        setEditError('Дата окончания должна быть позже даты начала.');
      } else {
        setEditError(err.message || err.data?.message || 'Не удалось обновить промокод.');
      }
    } finally {
      setEditSubmitting(false);
    }
  };

  const handleQuickToggleActive = async (promo: SellerPromotion) => {
    try {
      await updateSellerPromotion(promo.id, { isActive: !promo.isActive });
      await fetchPromotions();
    } catch (err: any) {
      setError(err.message || 'Не удалось изменить статус промокода.');
    }
  };

  if (isLoading) {
    return (
      <SellerPageFrame variant="summary">
        <SellerSurface className="py-20 flex flex-col justify-center items-center">
          <div className="animate-spin rounded-full h-10 w-10 border-b-2 border-gray-900 mb-4"></div>
          <div className="text-sm text-gray-500">Загружаем промокоды...</div>
        </SellerSurface>
      </SellerPageFrame>
    );
  }

  if (error && promotions.length === 0) {
    return (
      <SellerPageFrame variant="summary">
        <div className="rounded-lg border border-red-200 bg-red-50 p-6 flex items-center gap-3 text-red-600">
          <AlertCircle className="w-5 h-5 shrink-0" />
          <span>{error}</span>
        </div>
      </SellerPageFrame>
    );
  }

  return (
    <SellerPageFrame variant="summary">
      <SellerPageHeader
        eyebrow="Маркетинг"
        title="Промокоды"
        description="Собственные промокоды продавца. Скидка финансируется вами и применяется ко всем вашим товарам."
        action={
          <button
            type="button"
            onClick={() => {
              resetCreateForm();
              setIsCreateOpen(true);
            }}
            data-testid="create-promo-button"
            className="inline-flex items-center gap-2 px-4 py-2 bg-black text-white rounded-lg text-sm font-medium hover:bg-gray-800 transition-colors shadow-xs cursor-pointer"
          >
            <Plus className="w-4 h-4" />
            Создать промокод
          </button>
        }
      />

      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-4 flex items-center gap-3 text-red-600 text-sm">
          <AlertCircle className="w-4 h-4 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* Promotions List / Table */}
      {promotions.length === 0 ? (
        <SellerSurface className="py-16 px-6 text-center">
          <div className="inline-flex p-4 rounded-full bg-gray-100 text-gray-500 mb-4">
            <Tag className="w-8 h-8" />
          </div>
          <h3 className="text-lg font-medium text-gray-900 mb-1">У вас пока нет промокодов</h3>
          <p className="text-sm text-gray-500 max-w-md mx-auto mb-6">
            Создайте свой первый промокод, чтобы привлечь покупателей и увеличить продажи ваших товаров.
          </p>
          <button
            type="button"
            onClick={() => {
              resetCreateForm();
              setIsCreateOpen(true);
            }}
            data-testid="create-first-promo-button"
            className="inline-flex items-center gap-2 px-4 py-2 bg-black text-white rounded-lg text-sm font-medium hover:bg-gray-800 transition-colors shadow-xs cursor-pointer"
          >
            <Plus className="w-4 h-4" />
            Создать первый промокод
          </button>
        </SellerSurface>
      ) : (
        <SellerSurface className="overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm" data-testid="promotions-table">
              <thead className="bg-gray-50 border-b border-gray-200 text-xs font-semibold text-gray-500 uppercase tracking-wider">
                <tr>
                  <th className="px-6 py-3">Промокод</th>
                  <th className="px-6 py-3">Скидка</th>
                  <th className="px-6 py-3">Мин. заказ</th>
                  <th className="px-6 py-3">Использовано</th>
                  <th className="px-6 py-3">Лимиты</th>
                  <th className="px-6 py-3">Период действия</th>
                  <th className="px-6 py-3">Статус</th>
                  <th className="px-6 py-3 text-right">Действия</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {promotions.map((p) => {
                  const statusConf = STATUS_CONFIG[p.status] || STATUS_CONFIG.active;
                  const StatusIcon = statusConf.icon;

                  let discountText = '';
                  if (p.discountType === 'percent') {
                    discountText = `${p.discountValueBps / 100}%`;
                  } else {
                    discountText = currencyFormatter.format(p.discountValueFixedCents / 100);
                  }

                  const minOrderText =
                    p.minOrderSubtotalCents > 0
                      ? currencyFormatter.format(p.minOrderSubtotalCents / 100)
                      : 'Без ограничений';

                  const formatWindowDate = (dt: string | null) => {
                    if (!dt) return null;
                    return new Date(dt).toLocaleDateString('ru-RU', {
                      day: 'numeric',
                      month: 'short',
                      year: 'numeric',
                    });
                  };

                  const startsFormatted = formatWindowDate(p.startsAt);
                  const endsFormatted = formatWindowDate(p.endsAt);

                  return (
                    <tr
                      key={p.id}
                      data-testid={`promo-row-${p.code}`}
                      className="hover:bg-gray-50/50 transition-colors"
                    >
                      <td className="px-6 py-4">
                        <div className="flex items-center gap-2">
                          <span className="font-mono font-bold text-gray-900 tracking-wide text-base">
                            {p.code}
                          </span>
                          {p.firstPaidOrderOnly && (
                            <span
                              title="Только для первого заказа покупателя"
                              className="px-1.5 py-0.5 text-[10px] font-semibold bg-blue-50 text-blue-700 border border-blue-200 rounded"
                            >
                              1-й заказ
                            </span>
                          )}
                        </div>
                      </td>
                      <td className="px-6 py-4 font-semibold text-gray-900">{discountText}</td>
                      <td className="px-6 py-4 text-gray-600">{minOrderText}</td>
                      <td className="px-6 py-4 text-gray-600">
                        <span className="font-medium text-gray-900">{p.consumedUsageCount}</span>
                        {p.reservedUsageCount > 0 && (
                          <span
                            title="В оформлении / резерве"
                            className="text-xs text-amber-600 ml-1.5"
                          >
                            (+{p.reservedUsageCount} в резерве)
                          </span>
                        )}
                      </td>
                      <td className="px-6 py-4 text-xs text-gray-500 space-y-0.5">
                        <div>
                          Общий:{' '}
                          <span className="font-medium text-gray-900">
                            {p.globalUsageLimit != null ? p.globalUsageLimit : '∞'}
                          </span>
                        </div>
                        <div>
                          На клиента:{' '}
                          <span className="font-medium text-gray-900">
                            {p.perCustomerUsageLimit || 1}
                          </span>
                        </div>
                      </td>
                      <td className="px-6 py-4 text-xs text-gray-500">
                        {startsFormatted || endsFormatted ? (
                          <div className="space-y-0.5">
                            {startsFormatted && <div>С {startsFormatted}</div>}
                            {endsFormatted && <div>По {endsFormatted}</div>}
                          </div>
                        ) : (
                          <span className="text-gray-400">Бессрочно</span>
                        )}
                      </td>
                      <td className="px-6 py-4">
                        <span
                          data-testid={`promo-status-badge-${p.code}`}
                          className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium border ${statusConf.badgeClass}`}
                        >
                          <StatusIcon className="w-3.5 h-3.5" />
                          {statusConf.label}
                        </span>
                      </td>
                      <td className="px-6 py-4 text-right">
                        <div className="flex items-center justify-end gap-2">
                          <button
                            type="button"
                            onClick={() => handleQuickToggleActive(p)}
                            title={p.isActive ? 'Поставить на паузу' : 'Возобновить'}
                            data-testid={`promo-toggle-active-${p.code}`}
                            className={`p-1.5 rounded-lg border text-xs font-medium transition-colors cursor-pointer ${
                              p.isActive
                                ? 'border-amber-200 text-amber-700 hover:bg-amber-50'
                                : 'border-emerald-200 text-emerald-700 hover:bg-emerald-50'
                            }`}
                          >
                            {p.isActive ? (
                              <Pause className="w-3.5 h-3.5" />
                            ) : (
                              <Play className="w-3.5 h-3.5" />
                            )}
                          </button>
                          <button
                            type="button"
                            onClick={() => openEditDrawer(p)}
                            title="Редактировать параметры"
                            data-testid={`promo-edit-button-${p.code}`}
                            className="p-1.5 rounded-lg border border-gray-200 text-gray-700 hover:bg-gray-100 transition-colors cursor-pointer"
                          >
                            <Edit2 className="w-3.5 h-3.5" />
                          </button>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </SellerSurface>
      )}

      {/* CREATE MODAL */}
      <SellerModal
        isOpen={isCreateOpen}
        onClose={() => setIsCreateOpen(false)}
        title="Создание промокода"
        data-testid="create-promo-modal"
      >
        <form onSubmit={handleCreateSubmit} className="space-y-6">
          {createError && (
            <div
              data-testid="create-promo-error"
              className="p-3 bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg flex items-start gap-2"
            >
              <AlertCircle className="w-4 h-4 mt-0.5 shrink-0" />
              <span>{createError}</span>
            </div>
          )}

          <div>
            <label className="block text-xs font-semibold text-gray-700 uppercase tracking-wider mb-1">
              Код промокода <span className="text-red-500">*</span>
            </label>
            <input
              type="text"
              required
              data-testid="input-promo-code"
              value={createCode}
              onChange={(e) => setCreateCode(e.target.value.toUpperCase())}
              placeholder="НАПРИМЕР, SPRING20"
              className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm font-mono uppercase tracking-wider focus:outline-none focus:ring-2 focus:ring-black"
            />
            <p className="text-xs text-gray-400 mt-1">
              После создания код изменить нельзя. Нечувствителен к регистру.
            </p>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold text-gray-700 uppercase tracking-wider mb-2">
                Тип скидки <span className="text-red-500">*</span>
              </label>
              <div className="grid grid-cols-2 gap-2">
                <button
                  type="button"
                  data-testid="radio-discount-percent"
                  onClick={() => setCreateDiscountType('percent')}
                  className={`py-2 px-3 border rounded-lg text-sm font-medium transition-colors cursor-pointer ${
                    createDiscountType === 'percent'
                      ? 'border-black bg-black text-white'
                      : 'border-gray-200 text-gray-700 hover:bg-gray-50'
                  }`}
                >
                  Процентная (%)
                </button>
                <button
                  type="button"
                  data-testid="radio-discount-fixed"
                  onClick={() => setCreateDiscountType('fixed')}
                  className={`py-2 px-3 border rounded-lg text-sm font-medium transition-colors cursor-pointer ${
                    createDiscountType === 'fixed'
                      ? 'border-black bg-black text-white'
                      : 'border-gray-200 text-gray-700 hover:bg-gray-50'
                  }`}
                >
                  Фиксированная (₽)
                </button>
              </div>
            </div>

            {createDiscountType === 'percent' ? (
              <div>
                <label className="block text-xs font-semibold text-gray-700 uppercase tracking-wider mb-1">
                  Размер скидки (%) <span className="text-red-500">*</span>
                </label>
                <div className="relative">
                  <input
                    type="number"
                    min="1"
                    max="100"
                    step="0.01"
                    required
                    data-testid="input-discount-percent"
                    value={createDiscountPercent}
                    onChange={(e) => setCreateDiscountPercent(e.target.value)}
                    placeholder="10"
                    className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-black"
                  />
                  <span className="absolute right-3 top-2 text-gray-400 text-sm">%</span>
                </div>
              </div>
            ) : (
              <div>
                <label className="block text-xs font-semibold text-gray-700 uppercase tracking-wider mb-1">
                  Размер скидки (₽) <span className="text-red-500">*</span>
                </label>
                <div className="relative">
                  <input
                    type="number"
                    min="1"
                    step="1"
                    required
                    data-testid="input-discount-fixed"
                    value={createDiscountFixedRub}
                    onChange={(e) => setCreateDiscountFixedRub(e.target.value)}
                    placeholder="500"
                    className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-black"
                  />
                  <span className="absolute right-3 top-2 text-gray-400 text-sm">₽</span>
                </div>
              </div>
            )}
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 items-center">
            <div>
              <label className="block text-xs font-semibold text-gray-700 uppercase tracking-wider mb-1">
                Минимальная сумма заказа (₽)
              </label>
              <input
                type="number"
                min="0"
                step="1"
                data-testid="input-min-order"
                value={createMinOrderSubtotalRub}
                onChange={(e) => setCreateMinOrderSubtotalRub(e.target.value)}
                placeholder="0 (без ограничений)"
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-black"
              />
            </div>

            <div className="flex items-center gap-2 sm:pt-5">
              <input
                type="checkbox"
                id="create-first-paid-only"
                data-testid="input-first-paid-only"
                checked={createFirstPaidOnly}
                onChange={(e) => setCreateFirstPaidOnly(e.target.checked)}
                className="h-4 w-4 rounded border-gray-300 text-black focus:ring-black"
              />
              <label htmlFor="create-first-paid-only" className="text-sm text-gray-700">
                Только для первого заказа покупателя
              </label>
            </div>
          </div>

          <div className="border-t border-gray-200 pt-4">
            <h4 className="text-xs font-semibold text-gray-900 uppercase tracking-wider mb-3">
              Период действия (опционально)
            </h4>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label className="block text-xs text-gray-500 mb-1">Дата начала</label>
                <SellerDateTimePicker
                  data-testid="input-starts-at"
                  value={createStartsAt}
                  onChange={setCreateStartsAt}
                  placeholder="Выберите дату и время"
                  defaultTime="00:00"
                  align="left"
                />
              </div>
              <div>
                <label className="block text-xs text-gray-500 mb-1">Дата окончания</label>
                <SellerDateTimePicker
                  data-testid="input-ends-at"
                  value={createEndsAt}
                  onChange={setCreateEndsAt}
                  placeholder="Выберите дату и время"
                  defaultTime="23:59"
                  align="right"
                />
              </div>
            </div>
            {createDateError && (
              <div className="mt-2 text-xs text-red-600 flex items-center gap-1.5" data-testid="create-date-error">
                <AlertCircle className="w-3.5 h-3.5 shrink-0" />
                <span>{createDateError}</span>
              </div>
            )}
          </div>

          <div className="border-t border-gray-200 pt-4">
            <h4 className="text-xs font-semibold text-gray-900 uppercase tracking-wider mb-3">
              Ограничения использования
            </h4>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label className="block text-xs text-gray-500 mb-1">Общий лимит (всего)</label>
                <input
                  type="number"
                  min="1"
                  data-testid="input-global-limit"
                  value={createGlobalLimit}
                  onChange={(e) => setCreateGlobalLimit(e.target.value)}
                  placeholder="Без лимита"
                  className="w-full px-3 py-1.5 border border-gray-300 rounded-lg text-xs focus:outline-none focus:ring-2 focus:ring-black"
                />
              </div>
              <div>
                <label className="block text-xs text-gray-500 mb-1">Лимит на одного клиента</label>
                <input
                  type="number"
                  min="1"
                  required
                  data-testid="input-per-customer-limit"
                  value={createPerCustomerLimit}
                  onChange={(e) => setCreatePerCustomerLimit(e.target.value)}
                  className="w-full px-3 py-1.5 border border-gray-300 rounded-lg text-xs focus:outline-none focus:ring-2 focus:ring-black"
                />
              </div>
            </div>
          </div>

          <div className="pt-4 border-t border-gray-200 flex justify-end gap-3">
            <button
              type="button"
              onClick={() => setIsCreateOpen(false)}
              className="px-4 py-2 border border-gray-300 text-gray-700 rounded-lg text-sm font-medium hover:bg-gray-50 transition-colors cursor-pointer"
            >
              Отмена
            </button>
            <button
              type="submit"
              disabled={createSubmitting || Boolean(createDateError)}
              data-testid="submit-create-promo"
              className="px-4 py-2 bg-black text-white rounded-lg text-sm font-medium hover:bg-gray-800 transition-colors shadow-xs cursor-pointer disabled:opacity-50"
            >
              {createSubmitting ? 'Создание...' : 'Создать'}
            </button>
          </div>
        </form>
      </SellerModal>

      {/* EDIT MODAL (MUTABLE FIELDS ONLY) */}
      <SellerModal
        isOpen={Boolean(editPromo)}
        onClose={() => setEditPromo(null)}
        title={editPromo ? `Параметры промокода: ${editPromo.code}` : ''}
        data-testid="edit-promo-modal"
      >
        {editPromo && (
          <form onSubmit={handleEditSubmit} className="space-y-6">
            {editError && (
              <div
                data-testid="edit-promo-error"
                className="p-3 bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg flex items-start gap-2"
              >
                <AlertCircle className="w-4 h-4 mt-0.5 shrink-0" />
                <span>{editError}</span>
              </div>
            )}

            {/* Read-Only Economics Section */}
            <div className="p-4 bg-gray-50 border border-gray-200 rounded-lg space-y-2">
              <div className="text-xs font-semibold text-gray-500 uppercase tracking-wider">
                Неизменяемые экономические параметры
              </div>
              <div className="grid grid-cols-2 gap-2 text-xs">
                <div>
                  <span className="text-gray-500">Скидка: </span>
                  <span className="font-semibold text-gray-900">
                    {editPromo.discountType === 'percent'
                      ? `${editPromo.discountValueBps / 100}%`
                      : currencyFormatter.format(editPromo.discountValueFixedCents / 100)}
                  </span>
                </div>
                <div>
                  <span className="text-gray-500">Мин. заказ: </span>
                  <span className="font-semibold text-gray-900">
                    {editPromo.minOrderSubtotalCents > 0
                      ? currencyFormatter.format(editPromo.minOrderSubtotalCents / 100)
                      : '0 ₽'}
                  </span>
                </div>
                <div>
                  <span className="text-gray-500">1-й заказ: </span>
                  <span className="font-semibold text-gray-900">
                    {editPromo.firstPaidOrderOnly ? 'Да' : 'Нет'}
                  </span>
                </div>
                <div>
                  <span className="text-gray-500">Финансирование: </span>
                  <span className="font-semibold text-gray-900">Продавец (SELLER)</span>
                </div>
              </div>
              <p className="text-[11px] text-gray-400 mt-1">
                Экономические параметры зафиксированы для защиты уже созданных заказов и расчётов.
              </p>
            </div>

            {/* Editable Operational Fields */}
            <div>
              <label className="block text-xs font-semibold text-gray-700 uppercase tracking-wider mb-2">
                Активность промокода
              </label>
              <div className="flex items-center gap-3">
                <button
                  type="button"
                  data-testid="edit-toggle-active"
                  onClick={() => setEditIsActive(!editIsActive)}
                  className={`inline-flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm font-medium border transition-colors cursor-pointer ${
                    editIsActive
                      ? 'border-emerald-300 bg-emerald-50 text-emerald-800'
                      : 'border-amber-300 bg-amber-50 text-amber-800'
                  }`}
                >
                  {editIsActive ? (
                    <>
                      <CheckCircle2 className="w-4 h-4 text-emerald-600" />
                      Активен (применяется при оформлении)
                    </>
                  ) : (
                    <>
                      <Pause className="w-4 h-4 text-amber-600" />
                      На паузе (новые заказы не принимаются)
                    </>
                  )}
                </button>
              </div>
            </div>

            <div className="border-t border-gray-200 pt-4">
              <h4 className="text-xs font-semibold text-gray-900 uppercase tracking-wider mb-3">
                Период действия
              </h4>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs text-gray-500 mb-1">Дата начала</label>
                  <SellerDateTimePicker
                    data-testid="edit-input-starts-at"
                    value={editStartsAt}
                    onChange={setEditStartsAt}
                    placeholder="Выберите дату и время"
                    defaultTime="00:00"
                    align="left"
                  />
                </div>
                <div>
                  <label className="block text-xs text-gray-500 mb-1">Дата окончания</label>
                  <SellerDateTimePicker
                    data-testid="edit-input-ends-at"
                    value={editEndsAt}
                    onChange={setEditEndsAt}
                    placeholder="Выберите дату и время"
                    defaultTime="23:59"
                    align="right"
                  />
                </div>
              </div>
              {editDateError && (
                <div className="mt-2 text-xs text-red-600 flex items-center gap-1.5" data-testid="edit-date-error">
                  <AlertCircle className="w-3.5 h-3.5 shrink-0" />
                  <span>{editDateError}</span>
                </div>
              )}
            </div>

            <div className="border-t border-gray-200 pt-4">
              <h4 className="text-xs font-semibold text-gray-900 uppercase tracking-wider mb-3">
                Ограничения использования
              </h4>
              <div className="text-xs text-gray-500 mb-2">
                Текущее использование:{' '}
                <span className="font-semibold text-gray-900">
                  {editPromo.consumedUsageCount} завершено, {editPromo.reservedUsageCount} в резерве
                </span>
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs text-gray-500 mb-1">Общий лимит</label>
                  <input
                    type="number"
                    min="1"
                    data-testid="edit-input-global-limit"
                    value={editGlobalLimit}
                    onChange={(e) => setEditGlobalLimit(e.target.value)}
                    placeholder="Без лимита"
                    className="w-full px-3 py-1.5 border border-gray-300 rounded-lg text-xs focus:outline-none focus:ring-2 focus:ring-black"
                  />
                  <p className="text-[11px] text-gray-400 mt-0.5">
                    Не может быть меньше{' '}
                    {editPromo.consumedUsageCount + editPromo.reservedUsageCount}.
                  </p>
                </div>
                <div>
                  <label className="block text-xs text-gray-500 mb-1">Лимит на клиента</label>
                  <input
                    type="number"
                    min="1"
                    required
                    data-testid="edit-input-per-customer-limit"
                    value={editPerCustomerLimit}
                    onChange={(e) => setEditPerCustomerLimit(e.target.value)}
                    className="w-full px-3 py-1.5 border border-gray-300 rounded-lg text-xs focus:outline-none focus:ring-2 focus:ring-black"
                  />
                </div>
              </div>
            </div>

            <div className="pt-4 border-t border-gray-200 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setEditPromo(null)}
                className="px-4 py-2 border border-gray-300 text-gray-700 rounded-lg text-sm font-medium hover:bg-gray-50 transition-colors cursor-pointer"
              >
                Отмена
              </button>
              <button
                type="submit"
                disabled={editSubmitting || Boolean(editDateError)}
                data-testid="submit-edit-promo"
                className="px-4 py-2 bg-black text-white rounded-lg text-sm font-medium hover:bg-gray-800 transition-colors shadow-xs cursor-pointer disabled:opacity-50"
              >
                {editSubmitting ? 'Сохранение...' : 'Сохранить изменения'}
              </button>
            </div>
          </form>
        )}
      </SellerModal>
    </SellerPageFrame>
  );
}
