import { useState, useEffect, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import { cn } from '../../lib/utils';
import { Calendar } from 'lucide-react';

type PeriodOption = 'today' | '7d' | '30d' | '90d' | 'custom';

export function AnalyticsPeriodPicker() {
  const [searchParams, setSearchParams] = useSearchParams();
  const currentPeriod = (searchParams.get('period') || '30d') as PeriodOption;
  
  const pFrom = searchParams.get('from');
  const pTo = searchParams.get('to');
  
  const [isOpen, setIsOpen] = useState(false);
  const [tempFrom, setTempFrom] = useState('');
  const [tempTo, setTempTo] = useState('');
  
  const popoverRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (isOpen) {
      setTempFrom(pFrom ? pFrom.split('T')[0] : '');
      setTempTo(pTo ? pTo.split('T')[0] : '');
    }
  }, [isOpen, pFrom, pTo]);

  // Close on outside click
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (popoverRef.current && !popoverRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
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
    if (!tempFrom || !tempTo) return;
    if (new Date(tempFrom) > new Date(tempTo)) return;
    
    const newParams = new URLSearchParams(searchParams);
    newParams.set('period', 'custom');
    newParams.set('from', `${tempFrom}T00:00:00+03:00`);
    newParams.set('to', `${tempTo}T23:59:59.999+03:00`);
    setSearchParams(newParams);
    setIsOpen(false);
  };

  const handleReset = () => {
    setIsOpen(false);
    handlePeriodChange('30d');
  };

  const isValid = tempFrom && tempTo && new Date(tempFrom) <= new Date(tempTo);

  return (
    <div className="relative flex items-center" ref={popoverRef}>
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
        <div className="absolute right-0 top-full mt-2 w-72 bg-white rounded-lg shadow-lg border border-gray-200 z-50 p-4">
          <div className="space-y-4">
            <div>
              <label className="block text-xs font-medium text-gray-700 mb-1">От</label>
              <div className="relative">
                <Calendar className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
                <input 
                  type="date"
                  value={tempFrom}
                  onChange={(e) => setTempFrom(e.target.value)}
                  className="w-full pl-9 pr-3 py-2 bg-gray-50 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-700 mb-1">До</label>
              <div className="relative">
                <Calendar className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
                <input 
                  type="date"
                  value={tempTo}
                  onChange={(e) => setTempTo(e.target.value)}
                  className="w-full pl-9 pr-3 py-2 bg-gray-50 border border-gray-300 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>
            </div>
            
            <div className="flex items-center gap-2 pt-2 border-t border-gray-100">
              <button
                onClick={handleReset}
                className="flex-1 px-3 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
              >
                Сбросить
              </button>
              <button
                onClick={handleApply}
                disabled={!isValid}
                className="flex-1 px-3 py-2 text-sm font-medium text-white bg-black rounded-md hover:bg-gray-800 disabled:opacity-50 disabled:cursor-not-allowed focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-gray-900"
              >
                Применить
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
