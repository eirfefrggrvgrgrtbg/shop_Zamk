import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowRight, Gavel } from 'lucide-react';
import { motion } from 'framer-motion';
import { getHomepageAuctions } from '@zamk/api-client/src/public';
import type { AuctionEvent } from '@zamk/api-client';
import { Button } from '../ui/Button';
import { SectionHeader } from '../editorial/StudioKit';
import { AuctionLotCard } from '../auctions/AuctionLotCard';

const reveal = {
  initial: { opacity: 0, y: 24 },
  whileInView: { opacity: 1, y: 0 },
  viewport: { once: true, margin: '-60px' },
  transition: { duration: 0.8, ease: [0.16, 1, 0.3, 1] as const },
};

export function HomeAuctionBlock() {
  const [auctions, setAuctions] = useState<AuctionEvent[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;

    async function loadAuctions() {
      try {
        const data = await getHomepageAuctions();
        if (!cancelled) {
          setAuctions(data);
        }
      } catch (err) {
        console.error('Failed to load homepage auctions', err);
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    }

    loadAuctions();

    return () => {
      cancelled = true;
    };
  }, []);

  if (isLoading || auctions.length === 0) {
    return null; // Don't show the block if loading or no auctions
  }

  // Find the first active auction or just the first one
  const activeAuction = auctions.find((a) => a.status === 'live') || auctions[0];
  const activeLots = activeAuction.lots || [];

  if (activeLots.length === 0) {
    return null;
  }

  return (
    <motion.section {...reveal} className="glass-panel p-7 md:p-10 border-primary/20 dark:border-primary/20 relative overflow-hidden">
      <div className="absolute top-0 right-0 p-8 opacity-5">
        <Gavel className="w-64 h-64" />
      </div>
      
      <div className="relative z-10">
        <SectionHeader 
          label="Эксклюзив" 
          title={activeAuction.title} 
          description={activeAuction.description || 'Редкие товары, доступные только на аукционе ZAMK.'}
          action={
            <Link to="/auction">
              <Button variant="primary" className="gap-2 bg-primary hover:bg-primary/90 text-white border-transparent">
                Перейти к аукциону <ArrowRight className="w-4 h-4" />
              </Button>
            </Link>
          }
        />
        
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 md:gap-5 mt-6">
          {activeLots.slice(0, 4).map((lot) => (
            <AuctionLotCard key={lot.id} lot={lot} auction={activeAuction} />
          ))}
        </div>
      </div>
    </motion.section>
  );
}
