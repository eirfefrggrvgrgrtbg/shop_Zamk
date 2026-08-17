import { ArrowDownIcon, ArrowUpIcon, MinusIcon } from 'lucide-react';
import { cn } from '../../lib/utils';

interface MetricCardProps {
  title: string;
  value: string;
  comparisonState?: 'positive' | 'negative' | 'unchanged' | 'new';
  changePercent?: number | null;
}

export function MetricCard({ title, value, comparisonState, changePercent }: MetricCardProps) {
  return (
    <div className="bg-white rounded-xl border border-gray-200 p-5 shadow-sm">
      <h3 className="text-sm font-medium text-gray-500 mb-1">{title}</h3>
      <div className="flex items-baseline gap-3">
        <span className="text-2xl font-bold text-gray-900">{value}</span>
        {comparisonState && comparisonState !== 'new' && changePercent !== null && changePercent !== undefined && (
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
            Новый результат
          </span>
        )}
      </div>
    </div>
  );
}
