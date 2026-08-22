import { useState, useEffect, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import { cn } from '../../lib/utils';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import { format, addMonths, subMonths, startOfMonth, endOfMonth, eachDayOfInterval, isSameDay, isWithinInterval } from 'date-fns';
import { ru } from 'date-fns/locale';

type PeriodOption = 'today' | '7d' | '30d' | '90d' | 'custom';

// Simple date parser for YYYY-MM-DD local strings
const parseDateOnly = (dateStr: string) => {
  const [y, m, d] = dateStr.split('-').map(Number);
  return new Date(y, m - 1, d);
};

const formatDateOnly = (d: Date) => format(d, 'yyyy-MM-dd');
const formatDisplay = (d: Date) => format(d, 'dd.MM.yyyy');

export function AnalyticsPeriodPicker({ from, to }: { from: string, to: string }) {
  const [searchParams, setSearchParams] = useSearchParams();
  const currentPeriod = (searchParams.get('period') || '30d') as PeriodOption;
  
  const [isOpen, setIsOpen] = useState(false);
  
  // Selection state
  const [rangeStart, setRangeStart] = useState<Date | null>(null);
  const [rangeEnd, setRangeEnd] = useState<Date | null>(null);
  const [currentMonth, setCurrentMonth] = useState(startOfMonth(new Date()));
  
  const popoverRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (isOpen) {
      if (from && to) {
        const fromDateStr = from.split('T')[0];
        const toDateStr = to.split('T')[0];
        
        const fDate = parseDateOnly(fromDateStr);
        // We subtract 1 day from the parsed to date to get the inclusive UI end date
        const tDateObj = parseDateOnly(toDateStr);
        tDateObj.setDate(tDateObj.getDate() - 1);

        setRangeStart(fDate);
        setRangeEnd(tDateObj);
        setCurrentMonth(startOfMonth(fDate));
      } else {
        setRangeStart(null);
        setRangeEnd(null);
        setCurrentMonth(startOfMonth(new Date()));
      }
    }
  }, [isOpen, from, to]);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (popoverRef.current && !popoverRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [popoverRef]);

  const handlePeriodChange = (val: PeriodOption) => {
    if (val === 'custom') {
      setIsOpen(!isOpen);
      return;
    }
    setIsOpen(false);
    const newParams = new URLSearchParams(searchParams);
    newParams.set('period', val);
    newParams.delete('from');
    newParams.delete('to');
    setSearchParams(newParams);
  };

  const handleApply = () => {
    if (!rangeStart || !rangeEnd) return;
    
    let start = rangeStart;
    let end = rangeEnd;
    if (start > end) {
      start = rangeEnd;
      end = rangeStart;
    }

    const startStr = formatDateOnly(start);
    // Add 1 calendar day to 'end' for exclusive backend semantics
    const exclusiveEnd = new Date(end);
    exclusiveEnd.setDate(exclusiveEnd.getDate() + 1);
    const endStr = formatDateOnly(exclusiveEnd);
    
    const newParams = new URLSearchParams(searchParams);
    newParams.set('period', 'custom');
    newParams.set('from', `${startStr}T00:00:00+03:00`);
    newParams.set('to', `${endStr}T00:00:00+03:00`);
    setSearchParams(newParams);
    setIsOpen(false);
  };

  const handleReset = () => {
    setIsOpen(false);
    handlePeriodChange('30d');
  };

  const handleDayClick = (day: Date) => {
    if (!rangeStart || (rangeStart && rangeEnd)) {
      setRangeStart(day);
      setRangeEnd(null);
    } else {
      if (day < rangeStart) {
        setRangeStart(day);
        setRangeEnd(null);
      } else {
        setRangeEnd(day);
      }
    }
  };

  const days = eachDayOfInterval({
    start: startOfMonth(currentMonth),
    end: endOfMonth(currentMonth)
  });

  const firstDayOfWeek = days[0].getDay();
  const paddingOffset = firstDayOfWeek === 0 ? 6 : firstDayOfWeek - 1;
  const paddingDays = Array.from({ length: paddingOffset }, (_, i) => i);

  let visibleText = '';
  if (from && to) {
    const fDate = parseDateOnly(from.split('T')[0]);
    const tDateObj = parseDateOnly(to.split('T')[0]);
    tDateObj.setDate(tDateObj.getDate() - 1);
    if (isSameDay(fDate, tDateObj)) {
      visibleText = formatDisplay(fDate);
    } else {
      visibleText = `${formatDisplay(fDate)} — ${formatDisplay(tDateObj)}`;
    }
  }

  return (
    <div className="relative flex items-center gap-4" ref={popoverRef}>
      <button 
        onClick={() => setIsOpen(!isOpen)}
        className="text-sm font-medium text-gray-700 hover:text-gray-900 px-3 py-2 rounded-md hover:bg-gray-100 transition-colors"
      >
        {visibleText}
      </button>
      <div className="flex flex-nowrap items-center space-x-1 bg-gray-100 p-1 rounded-lg overflow-x-auto">
        {(['today', '7d', '30d', '90d', 'custom'] as PeriodOption[]).map((p) => {
          const isActive = currentPeriod === p;
          const label = p === 'today' ? 'Сегодня' : p === '7d' ? '7 дней' : p === '30d' ? '30 дней' : p === '90d' ? '90 дней' : 'Произвольный';
          return (
            <button
              key={p}
              onClick={() => handlePeriodChange(p)}
              className={cn(
                "px-3 py-1.5 sm:px-4 sm:py-2 text-sm font-medium rounded-md transition-colors whitespace-nowrap",
                isActive ? "bg-white text-gray-900 shadow-sm" : "text-gray-600 hover:text-gray-900"
              )}
            >
              {label}
            </button>
          )
        })}
      </div>

      {isOpen && (
        <div className="absolute right-0 top-full mt-2 w-[340px] bg-white rounded-lg shadow-xl border border-gray-200 z-50 p-4">
          <div className="flex justify-between items-center mb-4">
            <button onClick={() => setCurrentMonth(subMonths(currentMonth, 1))} className="p-1 hover:bg-gray-100 rounded">
              <ChevronLeft className="w-5 h-5 text-gray-600" />
            </button>
            <div className="font-semibold text-gray-900 capitalize">
              {format(currentMonth, 'LLLL yyyy', { locale: ru })}
            </div>
            <button onClick={() => setCurrentMonth(addMonths(currentMonth, 1))} className="p-1 hover:bg-gray-100 rounded">
              <ChevronRight className="w-5 h-5 text-gray-600" />
            </button>
          </div>
          
          <div className="grid grid-cols-7 gap-1 text-center mb-2">
            {['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'].map(d => (
              <div key={d} className="text-xs font-medium text-gray-500">{d}</div>
            ))}
          </div>
          
          <div className="grid grid-cols-7 gap-1">
            {paddingDays.map(i => <div key={`pad-${i}`} />)}
            {days.map(day => {
              const isSelected = (rangeStart && isSameDay(day, rangeStart)) || (rangeEnd && isSameDay(day, rangeEnd));
              const isInRange = rangeStart && rangeEnd && isWithinInterval(day, { start: rangeStart, end: rangeEnd });
              
              return (
                <button
                  key={day.toISOString()}
                  onClick={() => handleDayClick(day)}
                  className={cn(
                    "h-8 rounded-md text-sm flex items-center justify-center transition-colors",
                    isSelected ? "bg-black text-white font-bold hover:bg-gray-800" :
                    isInRange ? "bg-gray-100 text-gray-900 hover:bg-gray-200" :
                    "text-gray-900 hover:bg-gray-100"
                  )}
                >
                  {format(day, 'd')}
                </button>
              );
            })}
          </div>

          <div className="mt-4 pt-4 border-t border-gray-100 flex justify-between items-center text-sm text-gray-700">
            <div>
              <span className="font-medium text-gray-500">От:</span> {rangeStart ? formatDisplay(rangeStart) : '—'}
            </div>
            <div>
              <span className="font-medium text-gray-500">До:</span> {rangeEnd ? formatDisplay(rangeEnd) : '—'}
            </div>
          </div>

          <div className="mt-4 flex items-center gap-2">
            <button
              onClick={handleReset}
              className="flex-1 px-3 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-gray-200"
            >
              Сбросить
            </button>
            <button
              onClick={handleApply}
              disabled={!rangeStart || !rangeEnd}
              className="flex-1 px-3 py-2 text-sm font-medium text-white bg-black rounded-md hover:bg-gray-800 disabled:opacity-50 disabled:cursor-not-allowed focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-gray-900"
            >
              Применить
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
