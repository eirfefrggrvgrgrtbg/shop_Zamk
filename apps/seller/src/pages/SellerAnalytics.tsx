import { useMemo } from 'react';
import { useSearchParams } from 'react-router-dom';
import { formatInTimeZone } from 'date-fns-tz';
import { OverviewTab } from '../components/analytics/OverviewTab';
import { ProductsTab } from '../components/analytics/ProductsTab';
import { InventoryTab } from '../components/analytics/InventoryTab';
import { FinanceTab } from '../components/analytics/FinanceTab';
import { cn } from '../lib/utils';

type PeriodOption = 'today' | '7d' | '30d' | '90d' | 'custom';
type TabOption = 'overview' | 'products' | 'inventory' | 'finance';

const TIMEZONE = 'Europe/Moscow';

export function SellerAnalytics() {
  const [searchParams, setSearchParams] = useSearchParams();
  
  const currentTab = (searchParams.get('tab') as TabOption) || 'overview';

  const { from, to } = useMemo(() => {
    const now = new Date();
    
    const getBounds = (daysOffset: number) => {
      const d = new Date(now.getTime() + daysOffset * 24 * 60 * 60 * 1000);
      const ymd = formatInTimeZone(d, TIMEZONE, 'yyyy-MM-dd');
      // Moscow is fixed UTC+3
      return {
        start: `${ymd}T00:00:00+03:00`,
        end: `${ymd}T23:59:59.999+03:00`
      };
    };

    let fromStr: string;
    let toStr: string = getBounds(0).end;

    const activePeriod = searchParams.get('period') || '30d';

    if (activePeriod === 'today') {
      fromStr = getBounds(0).start;
    } else if (activePeriod === '7d') {
      fromStr = getBounds(-7).start;
    } else if (activePeriod === '30d') {
      fromStr = getBounds(-30).start;
    } else if (activePeriod === '90d') {
      fromStr = getBounds(-90).start;
    } else {
      const pFrom = searchParams.get('from');
      const pTo = searchParams.get('to');
      if (pFrom && pTo) {
        fromStr = pFrom;
        toStr = pTo;
      } else {
        fromStr = getBounds(-30).start;
      }
    }

    return { 
      from: fromStr, 
      to: toStr 
    };
  }, [searchParams]);

  const handlePeriodChange = (val: PeriodOption) => {
    const newParams = new URLSearchParams(searchParams);
    newParams.set('period', val);
    if (val !== 'custom') {
      newParams.delete('from');
      newParams.delete('to');
    }
    setSearchParams(newParams);
  };

  const handleTabChange = (val: TabOption) => {
    const newParams = new URLSearchParams(searchParams);
    newParams.set('tab', val);
    setSearchParams(newParams);
  };

  const tabs: { id: TabOption; label: string }[] = [
    { id: 'overview', label: 'Обзор' },
    { id: 'products', label: 'Товары' },
    { id: 'inventory', label: 'Остатки' },
    { id: 'finance', label: 'Финансы и возвраты' },
  ];

  return (
    <div className="p-8 max-w-7xl mx-auto space-y-6">
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <h1 className="text-2xl font-bold text-gray-900">Аналитика</h1>
        
        <div className="flex flex-col sm:flex-row items-end sm:items-center space-y-2 sm:space-y-0 sm:space-x-4">
          {(searchParams.get('period') || '30d') === 'custom' && (
            <div className="flex items-center space-x-2 bg-white border border-gray-200 rounded-lg p-1 text-sm">
              <input 
                type="date" 
                className="bg-transparent outline-none px-2 text-gray-700" 
                value={from.split('T')[0]} 
                onChange={(e) => {
                  const newParams = new URLSearchParams(searchParams);
                  newParams.set('from', `${e.target.value}T00:00:00+03:00`);
                  setSearchParams(newParams);
                }}
              />
              <span className="text-gray-400">—</span>
              <input 
                type="date" 
                className="bg-transparent outline-none px-2 text-gray-700" 
                value={to.split('T')[0]} 
                onChange={(e) => {
                  const newParams = new URLSearchParams(searchParams);
                  newParams.set('to', `${e.target.value}T23:59:59.999+03:00`);
                  setSearchParams(newParams);
                }}
              />
            </div>
          )}
          <div className="flex items-center space-x-1 bg-gray-100 p-1 rounded-lg overflow-x-auto max-w-full">
            {(['today', '7d', '30d', '90d', 'custom'] as PeriodOption[]).map((p) => {
              const isActive = (searchParams.get('period') || '30d') === p;
              return (
                <button
                  key={p}
                  onClick={() => handlePeriodChange(p)}
                  className={cn(
                    "px-3 py-1.5 sm:px-4 sm:py-2 text-sm font-medium rounded-md transition-colors whitespace-nowrap",
                    isActive ? "bg-white text-gray-900 shadow-sm" : "text-gray-600 hover:text-gray-900"
                  )}
                >
                  {p === 'today' ? 'Сегодня' : p === '7d' ? '7 дней' : p === '30d' ? '30 дней' : p === '90d' ? '90 дней' : 'Произвольный'}
                </button>
              )
            })}
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="border-b border-gray-200">
        <nav className="-mb-px flex space-x-8">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => handleTabChange(tab.id)}
              className={cn(
                "whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm transition-colors",
                currentTab === tab.id
                  ? "border-blue-500 text-blue-600"
                  : "border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300"
              )}
            >
              {tab.label}
            </button>
          ))}
        </nav>
      </div>

      {/* Tab Content */}
      <div className="py-4">
        {currentTab === 'overview' && <OverviewTab from={from} to={to} />}
        {currentTab === 'products' && <ProductsTab from={from} to={to} />}
        {currentTab === 'inventory' && <InventoryTab from={from} to={to} />}
        {currentTab === 'finance' && <FinanceTab from={from} to={to} />}
      </div>
    </div>
  );
}
