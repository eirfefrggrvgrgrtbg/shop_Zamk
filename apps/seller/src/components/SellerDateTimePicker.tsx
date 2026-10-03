import { useState, useEffect, useRef } from 'react';
import {
  format,
  startOfMonth,
  endOfMonth,
  eachDayOfInterval,
  isSameDay,
  addMonths,
  subMonths,
} from 'date-fns';
import { ru } from 'date-fns/locale';
import { Calendar, Clock, ChevronLeft, ChevronRight, X } from 'lucide-react';
import { cn } from '../lib/utils';

export interface SellerDateTimePickerProps {
  value: string;
  onChange: (value: string) => void;
  label?: string;
  placeholder?: string;
  defaultTime?: string; // e.g. "00:00" or "23:59"
  align?: 'left' | 'right';
  'data-testid'?: string;
  disabled?: boolean;
}

// Parses string representation (e.g. "2026-10-01T14:30" or ISO string)
function parseDateParts(val: string): { date: Date; hours: string; minutes: string } | null {
  if (!val) return null;
  const parts = val.split('T');
  if (parts.length === 2) {
    const [datePart, timePart] = parts;
    const [y, m, d] = datePart.split('-').map(Number);
    const timeClean = timePart.replace('Z', '').split(':');
    const hh = timeClean[0] ? timeClean[0].padStart(2, '0') : '00';
    const mm = timeClean[1] ? timeClean[1].padStart(2, '0') : '00';
    const parsed = new Date(y, m - 1, d, Number(hh), Number(mm));
    if (!isNaN(parsed.getTime())) {
      return { date: parsed, hours: hh, minutes: mm };
    }
  }
  const fallback = new Date(val);
  if (!isNaN(fallback.getTime())) {
    return {
      date: fallback,
      hours: String(fallback.getHours()).padStart(2, '0'),
      minutes: String(fallback.getMinutes()).padStart(2, '0'),
    };
  }
  return null;
}

