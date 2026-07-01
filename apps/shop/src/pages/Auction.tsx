import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import { Gavel, AlertCircle } from 'lucide-react';
import { getActiveAuctions } from '@zamk/api-client/src/public';
import type { AuctionEvent } from '@zamk/api-client';
import { SectionHeader } from '../components/editorial/StudioKit';
import { AuctionLotCard } from '../components/auctions/AuctionLotCard';
import { useAuctionStream } from '../hooks/useAuctionStream';

export function AuctionPage() {
  const [auctions, setAuctions] = useState<AuctionEvent[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  const fetchAuctions = async () => {
    try {
      const data = await getActiveAuctions();
      setAuctions(data);
      setError('');
    } catch (err) {
      console.error('Failed to load auctions:', err);
      if (auctions.length === 0) {
        setError('Не удалось загрузить аукционы. Пожалуйста, попробуйте позже.');
      }
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchAuctions();
    
    // Polling every 10 seconds
    const intervalId = setInterval(fetchAuctions, 10000);
    return () => clearInterval(intervalId);
  }, []);

  if (isLoading && auctions.length === 0) {
    return (
      <div className="min-h-[50vh] flex items-center justify-center">
        <div className="text-gray-500 animate-pulse flex items-center gap-2">
          <Gavel className="w-5 h-5" />
          <span>Загрузка аукционов...</span>
        </div>
      </div>
    );
  }

  if (error && auctions.length === 0) {
    return (
      <div className="min-h-[50vh] flex items-center justify-center p-4">
        <div className="text-red-500 flex flex-col items-center gap-3 bg-red-50 dark:bg-red-950/20 p-8 rounded-2xl max-w-md text-center">
          <AlertCircle className="w-8 h-8" />
          <p>{error}</p>
          <button 
            onClick={() => { setIsLoading(true); fetchAuctions(); }}
            className="mt-2 text-sm underline hover:text-red-600"
          >
            Повторить попытку
          </button>
        </div>
      </div>
    );
  }

  if (auctions.length === 0) {
    return (
      <div className="min-h-[50vh] flex items-center justify-center p-4">
        <div className="text-gray-500 flex flex-col items-center gap-3 bg-gray-50 dark:bg-zinc-800/50 p-12 rounded-3xl max-w-lg text-center border border-gray-100 dark:border-white/5">
          <Gavel className="w-12 h-12 opacity-50" />
          <h2 className="text-xl font-serif text-black dark:text-white mt-2">Нет активных аукционов</h2>
          <p className="text-sm">В данный момент нет открытых торгов. Загляните позже!</p>
        </div>
      </div>
    );
  }

  // Show the primary active auction or multiple if there are many.
  // For now, we'll just show all active ones separated by sections.
  
  return (
    <div className="max-w-[1400px] mx-auto px-4 sm:px-6 py-8 md:py-12 mt-16 md:mt-20 min-h-[60vh]">
      {auctions.map((auction, index) => (
        <AuctionSection key={auction.id} initialAuction={auction} index={index} />
      ))}
    </div>
  );
}

function AuctionSection({ initialAuction, index }: { initialAuction: AuctionEvent; index: number }) {
  const [auction, setAuction] = useState<AuctionEvent>(initialAuction);
  const { lastEvent } = useAuctionStream(auction.id);

  // Sync polling updates
  useEffect(() => {
    setAuction(initialAuction);
  }, [initialAuction]);

  useEffect(() => {
    if (!lastEvent) return;

    if (lastEvent.eventType === 'bid_accepted' || lastEvent.eventType === 'lot_extended' || lastEvent.eventType === 'lot_status_changed') {
      setAuction(prev => {
        const updatedLots = prev.lots?.map(lot => {
          if (lot.id === lastEvent.lotId) {
            return {
              ...lot,
              ...(lastEvent.currentBidCents !== undefined ? { currentBidCents: lastEvent.currentBidCents } : {}),
              ...(lastEvent.lotStatus ? { status: lastEvent.lotStatus as any } : {})
            };
          }
          return lot;
        });

        return {
          ...prev,
          ...(lastEvent.endsAt && (lastEvent.eventType === 'bid_accepted' || lastEvent.eventType === 'lot_extended') ? { endsAt: lastEvent.endsAt } : {}),
          lots: updatedLots
        };
      });
    } else if (lastEvent.eventType === 'auction_status_changed') {
      if (lastEvent.auctionStatus) {
        setAuction(prev => ({ ...prev, status: lastEvent.auctionStatus as any }));
      }
    }
  }, [lastEvent]);

  const lots = auction.lots || [];

  return (
    <motion.div 
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ delay: index * 0.1 }}
      className="mb-16"
    >
      <div className="bg-white/40 dark:bg-black/20 p-6 md:p-8 rounded-3xl border border-white/60 dark:border-white/5 backdrop-blur-xl mb-8">
        <SectionHeader 
          label={auction.status === 'live' ? 'Идёт сейчас' : 'Скоро начнётся'}
          title={auction.title}
          description={auction.description || 'Делайте ставки на эксклюзивные лоты.'}
        />
        
        <div className="mt-6 flex flex-wrap gap-4 text-sm font-medium">
          <div className="px-4 py-2 bg-white/60 dark:bg-white/10 rounded-full border border-black/5 dark:border-white/10">
            <span className="text-gray-500 mr-2">Статус:</span>
            <span className={auction.status === 'live' ? 'text-primary dark:text-primary animate-pulse' : 'text-gray-800 dark:text-white'}>
              {auction.status === 'live' ? 'LIVE' : auction.status === 'scheduled' ? 'Запланирован' : 'Завершен'}
            </span>
          </div>
        </div>
      </div>

      {lots.length > 0 ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 md:gap-5">
          {lots.map((lot) => (
            <AuctionLotCard key={lot.id} lot={lot} auction={auction} />
          ))}
        </div>
      ) : (
        <div className="text-center py-12 text-gray-500 border border-dashed border-gray-200 dark:border-zinc-800 rounded-2xl">
          Лоты пока не добавлены
        </div>
      )}
    </motion.div>
  );
}
