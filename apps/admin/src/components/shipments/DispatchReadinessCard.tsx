import { CheckCircle2, XCircle, AlertTriangle, Info } from 'lucide-react';
import type { DispatchReadiness, ReadinessCheck } from '../../api/adminPicking';

// FBO.DISPATCH.CHECKLIST.1: shows the backend's answer to "is this parcel ready to be handed over". It decides
// nothing: the readiness comes from the dispatch-context and the dispatch endpoint checks everything again.

export const READINESS_TITLES: Record<string, string> = {
  ORDER_STATE: 'Заказ в состоянии, допускающем отгрузку',
  NOT_ALREADY_SHIPPED: 'Посылка ещё не отгружена',
  PICKING_COMPLETE: 'Комплектация выполнена',
  PACKING_COMPLETE: 'Упаковка завершена',
  COMPOSITION_CONSISTENT: 'Состав посылки согласован с собранным',
  UNITS_AND_STOCK: 'Единицы на складе, остаток и резерв достаточны',
  SHIPMENT_STATE: 'Запись отправления в допустимом состоянии',
  RECIPIENT_PRESENT: 'Получатель указан',
  PHONE_PRESENT: 'Телефон получателя указан',
  ADDRESS_PRESENT: 'Адрес доставки указан',
  DELIVERY_METHOD: 'Способ доставки указан',
  POSTAL_CODE: 'Почтовый индекс',
  COURIER_COMMENT: 'Комментарий курьеру',
  CARRIER_AT_DISPATCH: 'Служба доставки и трек-номер',
};

function Row({ check }: { check: ReadinessCheck }) {
  const failed = !check.ok;
  const Icon = check.ok ? CheckCircle2 : check.severity === 'blocking' ? XCircle : check.severity === 'warning' ? AlertTriangle : Info;
  const color = check.ok
    ? 'text-emerald-600'
    : check.severity === 'blocking'
      ? 'text-rose-600'
      : check.severity === 'warning'
        ? 'text-amber-600'
        : 'text-gray-400';
  return (
    <li
      data-testid={`readiness-check-${check.code}`}
      data-ok={check.ok ? 'true' : 'false'}
      data-severity={check.severity}
      className="flex items-start gap-2.5 py-1.5"
    >
      <Icon className={`w-4 h-4 mt-0.5 shrink-0 ${color}`} aria-hidden />
      <div className="min-w-0">
        <div className={`text-xs font-semibold ${failed && check.severity === 'blocking' ? 'text-rose-800' : 'text-gray-900'}`}>
          {READINESS_TITLES[check.code] ?? check.code}
        </div>
        {check.detail && <div className="text-[11px] text-gray-600 break-words">{check.detail}</div>}
      </div>
    </li>
  );
}

export function DispatchReadinessCard({ readiness }: { readiness: DispatchReadiness }) {
  const blocking = readiness.checks.filter((c) => c.severity === 'blocking');
  const warnings = readiness.checks.filter((c) => c.severity === 'warning');
  const info = readiness.checks.filter((c) => c.severity === 'info');

  return (
    <div data-testid="dispatch-readiness" data-ready={readiness.ready ? 'true' : 'false'} className="bg-white border border-gray-200 rounded-xl p-5 shadow-sm space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="text-sm font-bold text-gray-900">Готовность к отгрузке</h3>
        {readiness.ready ? (
          <span data-testid="readiness-overall" className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200">
            <CheckCircle2 className="w-3.5 h-3.5" />
            Готова к отгрузке
          </span>
        ) : (
          <span data-testid="readiness-overall" className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold bg-rose-50 text-rose-700 border border-rose-200">
            <XCircle className="w-3.5 h-3.5" />
            Есть препятствия ({readiness.blockingCount})
          </span>
        )}
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-x-8 gap-y-3">
        <div>
          <div className="text-[11px] uppercase tracking-wide font-semibold text-gray-500 mb-1">Обязательные условия</div>
          <ul>{blocking.map((c) => <Row key={c.code} check={c} />)}</ul>
        </div>
        <div className="space-y-3">
          {warnings.length > 0 && (
            <div>
              <div className="text-[11px] uppercase tracking-wide font-semibold text-gray-500 mb-1">Предупреждения (не блокируют)</div>
              <ul>{warnings.map((c) => <Row key={c.code} check={c} />)}</ul>
            </div>
          )}
          {info.length > 0 && (
            <div>
              <div className="text-[11px] uppercase tracking-wide font-semibold text-gray-500 mb-1">Справочно</div>
              <ul>{info.map((c) => <Row key={c.code} check={c} />)}</ul>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
