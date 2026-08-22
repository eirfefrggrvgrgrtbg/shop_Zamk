import { useEffect, useState, useRef } from 'react';
import { getSellerBalance, getSellerPayouts } from '@zamk/api-client/src/seller';
import type { SellerBalance, PayoutBatchListResponse } from '@zamk/api-client/src/types';
import { Wallet, Clock, Lock, ChevronDown, FileText } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { cn } from '../../lib/utils';
import { format } from 'date-fns';
import { ru } from 'date-fns/locale';

const formatCents = (cents: number) => {
  return (cents / 100).toLocaleString('ru-RU', {
    style: 'currency',
    currency: 'RUB',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  });
};

export function GlobalMoneyStrip() {
  const [balance, setBalance] = useState<SellerBalance | null>(null);
  const [inTransitCents, setInTransitCents] = useState(0);
  const [isOpen, setIsOpen] = useState(false);
  const popoverRef = useRef<HTMLDivElement>(null);
  const navigate = useNavigate();

  useEffect(() => {
    Promise.all([
      getSellerBalance(),
      getSellerPayouts().catch(() => ({ items: [], totalCount: 0 } as PayoutBatchListResponse))
    ]).then(([bal, payouts]) => {
      setBalance(bal);
      const inTransit = payouts.items
        .filter(p => p.status === 'scheduled' || p.status === 'held' || p.status === 'processing')
        .reduce((sum, p) => sum + p.amountCents, 0);
      setInTransitCents(inTransit);
    }).catch(console.error);
  }, []);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (popoverRef.current && !popoverRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [popoverRef]);

  if (!balance) return null;

  const nextPayoutStr = balance.nextPayoutAt 
    ? format(new Date(balance.nextPayoutAt), 'd MMMM', { locale: ru })
    : null;

  return (
    <div className="hidden lg:flex relative mr-4" ref={popoverRef}>
      <button 
        onClick={() => setIsOpen(!isOpen)}
        className={cn(
          "flex items-center gap-6 bg-gray-50/80 px-4 py-2 rounded-xl border transition-colors",
          isOpen ? "border-gray-300 bg-gray-100/80" : "border-gray-100 hover:border-gray-200"
        )}
      >
        {/* Available for Payout */}
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-full bg-green-100 flex items-center justify-center shrink-0">
            <Wallet className="w-4 h-4 text-green-700" />
          </div>
          <div className="flex flex-col text-left">
            <span className="text-[10px] font-semibold text-gray-500 uppercase tracking-wider leading-none mb-1">Доступно к выплате</span>
            <span className="text-sm font-bold text-gray-900 leading-none">
              {formatCents(balance.availableCents ?? 0)}
            </span>
          </div>
        </div>

        <div className="w-px h-8 bg-gray-200"></div>

        {/* Frozen */}
        <div className="flex flex-col text-left">
          <span className="text-[10px] font-medium text-gray-500 uppercase tracking-wider leading-none mb-1 flex items-center gap-1">
            <Lock className="w-3 h-3" /> Заморожено
          </span>
          <span className="text-xs font-semibold text-gray-700 leading-none">
            {formatCents(balance.frozenCents ?? 0)}
          </span>
        </div>

        {/* Scheduled Payouts */}
        <div className="flex flex-col text-left">
          <span className="text-[10px] font-medium text-gray-500 uppercase tracking-wider leading-none mb-1 flex items-center gap-1">
            <Clock className="w-3 h-3" /> В пути
          </span>
          <span className="text-xs font-semibold text-gray-700 leading-none">
            {formatCents(inTransitCents)}
          </span>
        </div>

        <ChevronDown className={cn("w-4 h-4 text-gray-400 transition-transform ml-2", isOpen && "rotate-180")} />
      </button>

      {isOpen && (
        <div className="absolute right-0 top-full mt-2 w-[320px] bg-white rounded-2xl shadow-xl border border-gray-100 z-50 overflow-hidden">
          <div className="p-5 border-b border-gray-100">
            <div className="flex justify-between items-end mb-4">
              <div>
                <p className="text-xs text-gray-500 font-medium uppercase tracking-wider mb-1">Доступно к выплате</p>
                <p className="text-2xl font-bold text-gray-900">{formatCents(balance.availableCents ?? 0)}</p>
              </div>
            </div>
            
            <div className="space-y-3">
              <div className="flex justify-between items-center text-sm">
                <span className="text-gray-500 flex items-center gap-1.5"><Lock className="w-3.5 h-3.5" /> Заморожено</span>
                <span className="font-medium text-gray-900">{formatCents(balance.frozenCents ?? 0)}</span>
              </div>
              <div className="flex justify-between items-center text-sm">
                <span className="text-gray-500 flex items-center gap-1.5"><Clock className="w-3.5 h-3.5" /> В пути</span>
                <span className="font-medium text-gray-900">{formatCents(inTransitCents)}</span>
              </div>
              {nextPayoutStr && (
                <div className="flex justify-between items-center text-sm pt-2 border-t border-gray-100">
                  <span className="text-gray-500">Следующая выплата</span>
                  <span className="font-medium text-gray-900">~{nextPayoutStr}</span>
                </div>
              )}
            </div>
          </div>
          
          <div className="bg-gray-50 p-2">
            <button 
              onClick={() => { setIsOpen(false); navigate('/payouts'); }}
              className="w-full flex items-center gap-2 px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-200 rounded-xl transition-colors text-left"
            >
              <FileText className="w-4 h-4 text-gray-500" />
              История и детали выплат
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
