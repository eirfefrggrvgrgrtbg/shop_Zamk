import { useEffect, useState } from 'react';
import { CheckCircle, Info, XCircle } from 'lucide-react';
import { getSellerWarnings, getSellerViolations } from '@zamk/api-client/src/seller';
import type { SellerWarning, SellerViolation } from '@zamk/api-client/src/types';

const SEVERITY_COLORS: Record<string, string> = {
  low: 'bg-yellow-100 text-yellow-800 border-yellow-200',
  medium: 'bg-orange-100 text-orange-800 border-orange-200',
  high: 'bg-red-100 text-red-800 border-red-200',
};

const SEVERITY_LABELS: Record<string, string> = {
  low: 'Низкая',
  medium: 'Средняя',
  high: 'Высокая',
};

export function SellerWarnings() {
  const [warnings, setWarnings] = useState<SellerWarning[]>([]);
  const [violations, setViolations] = useState<SellerViolation[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;

    async function load() {
      setIsLoading(true);
      try {
        const [warns, viols] = await Promise.all([
          getSellerWarnings(),
          getSellerViolations(),
        ]);
        if (!cancelled) {
          setWarnings(warns);
          setViolations(viols);
        }
      } catch (err) {
        if (!cancelled) {
          setError('Не удалось загрузить предупреждения и нарушения.');
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    }

    load();
    return () => { cancelled = true; };
  }, []);

  const activeWarnings = warnings.filter(w => w.status === 'active');
  const activeViolations = violations.filter(v => v.status === 'active');
  const penaltyViolationsCount = activeViolations.filter(v => v.countsForPenalty).length;

  const history = [
    ...warnings.filter(w => w.status !== 'active').map(w => ({ ...w, _kind: 'warning' as const })),
    ...violations.filter(v => v.status !== 'active').map(v => ({ ...v, _kind: 'violation' as const }))
  ].sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime());

  if (isLoading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <p className="text-gray-500">Загрузка данных...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-8">
        <div className="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700">{error}</div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 p-8">
      <div className="mx-auto max-w-4xl space-y-8">
        <div>
          <p className="text-sm font-medium uppercase tracking-wide text-gray-500">Политика платформы</p>
          <h1 className="mt-2 text-3xl font-bold text-gray-900">Предупреждения и нарушения</h1>
        </div>

        {/* Инфо о комиссии */}
        <div className="rounded-2xl border border-blue-200 bg-blue-50 p-5">
          <div className="flex items-start gap-3">
            <Info className="mt-0.5 h-5 w-5 flex-shrink-0 text-blue-600" />
            <div className="text-sm text-blue-800">
              <p className="font-semibold">Базовая комиссия ZAMK — 9%.</p>
              <p className="mt-1">
                Сейчас автоматическое повышение комиссии не включено.
              </p>
              {penaltyViolationsCount >= 2 && (
                <p className="mt-3 font-semibold text-red-700">
                  У магазина 2 или более активных нарушения. В будущем комиссия может быть временно повышена до 18% на 1 месяц.
                </p>
              )}
            </div>
          </div>
        </div>

        {/* Активные нарушения */}
        <section className="space-y-4">
          <h2 className="text-lg font-semibold text-gray-900">Активные нарушения ({activeViolations.length})</h2>
          {activeViolations.length === 0 ? (
            <div className="rounded-xl border border-dashed border-gray-300 bg-white p-5 text-sm text-gray-500">
              Нет активных нарушений.
            </div>
          ) : (
            <div className="space-y-3">
              {activeViolations.map(v => (
                <div key={v.id} className={`rounded-xl border p-5 ${SEVERITY_COLORS[v.severity]}`}>
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <h3 className="font-semibold">{v.title}</h3>
                      <p className="mt-1 opacity-90">{v.description}</p>
                    </div>
                    <span className="whitespace-nowrap rounded-md bg-white/50 px-2.5 py-1 text-xs font-medium backdrop-blur-sm">
                      {SEVERITY_LABELS[v.severity]}
                    </span>
                  </div>
                  <div className="mt-3 text-xs opacity-75">
                    Создано: {new Date(v.createdAt).toLocaleDateString('ru-RU')} · {v.countsForPenalty ? 'Влияет на комиссию' : 'Не влияет на комиссию'}
                  </div>
                </div>
              ))}
            </div>
          )}
        </section>

        {/* Активные предупреждения */}
        <section className="space-y-4">
          <h2 className="text-lg font-semibold text-gray-900">Активные предупреждения ({activeWarnings.length})</h2>
          {activeWarnings.length === 0 ? (
            <div className="rounded-xl border border-dashed border-gray-300 bg-white p-5 text-sm text-gray-500">
              Нет активных предупреждений.
            </div>
          ) : (
            <div className="space-y-3">
              {activeWarnings.map(w => (
                <div key={w.id} className="rounded-xl border border-gray-200 bg-white p-5">
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <h3 className="font-semibold text-gray-900">{w.title}</h3>
                      <p className="mt-1 text-sm text-gray-600">{w.message}</p>
                    </div>
                    <span className={`whitespace-nowrap rounded-md px-2.5 py-1 text-xs font-medium border ${SEVERITY_COLORS[w.severity]}`}>
                      {SEVERITY_LABELS[w.severity]}
                    </span>
                  </div>
                  <div className="mt-3 text-xs text-gray-400">
                    Создано: {new Date(w.createdAt).toLocaleDateString('ru-RU')}
                  </div>
                </div>
              ))}
            </div>
          )}
        </section>

        {/* История */}
        {history.length > 0 && (
          <section className="space-y-4">
            <h2 className="text-lg font-semibold text-gray-900">История</h2>
            <div className="space-y-3">
              {history.map(item => (
                <div key={item.id} className="rounded-xl border border-gray-200 bg-gray-50 p-5 opacity-75">
                  <div className="flex items-start justify-between gap-4">
                    <div className="flex items-center gap-3">
                      {item.status === 'resolved' ? (
                        <CheckCircle className="h-5 w-5 text-green-500" />
                      ) : (
                        <XCircle className="h-5 w-5 text-gray-400" />
                      )}
                      <div>
                        <h3 className="font-medium text-gray-900">
                          {item.title} ({item._kind === 'warning' ? 'Предупреждение' : 'Нарушение'})
                        </h3>
                        <p className="mt-0.5 text-sm text-gray-600">
                          Статус: {item.status === 'resolved' ? 'Решено' : 'Отменено'}
                        </p>
                      </div>
                    </div>
                    <div className="text-right text-xs text-gray-500">
                      <div>Создано: {new Date(item.createdAt).toLocaleDateString('ru-RU')}</div>
                      {item.resolvedAt && (
                        <div>Закрыто: {new Date(item.resolvedAt).toLocaleDateString('ru-RU')}</div>
                      )}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </section>
        )}

      </div>
    </div>
  );
}
