import { useState, useMemo } from 'react';
import { useParams, Link } from 'react-router-dom';
import { MapPin, Star, Calendar, ChevronRight } from 'lucide-react';
import { ProductCard } from '../components/product/ProductCard';
import { getSellerBySlug, PRODUCTS } from '../lib/mock-data';
import { Button } from '../components/ui/Button';

export function SellerProfile() {
  const { slug } = useParams<{ slug: string }>();
  const seller = getSellerBySlug(slug || '');
  const [activeTab, setActiveTab] = useState<'products' | 'about'>('products');

  const sellerProducts = useMemo(() => {
    return PRODUCTS.filter((p) => p.sellerId === seller?.id);
  }, [seller?.id]);

  if (!seller) {
    return (
      <div className="relative z-10 min-h-screen flex items-center justify-center pt-24 pb-20">
        <div className="container mx-auto px-4 text-center">
          <h1 className="text-3xl lg:text-4xl font-serif text-graphite dark:text-white mb-4">Продавец не найден</h1>
          <p className="text-ash mb-8">Возможно, ссылка устарела или магазин был удален.</p>
          <Link to="/catalog">
            <Button>Перейти в каталог</Button>
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="relative z-10 min-h-screen pt-20 pb-20 bg-[#fbfbfb] dark:bg-[#0a0a0a]">
      {/* Hero Cover */}
      <div className="w-full h-[280px] sm:h-[360px] md:h-[420px] relative overflow-hidden">
        <img 
          src={seller.coverImage} 
          alt={`${seller.name} cover`} 
          className="w-full h-full object-cover"
        />
        <div className="absolute inset-0 bg-gradient-to-t from-black/60 via-black/20 to-transparent" />
      </div>

      <div className="container mx-auto px-4 sm:px-6 max-w-[1400px]">
        {/* Identity Block */}
        <div className="relative -mt-20 sm:-mt-24 mb-10 flex flex-col items-center text-center">
          <div className="w-32 h-32 sm:w-40 sm:h-40 rounded-full border-4 border-[#fbfbfb] dark:border-[#0a0a0a] overflow-hidden bg-white/10 backdrop-blur-md shadow-xl mb-4 relative z-10">
            <img 
              src={seller.avatar} 
              alt={seller.name} 
              className="w-full h-full object-cover"
            />
          </div>
          
          <h1 className="text-3xl sm:text-4xl font-serif text-graphite dark:text-white mb-2 leading-tight">
            {seller.name}
          </h1>
          
          <p className="text-sm sm:text-base text-graphite/70 dark:text-white/70 max-w-2xl mx-auto mb-5 leading-relaxed">
            {seller.shortDescription}
          </p>

          <div className="flex flex-wrap items-center justify-center gap-4 sm:gap-6 text-sm text-graphite/60 dark:text-white/60">
            <div className="flex items-center gap-1.5">
              <Star className="w-4 h-4 fill-amber-400 text-amber-400" />
              <span className="font-medium text-graphite dark:text-white">{seller.rating.toFixed(1)}</span>
              <span>({seller.reviewCount} отзывов)</span>
            </div>
            {seller.city && seller.country && (
              <>
                <span className="hidden sm:inline w-1 h-1 rounded-full bg-border-lighter dark:bg-white/20" />
                <div className="flex items-center gap-1.5">
                  <MapPin className="w-4 h-4" />
                  <span>{seller.city}, {seller.country}</span>
                </div>
              </>
            )}
            <span className="hidden sm:inline w-1 h-1 rounded-full bg-border-lighter dark:bg-white/20" />
            <div className="flex items-center gap-1.5">
              <Calendar className="w-4 h-4" />
              <span>На ZAMK с {seller.joinedAt}</span>
            </div>
          </div>
        </div>

        {/* Tabs */}
        <div className="flex items-center justify-center gap-8 border-b border-border-lighter dark:border-white/10 mb-10">
          <button
            onClick={() => setActiveTab('products')}
            className={`pb-4 text-[15px] font-medium transition-all relative ${
              activeTab === 'products'
                ? 'text-graphite dark:text-white'
                : 'text-graphite/40 dark:text-white/40 hover:text-graphite dark:hover:text-white'
            }`}
          >
            Товары продавца
            <span className="ml-2 text-xs opacity-60 font-normal">({sellerProducts.length})</span>
            {activeTab === 'products' && (
              <span className="absolute bottom-0 left-0 w-full h-[1px] bg-graphite dark:bg-white" />
            )}
          </button>
          <button
            onClick={() => setActiveTab('about')}
            className={`pb-4 text-[15px] font-medium transition-all relative ${
              activeTab === 'about'
                ? 'text-graphite dark:text-white'
                : 'text-graphite/40 dark:text-white/40 hover:text-graphite dark:hover:text-white'
            }`}
          >
            О продавце
            {activeTab === 'about' && (
              <span className="absolute bottom-0 left-0 w-full h-[1px] bg-graphite dark:bg-white" />
            )}
          </button>
        </div>

        {/* Tab Content */}
        <div className="min-h-[400px]">
          {activeTab === 'products' ? (
            sellerProducts.length > 0 ? (
              <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-x-4 gap-y-8 sm:gap-x-6 sm:gap-y-10">
                {sellerProducts.map((product) => (
                  <ProductCard key={product.id} product={product} />
                ))}
              </div>
            ) : (
              <div className="text-center py-20">
                <p className="text-ash">У этого продавца пока нет активных товаров.</p>
              </div>
            )
          ) : (
            <div className="max-w-3xl mx-auto py-8">
              <h2 className="text-xl font-serif text-graphite dark:text-white mb-6">Концепция магазина</h2>
              <div className="prose prose-sm sm:prose-base dark:prose-invert prose-p:text-graphite/70 dark:prose-p:text-white/70 prose-p:leading-relaxed">
                {seller.fullDescription.split('\n').map((paragraph: string, idx: number) => (
                  <p key={idx} className="mb-4">{paragraph}</p>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
