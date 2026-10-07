import { useMemo, useRef, useState, type FormEvent } from 'react';
import { ChevronLeft, ChevronRight } from 'lucide-react';

export type Period = 'today' | '7d' | '30d' | 'custom';
export type MarketingRange = { from: string; to: string };

const options: { value: Period; label: string }[] = [
  { value: 'today', label: 'Сегодня' },
  { value: '7d', label: '7 дней' },
  { value: '30d', label: '30 дней' },
  { value: 'custom', label: 'Период' },
];

const MONTH_NAMES = [
  'Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь',
  'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь',
];

const WEEKDAY_NAMES = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'];

// The API uses an exclusive upper bound. A custom end date includes its entire UTC day.
export function customRange(from: string, to: string): MarketingRange | null {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(from) || !/^\d{4}-\d{2}-\d{2}$/.test(to)) return null;
  const start = new Date(`${from}T00:00:00.000Z`);
  const end = new Date(`${to}T00:00:00.000Z`);
  if (!Number.isFinite(start.getTime()) || !Number.isFinite(end.getTime()) || start > end) return null;
  if (start.toISOString().slice(0, 10) !== from || end.toISOString().slice(0, 10) !== to) return null;
  end.setUTCDate(end.getUTCDate() + 1);
  if (!Number.isFinite(end.getTime())) return null;
  return { from: start.toISOString(), to: end.toISOString() };
}

export function formatDisplayDate(dateStr: string | null): string {
  if (!dateStr) return '—';
  const parts = dateStr.split('-');
  if (parts.length !== 3) return dateStr;
  return `${parts[2]}.${parts[1]}.${parts[0]}`;
}

export function useMarketingRange() {
  const [period, setPeriod] = useState<Period>('30d');
  const [custom, setCustom] = useState<MarketingRange | null>(null);
  const range = useMemo(() => {
    if (period === 'custom' && custom) return custom;
    const to = new Date();
    const from = new Date(to);
    if (period === 'today') from.setUTCHours(0, 0, 0, 0);
    else from.setUTCDate(from.getUTCDate() - (period === '7d' ? 7 : 30));
    return { from: from.toISOString(), to: to.toISOString() };
  }, [period, custom]);
  return {
    period,
    range,
    select: (value: Period, dates?: MarketingRange) => {
      if (dates) setCustom(dates);
      setPeriod(value);
    },
  };
}

function getMonthDays(year: number, month: number) {
  const daysInMonth = new Date(Date.UTC(year, month + 1, 0)).getUTCDate();
  // Monday-based offset (0 = Monday, 6 = Sunday)
  const firstDay = (new Date(Date.UTC(year, month, 1)).getUTCDay() + 6) % 7;
  const days: { dateStr: string; dayNum: number }[] = [];
  for (let d = 1; d <= daysInMonth; d++) {
    const mm = String(month + 1).padStart(2, '0');
    const dd = String(d).padStart(2, '0');
    days.push({
      dateStr: `${year}-${mm}-${dd}`,
      dayNum: d,
    });
  }
  return { firstDay, days };
}

