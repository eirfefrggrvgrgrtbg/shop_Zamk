import { CheckCircle2, Circle, XCircle } from 'lucide-react';
import type { CustomerFulfillment } from '@zamk/api-client/src/types';
import { buildTrackingSteps, parcelStatusLabel } from './trackingSteps';

function formatDate(iso?: string): string | null {
  if (!iso) return null;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return null;
  return d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' });
}

interface Props {
  status: string;
  createdAt?: string;
  cancelledAt?: string | null;
  fulfillments?: CustomerFulfillment[] | null;
  /** the parcels are still being loaded */
  loading?: boolean;
  /** the parcels could not be loaded: the order-level timeline stays, a neutral note is shown */
  failed?: boolean;
}

export function OrderTracking({ status, createdAt, cancelledAt, fulfillments, loading, failed }: Props) {
  const steps = buildTrackingSteps({ status, createdAt, cancelledAt, fulfillments });
  const showParcels = !loading && !failed && (fulfillments?.length ?? 0) > 1 && status !== 'cancelled';

  return (
    <div data-testid="order-tracking" className="mt-5 pt-4 border-t border-border-lighter/60 dark:border-white/5">
      <ol className="space-y-0">
        {steps.map((step, idx) => {
          const isLast = idx === steps.length - 1;
          const date = formatDate(step.date);
          return (
            <li key={step.key} data-testid={`tracking-step-${step.key}`} data-state={step.state} className="flex gap-3">
              <div className="flex flex-col items-center">
                {step.state === 'done' ? (
                  <CheckCircle2 className="w-5 h-5 text-emerald-600 dark:text-emerald-400" aria-hidden />
                ) : step.state === 'cancelled' ? (
                  <XCircle className="w-5 h-5 text-rose-600 dark:text-rose-400" aria-hidden />
                ) : step.state === 'current' ? (
                  <Circle className="w-5 h-5 text-blue-600 dark:text-blue-400 fill-blue-100 dark:fill-blue-950" aria-hidden />
                ) : (
                  <Circle className="w-5 h-5 text-graphite/25 dark:text-white/20" aria-hidden />
                )}
                {!isLast && (
                  <div className={`w-px flex-1 min-h-[14px] my-1 ${step.state === 'done' ? 'bg-emerald-600/60 dark:bg-emerald-400/60' : 'bg-border-lighter dark:bg-white/10'}`} />
                )}
              </div>
              <div className="pb-3 min-w-0">
                <p className={`text-[14px] leading-5 ${step.state === 'pending' ? 'text-graphite/50 dark:text-white/40 font-medium' : step.state === 'cancelled' ? 'text-rose-700 dark:text-rose-300 font-semibold' : 'text-graphite dark:text-white font-semibold'}`}>
                  {step.label}
                </p>
                {(date || step.note) && (
                  <p className="text-[12.5px] text-graphite/65 dark:text-white/60 mt-0.5">
                    {[date, step.note].filter(Boolean).join(' · ')}
                  </p>
                )}
              </div>
            </li>
          );
        })}
      </ol>

      {showParcels && (
        <ul data-testid="tracking-parcels" className="mt-1 space-y-1.5">
          {fulfillments!.map((f, i) => (
            <li key={f.id} className="flex items-center justify-between gap-3 text-[13px] px-3 py-2 rounded-xl bg-graphite/[0.03] dark:bg-white/5 border border-border-lighter dark:border-white/10">
              <span className="font-medium text-graphite dark:text-white">Посылка №{i + 1}</span>
              <span className="text-graphite/70 dark:text-white/70 text-right">{parcelStatusLabel(f.status)}</span>
            </li>
          ))}
        </ul>
      )}
      {failed && status !== 'cancelled' && (
        <p className="mt-1 text-[12.5px] text-graphite/55 dark:text-white/50">Сведения о посылках временно недоступны</p>
      )}
    </div>
  );
}
