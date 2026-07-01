import { Link } from 'react-router-dom';
import { Gavel } from 'lucide-react';
import type { AuctionLot, AuctionEvent } from '@zamk/api-client';
import { formatPrice } from '../../lib/utils';
import { Countdown } from './Countdown';

interface AuctionLotCardProps {
  lot: AuctionLot;
  auction?: AuctionEvent;
}

export function AuctionLotCard({ lot, auction }: AuctionLotCardProps) {
  // Use auction endsAt if lot is active, otherwise rely on auction status
  const endsAt = auction?.endsAt || new Date(Date.now() + 86400000).toISOString();
  
  const currentPrice = lot.currentBidCents || lot.startPriceCents;
  const isEnded = auction?.status === 'ended' || lot.status === 'ended_no_bids' || lot.status === 'won_pending_payment' || lot.status === 'paid';

  return (
    <div className="group relative flex flex-col items-center w-full transition-all duration-500 hover:-translate-y-2">
      <div className="relative w-full bg-white/5 dark:bg-zinc-800/5 backdrop-blur-xl p-3 pb-6 shadow-sm hover:shadow-lg dark:shadow-none rounded-2xl border border-white/20 dark:border-white/5 transition-shadow">
        <div className="relative w-full aspect-[4/5] overflow-hidden rounded-[12px] bg-white/5 dark:bg-zinc-900/10 border border-white/10 dark:border-white/5">
          <Link
            to={`/auction/lots/${lot.id}`}
            className="block w-full h-full"
          >
            {lot.images && lot.images.length > 0 ? (
              <img
                src={lot.images[0].imageUrl}
                alt={lot.title}
                className="w-full h-full object-cover transition-transform duration-700 ease-out group-hover:scale-105"
                loading="lazy"
              />
            ) : (
              <div className="w-full h-full flex items-center justify-center bg-gray-100 dark:bg-zinc-800 text-gray-400">
                Нет фото
              </div>
            )}
          </Link>

          {/* Badges */}
          <div className="absolute top-3 right-3 flex flex-col gap-2 z-10 pointer-events-none">
            {lot.status === 'active' && (
              <span className="inline-flex px-2 py-1 bg-primary text-white text-[9px] font-bold uppercase tracking-wider animate-pulse">
                LIVE
              </span>
            )}
            {isEnded && (
              <span className="inline-flex px-2 py-1 bg-gray-500 text-white text-[9px] font-bold uppercase tracking-wider">
                ЗАВЕРШЕН
              </span>
            )}
          </div>
        </div>
      </div>

      <div className="relative z-10 w-[92%] -mt-4 bg-[#f4f4f4] dark:bg-zinc-900 p-4 pt-4 shadow-lg dark:shadow-black/60 border border-gray-200/80 dark:border-zinc-700 flex flex-col transform transition-transform duration-500 font-mono">
        <div className="flex flex-col text-[11px] leading-relaxed text-black dark:text-gray-100 font-medium">
          <div className="uppercase tracking-widest text-[9px] mb-2 border-b border-dashed border-gray-400 dark:border-zinc-500 pb-2">
            <div className="flex gap-2 items-center text-black/60 dark:text-white/60">
              <Gavel className="w-3 h-3" />
              <span>ЛОТ АУКЦИОНА</span>
            </div>
          </div>
          
          <div className="flex w-full gap-2 items-start mt-1">
            <Link to={`/auction/lots/${lot.id}`} className="group/name flex-1 min-w-0">
              <span className="line-clamp-2 uppercase group-hover/name:underline decoration-1 underline-offset-4 decoration-gray-500 transition-all leading-tight">
                {lot.title}
              </span>
            </Link>
          </div>

          <div className="flex gap-2 items-center mt-2 uppercase">
            <span className="opacity-80">
              {lot.currentBidCents ? 'Текущая ставка:' : 'Начальная цена:'}
            </span>
            <span className="text-[13px] font-bold text-primary dark:text-white">
              {formatPrice(currentPrice)}
            </span>
          </div>

          <div className="flex gap-2 items-center mt-1 text-[10px] uppercase text-gray-500 dark:text-gray-400">
            {auction && (
              <>
                <span>Шаг: {formatPrice(auction.bidStepCents)}</span>
              </>
            )}
          </div>

          <div className="mt-3 pt-3 border-t border-dashed border-gray-400 dark:border-zinc-500 flex justify-between items-center">
            {lot.status === 'active' && auction ? (
              <Countdown endsAt={auction.endsAt} />
            ) : (
              <span className="text-gray-500">
                {isEnded ? 'Торги завершены' : 'Ожидает начала'}
              </span>
            )}
            
            <Link 
              to={`/auction/lots/${lot.id}`}
              className="px-3 py-1.5 bg-black text-white dark:bg-white dark:text-black text-[10px] uppercase font-bold tracking-widest hover:bg-primary transition-colors"
            >
              Сделать ставку
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}
