import { useState, useEffect } from 'react';
import { ArrowDownIcon, ArrowUpIcon, MinusIcon, HelpCircle } from 'lucide-react';
import { cn } from '../../lib/utils';

interface MetricCardProps {
  title: string;
  value: string;
  comparisonState?: 'positive' | 'negative' | 'unchanged' | 'new';
  changePercent?: number | null;
  explanation?: string;
  supportingText?: string;
}

export function MetricCard({
  title,
  value,
  comparisonState,
  changePercent,
  explanation,
  supportingText,
}: MetricCardProps) {
  const [isOpen, setIsOpen] = useState(false);

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape' && isOpen) {
        setIsOpen(false);
      }
    }
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen]);

  const tooltipId = `metric-hint-${title.toLowerCase().replace(/\s+/g, '-')}`;

  return (
    <div className="bg-white rounded-xl border border-gray-200 p-5 shadow-sm flex flex-col justify-between">
      <div>
        <div className="flex items-center justify-between gap-2 mb-1">
          <h3 className="text-sm font-medium text-gray-500 truncate">{title}</h3>
          {explanation && (
            <div
              className="relative inline-flex items-center"
              onMouseEnter={() => setIsOpen(true)}
              onMouseLeave={() => setIsOpen(false)}
            >
              <button
                type="button"
                onClick={() => setIsOpen((prev) => !prev)}
                onFocus={() => setIsOpen(true)}
                onBlur={() => setIsOpen(false)}
                className="text-gray-400 hover:text-gray-600 focus:text-gray-700 focus:outline-none rounded-full transition-colors p-0.5"
                aria-label={`Справка: ${title}`}
                aria-describedby={isOpen ? tooltipId : undefined}
                title={explanation}
              >
                <HelpCircle className="w-3.5 h-3.5" aria-hidden="true" />
              </button>
              {isOpen && (
                <div
                  id={tooltipId}
                  role="tooltip"
                  className="absolute right-0 bottom-full mb-2 w-64 p-2.5 bg-gray-900 text-white text-xs rounded-lg shadow-xl z-30 leading-normal"
                >
                  <p>{explanation}</p>
                  <div className="absolute right-2 top-full w-0 h-0 border-x-4 border-x-transparent border-t-4 border-t-gray-900" />
                </div>
              )}
            </div>
          )}
        </div>
        <div className="flex items-baseline gap-3 flex-wrap">
          <span className="text-2xl font-bold text-gray-900">{value}</span>
          {comparisonState && comparisonState !== 'new' && changePercent != null && !isNaN(changePercent) && isFinite(changePercent) && (
            <span
              className={cn(
                "flex items-center text-sm font-medium",
                comparisonState === 'positive' && "text-emerald-600",
                comparisonState === 'negative' && "text-red-600",
                comparisonState === 'unchanged' && "text-gray-500"
              )}
            >
              {comparisonState === 'positive' && <ArrowUpIcon className="w-3 h-3 mr-1" />}
              {comparisonState === 'negative' && <ArrowDownIcon className="w-3 h-3 mr-1" />}
              {comparisonState === 'unchanged' && <MinusIcon className="w-3 h-3 mr-1" />}
              {Math.abs(changePercent).toLocaleString('ru-RU', { maximumFractionDigits: 1 })}%
            </span>
          )}
          {comparisonState === 'new' && (
            <span className="text-sm font-medium text-blue-600 bg-blue-50 px-2 py-0.5 rounded">
              Новое
            </span>
          )}
        </div>
      </div>
      {supportingText && (
        <p className="text-xs text-gray-500 mt-2 font-medium">{supportingText}</p>
      )}
    </div>
  );
}