export function SellerDateTimePicker({
  value,
  onChange,
  placeholder = 'Выберите дату и время',
  defaultTime = '00:00',
  align = 'left',
  'data-testid': dataTestId,
  disabled = false,
}: SellerDateTimePickerProps) {
  const [isOpen, setIsOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  // Temporary selection inside popover
  const [selectedDate, setSelectedDate] = useState<Date | null>(null);
  const [viewMonth, setViewMonth] = useState<Date>(startOfMonth(new Date()));
  const [hours, setHours] = useState('00');
  const [minutes, setMinutes] = useState('00');

  // Sync internal state when popover opens or value changes
  useEffect(() => {
    if (isOpen) {
      const parsed = parseDateParts(value);
      if (parsed) {
        setSelectedDate(parsed.date);
        setViewMonth(startOfMonth(parsed.date));
        setHours(parsed.hours);
        setMinutes(parsed.minutes);
      } else {
        setSelectedDate(null);
        setViewMonth(startOfMonth(new Date()));
        const [defH, defM] = defaultTime.split(':');
        setHours(defH || '00');
        setMinutes(defM || '00');
      }
    }
  }, [isOpen, value, defaultTime]);

  // Click outside to close without applying unconfirmed changes
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    }
    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside);
      return () => document.removeEventListener('mousedown', handleClickOutside);
    }
  }, [isOpen]);

  // Escape to close popover without bubbling to parent modal/drawer
  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape' && isOpen) {
        event.stopPropagation();
        setIsOpen(false);
      }
    }
    window.addEventListener('keydown', handleKeyDown, true);
    return () => window.removeEventListener('keydown', handleKeyDown, true);
  }, [isOpen]);

  // Calendar calculations
  const monthStart = startOfMonth(viewMonth);
  const monthEnd = endOfMonth(viewMonth);
  const days = eachDayOfInterval({ start: monthStart, end: monthEnd });

  // Monday-first weekday offset
  const firstDayOfWeek = days[0].getDay();
  const paddingOffset = firstDayOfWeek === 0 ? 6 : firstDayOfWeek - 1;
  const paddingDays = Array.from({ length: paddingOffset }, (_, i) => i);

  const rawMonthTitle = format(viewMonth, 'LLLL yyyy', { locale: ru });
  const monthTitle = rawMonthTitle.charAt(0).toUpperCase() + rawMonthTitle.slice(1);

  const handleDayClick = (day: Date) => {
    setSelectedDate(day);
  };

  const handleApply = () => {
    if (!selectedDate) {
      onChange('');
      setIsOpen(false);
      return;
    }
    const y = selectedDate.getFullYear();
    const m = String(selectedDate.getMonth() + 1).padStart(2, '0');
    const d = String(selectedDate.getDate()).padStart(2, '0');
    const hh = String(Math.min(23, Math.max(0, parseInt(hours, 10) || 0))).padStart(2, '0');
    const mm = String(Math.min(59, Math.max(0, parseInt(minutes, 10) || 0))).padStart(2, '0');
    onChange(`${y}-${m}-${d}T${hh}:${mm}`);
    setIsOpen(false);
  };

  const handleClear = () => {
    setSelectedDate(null);
    onChange('');
    setIsOpen(false);
  };

  const parsedValue = parseDateParts(value);
  const displayFormatted = parsedValue
    ? `${String(parsedValue.date.getDate()).padStart(2, '0')}.${String(
        parsedValue.date.getMonth() + 1
      ).padStart(2, '0')}.${parsedValue.date.getFullYear()} ${parsedValue.hours}:${parsedValue.minutes}`
    : '';

  return (
    <div className="relative w-full" ref={containerRef}>
      {/* Trigger Button */}
      <button
        type="button"
        disabled={disabled}
        data-testid={dataTestId}
        aria-haspopup="dialog"
        aria-expanded={isOpen}
        onClick={() => !disabled && setIsOpen(!isOpen)}
        className={cn(
          "w-full flex items-center justify-between px-3 py-1.5 border rounded-lg text-xs transition-colors text-left",
          isOpen ? "border-black ring-1 ring-black bg-white" : "border-gray-300 hover:border-gray-400 bg-white",
          disabled && "bg-gray-50 text-gray-400 cursor-not-allowed border-gray-200"
        )}
      >
        <span className={displayFormatted ? "text-gray-900 font-medium" : "text-gray-400"}>
          {displayFormatted || placeholder}
        </span>
        <div className="flex items-center gap-1">
          {displayFormatted && !disabled && (
            <span
              role="button"
              tabIndex={0}
              data-testid={dataTestId ? `${dataTestId}-trigger-clear` : undefined}
              onClick={(e) => {
                e.stopPropagation();
                handleClear();
              }}
              className="p-0.5 hover:bg-gray-100 rounded-full text-gray-400 hover:text-gray-600 transition-colors"
              aria-label="Очистить дату"
            >
              <X className="w-3 h-3" />
            </span>
          )}
          <Calendar className="w-4 h-4 text-gray-400 shrink-0" />
        </div>
      </button>

      {/* Popover */}
      {isOpen && (
        <div
          role="dialog"
          aria-label="Выбор даты и времени"
          data-testid={dataTestId ? `${dataTestId}-popover` : 'date-time-popover'}
          className={cn(
            "absolute z-50 mt-1.5 w-[310px] bg-white rounded-xl shadow-2xl border border-gray-200 p-3.5",
            align === 'right' ? 'right-0' : 'left-0'
          )}
        >
          {/* Month / Year Navigation */}
          <div className="flex justify-between items-center mb-3">
            <button
              type="button"
              data-testid={dataTestId ? `${dataTestId}-prev-month` : 'date-picker-prev-month'}
              onClick={() => setViewMonth(subMonths(viewMonth, 1))}
              className="p-1 hover:bg-gray-100 rounded-md text-gray-600 hover:text-gray-900 transition-colors"
              aria-label="Предыдущий месяц"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>
            <div
              className="text-xs font-semibold text-gray-900"
              data-testid={dataTestId ? `${dataTestId}-month-title` : undefined}
            >
              {monthTitle}
            </div>
            <button
              type="button"
              data-testid={dataTestId ? `${dataTestId}-next-month` : 'date-picker-next-month'}
              onClick={() => setViewMonth(addMonths(viewMonth, 1))}
              className="p-1 hover:bg-gray-100 rounded-md text-gray-600 hover:text-gray-900 transition-colors"
              aria-label="Следующий месяц"
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>

          {/* Weekday Row (Mon-Sun) */}
          <div className="grid grid-cols-7 gap-1 text-center mb-1.5">
            {['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'].map((d) => (
              <div key={d} className="text-[11px] font-medium text-gray-400 py-0.5">
                {d}
              </div>
            ))}
          </div>

          {/* Day Grid */}
          <div className="grid grid-cols-7 gap-1">
            {paddingDays.map((i) => (
              <div key={`pad-${i}`} className="h-8 w-8" />
            ))}
            {days.map((day) => {
              const isSelected = selectedDate && isSameDay(day, selectedDate);
              const isToday = isSameDay(day, new Date());
              const dayIso = format(day, 'yyyy-MM-dd');

              return (
                <button
                  type="button"
                  key={day.toISOString()}
                  data-testid={dataTestId ? `${dataTestId}-day-${dayIso}` : `day-${dayIso}`}
                  onClick={() => handleDayClick(day)}
                  className={cn(
                    "h-8 w-8 rounded-lg text-xs flex items-center justify-center transition-all",
                    isSelected
                      ? "bg-black text-white font-bold shadow-xs hover:bg-gray-800"
                      : isToday
                      ? "border border-black font-semibold text-gray-900 hover:bg-gray-100"
                      : "text-gray-900 hover:bg-gray-100"
                  )}
                  aria-label={format(day, 'dd MMMM yyyy', { locale: ru })}
                >
                  {format(day, 'd')}
                </button>
              );
            })}
          </div>

          {/* Time Section */}
          <div className="mt-3 pt-3 border-t border-gray-100">
            <div className="flex items-center justify-between mb-2">
              <div className="flex items-center gap-1.5 text-xs font-medium text-gray-700">
                <Clock className="w-3.5 h-3.5 text-gray-500" />
                <span>Время (24ч)</span>
              </div>
              <div className="flex items-center gap-1 text-[10px]">
                {['00:00', '12:00', '23:59'].map((preset) => (
                  <button
                    key={preset}
                    type="button"
                    data-testid={dataTestId ? `${dataTestId}-preset-${preset}` : undefined}
                    onClick={() => {
                      const [h, m] = preset.split(':');
                      setHours(h);
                      setMinutes(m);
                    }}
                    className="px-1.5 py-0.5 rounded border border-gray-200 text-gray-600 hover:bg-gray-50 transition-colors"
                  >
                    {preset}
                  </button>
                ))}
              </div>
            </div>

            <div className="flex items-center justify-center gap-2">
              <div className="flex items-center gap-1.5">
                <input
                  type="text"
                  inputMode="numeric"
                  maxLength={2}
                  data-testid={dataTestId ? `${dataTestId}-hours` : 'date-picker-hours'}
                  value={hours}
                  onChange={(e) => {
                    const clean = e.target.value.replace(/\D/g, '').slice(0, 2);
                    setHours(clean);
                  }}
                  onBlur={() => {
                    const val = parseInt(hours, 10);
                    if (isNaN(val) || val < 0) setHours('00');
                    else if (val > 23) setHours('23');
                    else setHours(String(val).padStart(2, '0'));
                  }}
                  className="w-12 h-7 text-center border border-gray-300 rounded-md text-xs font-mono font-medium focus:outline-none focus:ring-1 focus:ring-black"
                  aria-label="Часы"
                />
                <span className="text-gray-400 font-bold text-xs">:</span>
                <input
                  type="text"
                  inputMode="numeric"
                  maxLength={2}
                  data-testid={dataTestId ? `${dataTestId}-minutes` : 'date-picker-minutes'}
                  value={minutes}
                  onChange={(e) => {
                    const clean = e.target.value.replace(/\D/g, '').slice(0, 2);
                    setMinutes(clean);
                  }}
                  onBlur={() => {
                    const val = parseInt(minutes, 10);
                    if (isNaN(val) || val < 0) setMinutes('00');
                    else if (val > 59) setMinutes('59');
                    else setMinutes(String(val).padStart(2, '0'));
                  }}
                  className="w-12 h-7 text-center border border-gray-300 rounded-md text-xs font-mono font-medium focus:outline-none focus:ring-1 focus:ring-black"
                  aria-label="Минуты"
                />
              </div>
            </div>
          </div>

          {/* Action Buttons */}
          <div className="mt-3 pt-3 border-t border-gray-100 flex items-center gap-2">
            <button
              type="button"
              data-testid={dataTestId ? `${dataTestId}-clear` : 'date-picker-clear'}
              onClick={handleClear}
              className="flex-1 px-3 py-1.5 text-xs font-medium text-gray-700 bg-white border border-gray-300 rounded-lg hover:bg-gray-50 focus:outline-none transition-colors"
            >
              Очистить
            </button>
            <button
              type="button"
              data-testid={dataTestId ? `${dataTestId}-apply` : 'date-picker-apply'}
              onClick={handleApply}
              className="flex-1 px-3 py-1.5 text-xs font-medium text-white bg-black rounded-lg hover:bg-gray-800 focus:outline-none transition-colors shadow-xs"
            >
              Применить
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
