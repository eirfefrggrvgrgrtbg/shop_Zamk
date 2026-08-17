import { useMemo, useState } from 'react';
import { 
  AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer 
} from 'recharts';
import { TimeseriesBucketDTO } from '../../api/selleranalytics';
import { formatCents } from '../../lib/utils';
import { format, parseISO } from 'date-fns';
import { ru } from 'date-fns/locale';

interface TimeseriesChartProps {
  data: TimeseriesBucketDTO[];
}

type MetricType = 'sales' | 'orders' | 'units' | 'income';

export function TimeseriesChart({ data }: TimeseriesChartProps) {
  const [activeMetric, setActiveMetric] = useState<MetricType>('sales');

  const chartData = useMemo(() => {
    return data.map(d => ({
      ...d,
      displayDate: format(parseISO(d.date), 'd MMM', { locale: ru }),
      sales: d.grossSalesCents / 100,
      income: d.netCommercialEarningCents / 100,
    }));
  }, [data]);

  const metrics: { id: MetricType; label: string; format: (val: number) => string }[] = [
    { id: 'sales', label: 'Продажи', format: (val) => formatCents(val * 100) },
    { id: 'orders', label: 'Заказы', format: (val) => `${val} шт.` },
    { id: 'units', label: 'Продано', format: (val) => `${val} шт.` },
    { id: 'income', label: 'Доход', format: (val) => formatCents(val * 100) },
  ];

  const activeMetricData = metrics.find(m => m.id === activeMetric)!;
  
  const dataKeyMap: Record<MetricType, keyof typeof chartData[0]> = {
    sales: 'sales',
    orders: 'ordersCount',
    units: 'unitsSold',
    income: 'income'
  };

  const CustomTooltip = ({ active, payload, label }: any) => {
    if (active && payload && payload.length) {
      return (
        <div className="bg-white p-3 border border-gray-200 rounded shadow-md">
          <p className="text-sm text-gray-500 mb-1">{label}</p>
          <p className="font-semibold text-gray-900">
            {activeMetricData.format(payload[0].value)}
          </p>
        </div>
      );
    }
    return null;
  };

  return (
    <div className="bg-white rounded-xl border border-gray-200 p-6 shadow-sm">
      <div className="flex space-x-2 mb-6">
        {metrics.map(m => (
          <button
            key={m.id}
            onClick={() => setActiveMetric(m.id)}
            className={`px-3 py-1.5 text-sm font-medium rounded-md transition-colors ${
              activeMetric === m.id 
                ? 'bg-blue-50 text-blue-700' 
                : 'text-gray-600 hover:bg-gray-100'
            }`}
          >
            {m.label}
          </button>
        ))}
      </div>
      
      <div className="h-72 w-full">
        {chartData.length === 0 ? (
          <div className="h-full flex items-center justify-center text-gray-500">
            Нет данных за выбранный период
          </div>
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
              <defs>
                <linearGradient id="colorMetric" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.2}/>
                  <stop offset="95%" stopColor="#3b82f6" stopOpacity={0}/>
                </linearGradient>
              </defs>
              <XAxis 
                dataKey="displayDate" 
                axisLine={false} 
                tickLine={false} 
                tick={{ fill: '#6b7280', fontSize: 12 }} 
                dy={10}
              />
              <YAxis 
                axisLine={false} 
                tickLine={false} 
                tick={{ fill: '#6b7280', fontSize: 12 }}
                tickFormatter={(val) => {
                  if (activeMetric === 'sales' || activeMetric === 'income') {
                    return val >= 1000 ? `${(val / 1000).toFixed(0)}k ₽` : `${val} ₽`;
                  }
                  return val.toString();
                }}
              />
              <CartesianGrid vertical={false} stroke="#e5e7eb" strokeDasharray="3 3" />
              <Tooltip content={<CustomTooltip />} />
              <Area 
                type="monotone" 
                dataKey={dataKeyMap[activeMetric]} 
                stroke="#3b82f6" 
                strokeWidth={2}
                fillOpacity={1} 
                fill="url(#colorMetric)" 
                activeDot={{ r: 6, strokeWidth: 0, fill: '#3b82f6' }}
              />
            </AreaChart>
          </ResponsiveContainer>
        )}
      </div>
    </div>
  );
}
