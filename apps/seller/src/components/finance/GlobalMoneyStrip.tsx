import { useEffect, useState } from 'react';
import { getSellerBalance } from '@zamk/api-client/src/seller';
import type { SellerBalance } from '@zamk/api-client/src/types';
import { Wallet, Clock, Lock } from 'lucide-react';
import { Link } from 'react-router-dom';

const formatCents = (cents: number) => {
  return (cents / 100).toLocaleString('ru-RU', {
    style: 'currency',
    currency: 'RUB',
    minimumFractionDigits: 0,
  });
};

export function GlobalMoneyStrip() {
  const [balance, setBalance] = useState<SellerBalance | null>(null);

  useEffect(() => {
    getSellerBalance().then(setBalance).catch(console.error);
  }, []);

  if (!balance) return null;

  return (
    <div className="bg-white border-b border-gray-200">
      <div className="flex flex-wrap items-center justify-between px-4 py-3 max-w-7xl mx-auto gap-4">
        
        {/* Available for Payout */}
        <div className="flex items-center gap-3 min-w-[200px]">
          <div className="w-10 h-10 rounded-full bg-green-100 flex items-center justify-center shrink-0">
            <Wallet className="w-5 h-5 text-green-600" />
          </div>
          <div>
            <p className="text-xs font-medium text-gray-500 uppercase tracking-wider">Доступно к выплате</p>
            <p className="text-lg font-bold text-gray-900 leading-tight">
              {formatCents(balance.availableCents ?? (balance as any).available_cents ?? 0)}
            </p>
          </div>
        </div>

        {/* Scheduled Payouts */}
        <div className="flex items-center gap-3 min-w-[200px]">
          <div className="w-10 h-10 rounded-full bg-blue-100 flex items-center justify-center shrink-0">
            <Clock className="w-5 h-5 text-blue-600" />
          </div>
          <div>
            <p className="text-xs font-medium text-gray-500 uppercase tracking-wider">В пути (Payouts)</p>
            <p className="text-lg font-bold text-gray-900 leading-tight">
              {formatCents((balance as any).pendingPayoutsCents ?? (balance as any).pending_payouts_cents ?? 0)}
            </p>
          </div>
        </div>

        {/* Frozen (14-day hold) */}
        <div className="flex items-center gap-3 min-w-[200px]">
          <div className="w-10 h-10 rounded-full bg-gray-100 flex items-center justify-center shrink-0">
            <Lock className="w-5 h-5 text-gray-600" />
          </div>
          <div>
            <p className="text-xs font-medium text-gray-500 uppercase tracking-wider">Заморожено (14 дн.)</p>
            <p className="text-lg font-bold text-gray-900 leading-tight">
              {formatCents(balance.frozenCents ?? (balance as any).frozen_cents ?? 0)}
            </p>
          </div>
        </div>

        {/* Actions */}
        <div className="flex items-center ml-auto">
          <Link 
            to="/payouts"
            className="inline-flex items-center justify-center rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 bg-black text-white shadow hover:bg-gray-800 h-9 px-4 py-2"
          >
            Подробнее
          </Link>
        </div>

      </div>
    </div>
  );
}