export function MarketingPeriodControl({
  period,
  range,
  onSelect,
}: {
  period: Period;
  range: MarketingRange;
  onSelect: (period: Period, range?: MarketingRange) => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);

  const [startDate, setStartDate] = useState<string | null>(null);
  const [endDate, setEndDate] = useState<string | null>(null);
  const [hoverDate, setHoverDate] = useState<string | null>(null);
  const [error, setError] = useState('');

  // Initial view year/month for the calendar
  const [viewDate, setViewDate] = useState<{ year: number; month: number }>(() => {
    const d = new Date();
    return { year: d.getUTCFullYear(), month: d.getUTCMonth() };
  });

  const close = () => {
    dialog.current?.close();
    trigger.current?.focus();
  };

  const openModal = () => {
    const fromStr = range.from.slice(0, 10);
    const toStr = new Date(Date.parse(range.to) - 1).toISOString().slice(0, 10);
    setStartDate(fromStr);
    setEndDate(toStr);
    setHoverDate(null);
    setError('');

    const startYear = parseInt(fromStr.slice(0, 4), 10);
    const startMonth = parseInt(fromStr.slice(5, 7), 10) - 1;
    if (Number.isFinite(startYear) && Number.isFinite(startMonth)) {
      setViewDate({ year: startYear, month: startMonth });
    }

    dialog.current?.showModal();
  };

  const handleDayClick = (dateStr: string) => {
    setError('');
    if (!startDate || (startDate && endDate)) {
      // Starting new selection
      setStartDate(dateStr);
      setEndDate(null);
    } else {
      // Completing selection
      if (dateStr < startDate) {
        setStartDate(dateStr);
        setEndDate(startDate);
      } else {
        setEndDate(dateStr);
      }
    }
  };

  const handlePrevMonth = () => {
    setViewDate(prev => {
      if (prev.month === 0) return { year: prev.year - 1, month: 11 };
      return { year: prev.year, month: prev.month - 1 };
    });
  };

  const handleNextMonth = () => {
    setViewDate(prev => {
      if (prev.month === 11) return { year: prev.year + 1, month: 0 };
      return { year: prev.year, month: prev.month + 1 };
    });
  };

  const apply = (event: FormEvent) => {
    event.preventDefault();
    if (!startDate || !endDate) {
      setError('Выберите начальную и конечную дату');
      return;
    }
    const value = customRange(startDate, endDate);
    if (!value) {
      setError('Укажите корректный период: начало не позже окончания');
      return;
    }
    onSelect('custom', value);
    close();
  };

  // Month 1 & Month 2 computations
  const month1 = viewDate;
  const month2 = {
    year: month1.month === 11 ? month1.year + 1 : month1.year,
    month: month1.month === 11 ? 0 : month1.month + 1,
  };

  const effectiveStart = startDate && !endDate && hoverDate && hoverDate < startDate ? hoverDate : startDate;
  const effectiveEnd = endDate || (startDate && !endDate && hoverDate && hoverDate >= startDate ? hoverDate : null);

  const renderMonthCalendar = (y: number, m: number, isSecond = false) => {
    const { firstDay, days } = getMonthDays(y, m);

    return (
      <div className="flex-1 min-w-0 select-none">
        <div className="flex items-center justify-between mb-3 px-1">
          {!isSecond ? (
            <button
              type="button"
              onClick={handlePrevMonth}
              aria-label="Предыдущий месяц"
              className="p-1 rounded-md text-gray-400 hover:text-gray-900 hover:bg-gray-100 transition-colors"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>
          ) : (
            <div className="w-6" />
          )}

          <span className="text-xs font-semibold tracking-wide text-gray-900">
            {MONTH_NAMES[m]} {y}
          </span>

          {isSecond ? (
            <button
              type="button"
              onClick={handleNextMonth}
              aria-label="Следующий месяц"
              className="p-1 rounded-md text-gray-400 hover:text-gray-900 hover:bg-gray-100 transition-colors"
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          ) : (
            <button
              type="button"
              onClick={handleNextMonth}
              aria-label="Следующий месяц"
              className="md:hidden p-1 rounded-md text-gray-400 hover:text-gray-900 hover:bg-gray-100 transition-colors"
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          )}
        </div>

        <div className="grid grid-cols-7 gap-y-1 text-center">
          {WEEKDAY_NAMES.map(day => (
            <div key={day} className="text-[10px] font-semibold uppercase tracking-wider text-gray-400 py-1">
              {day}
            </div>
          ))}

          {Array.from({ length: firstDay }).map((_, idx) => (
            <div key={`empty-${idx}`} className="h-8" />
          ))}

          {days.map(({ dateStr, dayNum }) => {
            const isStart = dateStr === startDate;
            const isEnd = dateStr === endDate;
            const inRange = Boolean(
              effectiveStart &&
              effectiveEnd &&
              dateStr > effectiveStart &&
              dateStr < effectiveEnd
            );
            const isHoverPreview = Boolean(
              startDate &&
              !endDate &&
              hoverDate &&
              ((dateStr >= startDate && dateStr <= hoverDate) || (dateStr <= startDate && dateStr >= hoverDate))
            );

            let bgClass = 'text-gray-700 hover:bg-gray-100 rounded-md';
            if (isStart || isEnd) {
              bgClass = 'bg-gray-900 text-white font-semibold rounded-md shadow-xs';
            } else if (inRange || isHoverPreview) {
              bgClass = 'bg-gray-100 text-gray-900 font-medium rounded-none';
            }

            return (
              <div
                key={dateStr}
                className={`p-0.5 ${inRange || isHoverPreview ? 'bg-gray-100' : ''} ${
                  isStart && (endDate || hoverDate) ? 'rounded-l-md' : ''
                } ${isEnd ? 'rounded-r-md' : ''}`}
              >
                <button
                  type="button"
                  data-date={dateStr}
                  aria-label={dateStr}
                  aria-pressed={isStart || isEnd}
                  onClick={() => handleDayClick(dateStr)}
                  onMouseEnter={() => startDate && !endDate && setHoverDate(dateStr)}
                  onMouseLeave={() => setHoverDate(null)}
                  className={`w-full h-7 text-xs flex items-center justify-center transition-colors cursor-pointer ${bgClass}`}
                >
                  {dayNum}
                </button>
              </div>
            );
          })}
        </div>
      </div>
    );
  };

  return (
    <div className="m-period-wrap">
      <div className="m-period" role="group" aria-label="Период аналитики">
        {options.map(option => (
          <button
            key={option.value}
            type="button"
            ref={option.value === 'custom' ? trigger : undefined}
            aria-pressed={period === option.value}
            onClick={() => {
              if (option.value !== 'custom') {
                onSelect(option.value);
                return;
              }
              openModal();
            }}
          >
            {option.label}
          </button>
        ))}
      </div>

      <dialog
        ref={dialog}
        className="m-date-dialog"
        aria-labelledby="marketing-dates-title"
        onClose={() => trigger.current?.focus()}
      >
        <form onSubmit={apply} className="w-full">
          <div className="flex items-baseline justify-between mb-1">
            <h2 id="marketing-dates-title" className="text-base font-semibold text-gray-900 tracking-tight">
              Период аналитики
            </h2>
          </div>
          <p className="text-[11px] text-gray-400 mb-5">
            Даты в UTC, день окончания включён
          </p>

          {/* Top Summary Box */}
          <div className="grid grid-cols-2 gap-4 border border-gray-100 rounded-lg p-3 bg-gray-50/60 mb-6">
            <div>
              <div className="text-[10px] uppercase tracking-widest font-semibold text-gray-400 mb-1">
                Начало
              </div>
              <div
                data-testid="calendar-start-summary"
                aria-label="Начало периода"
                className="text-sm font-medium text-gray-900 tabular-nums"
              >
                {formatDisplayDate(startDate)}
              </div>
            </div>
            <div>
              <div className="text-[10px] uppercase tracking-widest font-semibold text-gray-400 mb-1">
                Окончание
              </div>
              <div
                data-testid="calendar-end-summary"
                aria-label="Окончание периода"
                className="text-sm font-medium text-gray-900 tabular-nums"
              >
                {formatDisplayDate(endDate)}
              </div>
            </div>
          </div>

          {/* Calendar Months View */}
          <div className="flex flex-col md:flex-row gap-6 mb-6">
            {renderMonthCalendar(month1.year, month1.month, false)}
            <div className="hidden md:block flex-1 min-w-0">
              {renderMonthCalendar(month2.year, month2.month, true)}
            </div>
          </div>

          {error && (
            <p role="alert" className="text-xs text-rose-600 mb-4 font-medium">
              {error}
            </p>
          )}

          <div className="flex justify-end gap-2.5 pt-4 border-t border-gray-100">
            <button
              type="button"
              className="px-4 py-2 text-xs font-semibold text-gray-700 bg-white hover:bg-gray-50 border border-gray-200 rounded-lg transition-colors cursor-pointer"
              onClick={close}
            >
              Отмена
            </button>
            <button
              type="submit"
              disabled={!startDate || !endDate}
              className="px-4 py-2 text-xs font-semibold text-white bg-gray-900 hover:bg-gray-800 disabled:opacity-40 disabled:cursor-not-allowed rounded-lg transition-colors cursor-pointer"
            >
              Применить
            </button>
          </div>
        </form>
      </dialog>
    </div>
  );
}
