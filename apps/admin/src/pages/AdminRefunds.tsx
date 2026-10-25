import { useCallback, useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { AlertCircle, AlertTriangle, ReceiptText, RefreshCw, Send, ShieldAlert } from 'lucide-react';
import {
  addAdminBankRefundNote,
  getAdminBankRefund,
  getAdminBankRefunds,
  getBankRefundErrorMessage,
  reconcileAdminBankRefund,
  submitAdminBankRefund,
} from '../api/adminRefunds';
import type { AdminBankRefund, AdminBankRefundDetail, AdminBankRefundList, AdminBankRefundStage } from '../api/adminRefunds';
import { PermissionGuard } from '../components/PermissionGuard';

// BANK.REFUND.2: the bank refund queue. Operators see where each refund stands at the bank, why an action is blocked,
// and may only send a prepared refund, ask the bank for its state, or record an investigation note. There is no
// action that marks a refund as done by hand: only the bank's attributed confirmation does.

const STAGES: { stage: AdminBankRefundStage; label: string; badge: string }[] = [
  { stage: 'prepared', label: 'Возврат подготовлен', badge: 'bg-gray-100 text-gray-800' },
  { stage: 'sending', label: 'Отправка начата', badge: 'bg-blue-100 text-blue-800' },
  { stage: 'accepted', label: 'Принят банком, ожидает подтверждения', badge: 'bg-indigo-100 text-indigo-800' },
  { stage: 'unknown', label: 'Исход неизвестен', badge: 'bg-amber-100 text-amber-900' },
  { stage: 'review', label: 'Требуется сверка', badge: 'bg-red-100 text-red-800' },
  { stage: 'confirmed', label: 'Подтверждён возврат', badge: 'bg-green-100 text-green-800' },
  { stage: 'rejected', label: 'Подтверждён отказ', badge: 'bg-rose-100 text-rose-800' },
  { stage: 'legacy', label: 'Историческая запись (legacy)', badge: 'bg-slate-200 text-slate-800' },
  { stage: 'settled_locally', label: 'Завершён без банка (симулятор)', badge: 'bg-teal-100 text-teal-800' },
  { stage: 'failed_locally', label: 'Отклонён без банка (симулятор)', badge: 'bg-orange-100 text-orange-800' },
];

const stageInfo = (stage: string) => STAGES.find((s) => s.stage === stage) ?? { stage, label: stage, badge: 'bg-gray-100 text-gray-800' };

const formatRub = (cents?: number | null) =>
  cents == null ? '—' : new Intl.NumberFormat('ru-RU', { style: 'currency', currency: 'RUB', minimumFractionDigits: 2 }).format(cents / 100);
const formatDate = (value?: string | null) => (value ? new Date(value).toLocaleString('ru-RU') : '—');
const shortId = (id?: string | null) => (id ? id.slice(0, 8).toUpperCase() : '—');
const refundTitle = (r: AdminBankRefund) => (r.returnId ? `Возврат № ${shortId(r.returnId)}` : `Без заявки · refund ${shortId(r.id)}`);

const HISTORY_LABELS: Record<string, string> = {
  'refund.bank_submit': 'Отправка в банк',
  'refund.bank_reconcile': 'Проверка состояния в банке',
  'refund.bank_note': 'Комментарий расследования',
  'refund.bank_escalated': 'Передано на дальнейшее рассмотрение',
};

function StageBadge({ stage }: { stage: string }) {
  const info = stageInfo(stage);
  return (
    <span data-testid="bank-refund-stage" data-stage={stage} className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${info.badge}`}>
      {info.label}
    </span>
  );
}

export function AdminRefunds() {
  const [searchParams, setSearchParams] = useSearchParams();
  const stageFilter = searchParams.get('stage') ?? '';
  const selectedId = searchParams.get('refund');

  const [list, setList] = useState<AdminBankRefundList | null>(null);
  const [detail, setDetail] = useState<AdminBankRefundDetail | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isActing, setIsActing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [confirmSubmit, setConfirmSubmit] = useState(false);
  const [noteText, setNoteText] = useState('');
  const [noteEscalate, setNoteEscalate] = useState(false);

  const fetchList = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      setList(await getAdminBankRefunds({ stage: stageFilter || undefined, limit: 100 }));
    } catch (err: unknown) {
      setError(getBankRefundErrorMessage(err, 'Не удалось загрузить возмещения.'));
    } finally {
      setIsLoading(false);
    }
  }, [stageFilter]);

  const fetchDetail = useCallback(async (id: string) => {
    try {
      setActionError(null);
      setDetail(await getAdminBankRefund(id));
    } catch (err: unknown) {
      setDetail(null);
      setActionError(getBankRefundErrorMessage(err, 'Не удалось загрузить детали возмещения.'));
    }
  }, []);

  useEffect(() => {
    fetchList();
  }, [fetchList]);

  useEffect(() => {
    if (selectedId) fetchDetail(selectedId);
    else setDetail(null);
  }, [selectedId, fetchDetail]);

  const setParam = (key: string, value: string | null) => {
    const next = new URLSearchParams(searchParams);
    if (value) next.set(key, value);
    else next.delete(key);
    setSearchParams(next);
  };

  const runAction = async (fn: () => Promise<AdminBankRefundDetail>, fallback: string) => {
    if (isActing) return;
    try {
      setIsActing(true);
      setActionError(null);
      setDetail(await fn());
      await fetchList();
    } catch (err: unknown) {
      setActionError(getBankRefundErrorMessage(err, fallback));
      if (detail) await fetchDetail(detail.id);
    } finally {
      setIsActing(false);
      setConfirmSubmit(false);
    }
  };

  const handleSubmit = () => detail && runAction(() => submitAdminBankRefund(detail.id), 'Не удалось отправить возврат в банк.');
  const handleReconcile = () => detail && runAction(() => reconcileAdminBankRefund(detail.id), 'Не удалось проверить состояние в банке.');
  const handleNote = async () => {
    if (!detail || !noteText.trim()) return;
    await runAction(() => addAdminBankRefundNote(detail.id, noteText.trim(), noteEscalate), 'Не удалось записать комментарий.');
    setNoteText('');
    setNoteEscalate(false);
  };

  const counts = list?.stageCounts ?? {};
  const total = Object.values(counts).reduce((a, b) => a + (b ?? 0), 0);
  const items: AdminBankRefund[] = list?.items ?? [];
  const can = (a: string) => detail?.allowedActions.includes(a as never) ?? false;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Возмещения</h1>
        <p className="mt-1 text-sm text-gray-500">Банковские возвраты денег покупателям: стадия отправки, сверка и ручной разбор.</p>
      </div>

      {list && !list.providerConfigured && (
        <div data-testid="bank-provider-disabled" className="bg-amber-50 border-l-4 border-amber-400 p-4 text-sm text-amber-900">
          Отправка в банк выключена: провайдер возвратов не подключён. Возвраты остаются в резерве и не отправляются.
        </div>
      )}

      {error && (
        <div className="p-4 bg-red-50 text-red-700 rounded-md flex items-center">
          <AlertCircle className="h-5 w-5 mr-2 shrink-0" />
          {error}
        </div>
      )}

      <div className="flex flex-wrap gap-2" role="tablist" aria-label="Стадии возвратов">
        <button
          data-testid="bank-stage-filter-all"
          onClick={() => setParam('stage', null)}
          className={`px-3 py-1.5 rounded-full text-xs font-medium border ${!stageFilter ? 'bg-indigo-600 text-white border-indigo-600' : 'bg-white text-gray-700 border-gray-300'}`}
        >
          Все ({total})
        </button>
        {STAGES.filter((s) => (counts[s.stage] ?? 0) > 0 || s.stage === stageFilter).map((s) => (
          <button
            key={s.stage}
            data-testid={`bank-stage-filter-${s.stage}`}
            onClick={() => setParam('stage', s.stage)}
            className={`px-3 py-1.5 rounded-full text-xs font-medium border ${stageFilter === s.stage ? 'bg-indigo-600 text-white border-indigo-600' : 'bg-white text-gray-700 border-gray-300'}`}
          >
            {s.label} ({counts[s.stage] ?? 0})
          </button>
        ))}
      </div>

      {isLoading ? (
        <div className="text-center py-10">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600 mx-auto"></div>
          <p className="mt-2 text-sm text-gray-500">Загрузка возмещений...</p>
        </div>
      ) : items.length === 0 ? (
        <div className="text-center py-10 bg-white rounded-lg shadow">
          <ReceiptText className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-2 text-sm font-medium text-gray-900">Нет возмещений</h3>
          <p className="mt-1 text-sm text-gray-500">В этой стадии записей нет.</p>
        </div>
      ) : (
        <>
          <div className="hidden md:block shadow overflow-hidden border-b border-gray-200 sm:rounded-lg">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Заявка</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Заказ</th>
                  <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">Сумма</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Статус</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Последняя проверка</th>
                  <th className="px-4 py-3"><span className="sr-only">Действия</span></th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-gray-200">
                {items.map((r) => (
                  <tr key={r.id} data-testid={`bank-refund-row-${r.id}`} className={r.id === selectedId ? 'bg-indigo-50' : ''}>
                    <td className="px-4 py-3 text-sm">
                      <div className="font-medium text-gray-900">{refundTitle(r)}</div>
                      <div className="text-xs text-gray-500">refund {shortId(r.id)}</div>
                    </td>
                    <td className="px-4 py-3 text-sm text-gray-700">{r.orderNumber || shortId(r.orderId)}</td>
                    <td className="px-4 py-3 text-sm text-right text-gray-900 whitespace-nowrap">{formatRub(r.amountCents)}</td>
                    <td className="px-4 py-3"><StageBadge stage={r.stage} /></td>
                    <td className="px-4 py-3 text-sm text-gray-500 whitespace-nowrap">{formatDate(r.lastCheckedAt)}</td>
                    <td className="px-4 py-3 text-right text-sm">
                      <button onClick={() => setParam('refund', r.id)} className="text-indigo-600 hover:text-indigo-900 font-medium">Открыть</button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="md:hidden space-y-3">
            {items.map((r) => (
              <button
                key={r.id}
                data-testid={`bank-refund-card-${r.id}`}
                onClick={() => setParam('refund', r.id)}
                className="w-full text-left bg-white rounded-lg shadow p-4 space-y-2"
              >
                <div className="flex justify-between gap-2">
                  <span className="text-sm font-medium text-gray-900">{refundTitle(r)}</span>
                  <span className="text-sm font-semibold text-gray-900 whitespace-nowrap">{formatRub(r.amountCents)}</span>
                </div>
                <div className="text-xs text-gray-500">Заказ {r.orderNumber || shortId(r.orderId)}</div>
                <StageBadge stage={r.stage} />
              </button>
            ))}
          </div>
        </>
      )}

      {selectedId && (
        <div data-testid="bank-refund-detail" className="bg-white shadow sm:rounded-lg p-4 sm:p-6 space-y-5">
          {!detail ? (
            actionError ? <p className="text-sm text-red-700">{actionError}</p> : <p className="text-sm text-gray-500">Загрузка...</p>
          ) : (
            <>
              <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-3">
                <div>
                  <h2 className="text-lg font-medium text-gray-900">{refundTitle(detail)} · {formatRub(detail.amountCents)}</h2>
                  <p className="mt-1 text-xs text-gray-500 break-all">refund {detail.id}</p>
                </div>
                <div className="flex items-center gap-3">
                  <StageBadge stage={detail.stage} />
                  <button onClick={() => setParam('refund', null)} className="text-sm text-gray-500 hover:text-gray-700">Закрыть</button>
                </div>
              </div>

              <dl className="grid gap-4 grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 text-sm">
                <div><dt className="font-medium text-gray-500">Заказ</dt><dd className="mt-1 text-gray-900">{detail.orderNumber || detail.orderId}</dd></div>
                <div>
                  <dt className="font-medium text-gray-500">Сумма</dt>
                  <dd className="mt-1 text-gray-900">
                    {formatRub(detail.amountCents)}
                    {detail.isPartial && <span className="block text-xs text-gray-500">частичный, из {formatRub(detail.paymentAmountCents)}</span>}
                  </dd>
                </div>
                <div><dt className="font-medium text-gray-500">Провайдер</dt><dd className="mt-1 text-gray-900">{detail.providerName || 'не подключён'}</dd></div>
                <div><dt className="font-medium text-gray-500">Статус банка</dt><dd className="mt-1 text-gray-900">{detail.providerStatus || '—'}</dd></div>
                <div><dt className="font-medium text-gray-500">Попыток отправки</dt><dd className="mt-1 text-gray-900">{detail.submitAttempts}</dd></div>
                <div><dt className="font-medium text-gray-500">Отправлен</dt><dd className="mt-1 text-gray-900">{formatDate(detail.submittedAt)}</dd></div>
                <div><dt className="font-medium text-gray-500">Последняя проверка</dt><dd data-testid="bank-refund-last-checked" className="mt-1 text-gray-900">{formatDate(detail.lastCheckedAt)}</dd></div>
                <div><dt className="font-medium text-gray-500">Создан</dt><dd className="mt-1 text-gray-900">{formatDate(detail.createdAt)}</dd></div>
              </dl>

              {detail.reviewReason && (
                <div data-testid="bank-refund-review" className="p-4 bg-red-50 border border-red-200 rounded-md text-sm text-red-900 flex gap-2">
                  <ShieldAlert className="h-5 w-5 shrink-0" />
                  <div>
                    <p className="font-medium">Причина ручной сверки</p>
                    <p className="mt-1">{detail.reviewExplanation || detail.reviewReason}</p>
                    <p className="mt-1 text-xs text-red-700">Технически: {detail.reviewReason}</p>
                  </div>
                </div>
              )}
              {detail.blockingReason && (
                <div data-testid="bank-refund-blocking" className="p-4 bg-amber-50 border border-amber-200 rounded-md text-sm text-amber-900 flex gap-2">
                  <AlertTriangle className="h-5 w-5 shrink-0" />
                  <p>{detail.blockingReason}</p>
                </div>
              )}
              {actionError && (
                <div data-testid="bank-refund-action-error" className="p-3 bg-red-50 text-red-700 rounded-md text-sm">{actionError}</div>
              )}

              <PermissionGuard permission="refunds.create">
                <div className="flex flex-wrap gap-3">
                  {can('submit') && !confirmSubmit && (
                    <button
                      data-testid="bank-refund-submit"
                      onClick={() => setConfirmSubmit(true)}
                      disabled={isActing}
                      className="inline-flex items-center gap-2 px-4 py-2 rounded-md bg-indigo-600 text-white text-sm font-medium hover:bg-indigo-700 disabled:opacity-50"
                    >
                      <Send className="h-4 w-4" /> Отправить в банк
                    </button>
                  )}
                  {can('submit') && confirmSubmit && (
                    <div data-testid="bank-refund-submit-confirm" className="w-full p-4 border border-indigo-200 bg-indigo-50 rounded-md text-sm space-y-3">
                      <p>Банк получит запрос на возврат {formatRub(detail.amountCents)}. Запрос отправляется один раз и не повторяется автоматически.</p>
                      <div className="flex gap-3">
                        <button data-testid="bank-refund-submit-yes" onClick={handleSubmit} disabled={isActing} className="px-4 py-2 rounded-md bg-indigo-600 text-white font-medium disabled:opacity-50">
                          {isActing ? 'Отправка...' : 'Подтвердить отправку'}
                        </button>
                        <button onClick={() => setConfirmSubmit(false)} disabled={isActing} className="px-4 py-2 rounded-md border border-gray-300 bg-white">Отмена</button>
                      </div>
                    </div>
                  )}
                  {can('reconcile') && (
                    <button
                      data-testid="bank-refund-reconcile"
                      onClick={handleReconcile}
                      disabled={isActing}
                      className="inline-flex items-center gap-2 px-4 py-2 rounded-md border border-gray-300 bg-white text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
                    >
                      <RefreshCw className="h-4 w-4" /> Проверить состояние в банке
                    </button>
                  )}
                </div>
              </PermissionGuard>

              <p className="text-xs text-gray-500">
                Признать возврат выполненным вручную нельзя: возврат завершается только подтверждением банка, отнесённым к этой попытке.
              </p>

              <PermissionGuard permission="refunds.create">
                <div className="space-y-2">
                  <label htmlFor="bank-refund-note" className="block text-sm font-medium text-gray-700">Комментарий расследования</label>
                  <textarea
                    id="bank-refund-note"
                    data-testid="bank-refund-note-input"
                    value={noteText}
                    onChange={(e) => setNoteText(e.target.value)}
                    maxLength={2000}
                    rows={3}
                    className="w-full rounded-md border border-gray-300 p-2 text-sm"
                    placeholder="Что проверено, какой документ запрошен у банка"
                  />
                  <label className="flex items-center gap-2 text-sm text-gray-700">
                    <input type="checkbox" data-testid="bank-refund-note-escalate" checked={noteEscalate} onChange={(e) => setNoteEscalate(e.target.checked)} />
                    Передать на дальнейшее рассмотрение
                  </label>
                  <button
                    data-testid="bank-refund-note-submit"
                    onClick={handleNote}
                    disabled={isActing || !noteText.trim()}
                    className="px-4 py-2 rounded-md border border-gray-300 bg-white text-sm font-medium text-gray-700 disabled:opacity-50"
                  >
                    Записать в журнал
                  </button>
                </div>
              </PermissionGuard>

              <div>
                <h3 className="text-sm font-medium text-gray-900">Журнал операций</h3>
                {detail.history.length === 0 ? (
                  <p className="mt-2 text-sm text-gray-500">Действий сотрудников пока нет.</p>
                ) : (
                  <ul data-testid="bank-refund-history" className="mt-2 divide-y divide-gray-100 text-sm">
                    {detail.history.map((e, i) => (
                      <li key={i} className="py-2">
                        <div className="flex flex-wrap justify-between gap-2">
                          <span className="font-medium text-gray-900">{HISTORY_LABELS[e.action] ?? e.action}</span>
                          <span className="text-xs text-gray-500">{formatDate(e.createdAt)} · {e.actorEmail || 'сотрудник'}</span>
                        </div>
                        {e.outcome && <div className="text-xs text-gray-500">Стадия: {stageInfo(e.outcome).label}</div>}
                        {e.comment && <p className="mt-1 text-gray-700 whitespace-pre-wrap">{e.comment}</p>}
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </>
          )}
        </div>
      )}
    </div>
  );
}
