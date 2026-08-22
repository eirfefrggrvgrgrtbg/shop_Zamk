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
    <div className="hidden lg:flex items-center gap-6 mr-6">
      
      {/* Available for Payout */}
      <div className="flex items-center gap-2">
        <div className="w-8 h-8 rounded-full bg-green-100 flex items-center justify-center shrink-0">
          <Wallet className="w-4 h-4 text-green-600" />
        </div>
        <div className="flex flex-col">
          <span className="text-[10px] font-medium text-gray-500 uppercase tracking-wider leading-none mb-1">Доступно</span>
          <span className="text-sm font-bold text-gray-900 leading-none">
            {formatCents(balance.availableCents ?? (balance as any).available_cents ?? 0)}
          </span>
        </div>
      </div>

      {/* Scheduled Payouts */}
      <div className="flex items-center gap-2">
        <div className="w-8 h-8 rounded-full bg-blue-100 flex items-center justify-center shrink-0">
          <Clock className="w-4 h-4 text-blue-600" />
        </div>
        <div className="flex flex-col">
          <span className="text-[10px] font-medium text-gray-500 uppercase tracking-wider leading-none mb-1">В пути</span>
          <span className="text-sm font-bold text-gray-900 leading-none">
            {formatCents((balance as any).pendingPayoutsCents ?? (balance as any).pending_payouts_cents ?? 0)}
          </span>
        </div>
      </div>

      {/* Frozen (14-day hold) */}
      <div className="flex items-center gap-2">
        <div className="w-8 h-8 rounded-full bg-gray-100 flex items-center justify-center shrink-0">
          <Lock className="w-4 h-4 text-gray-600" />
        </div>
        <div className="flex flex-col">
          <span className="text-[10px] font-medium text-gray-500 uppercase tracking-wider leading-none mb-1">Заморожено</span>
          <span className="text-sm font-bold text-gray-900 leading-none">
            {formatCents(balance.frozenCents ?? (balance as any).frozen_cents ?? 0)}
          </span>
        </div>
      </div>

      {/* Actions */}
      <div className="flex items-center">
        <Link 
          to="/payouts"
          className="text-xs font-medium text-blue-600 hover:text-blue-800 transition-colors bg-blue-50 px-3 py-1.5 rounded-md"
        >
          Подробнее
        </Link>
      </div>

    </div>
  );
}
