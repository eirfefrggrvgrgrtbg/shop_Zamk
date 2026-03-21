import { useState } from 'react';
import { motion } from 'framer-motion';
import { Heart } from 'lucide-react';
import { Link } from 'react-router-dom';
import { Badge } from '../ui/Badge';
import { formatPrice } from '../../lib/utils';
import { useFavorites } from '../../contexts/FavoritesContext';
import type { Product } from '../../lib/mock-data';

interface ProductCardProps {
  product: Product;
}

export function ProductCard({ product }: ProductCardProps) {
  const [isHovered, setIsHovered] = useState(false);
  const { isFavorite, toggleFavorite } = useFavorites();
  const liked = isFavorite(product.id);

  return (
    <Link to={`/product/${product.id}`} className="block group">
      <motion.div
        className="shelf-card rounded-2xl overflow-hidden h-full flex flex-col"
        onHoverStart={() => setIsHovered(true)}
        onHoverEnd={() => setIsHovered(false)}
      >
        {/* Product Image — "Shelf / Stage" */}
        <div className="relative aspect-[4/5] bg-surface m-2 rounded-xl overflow-hidden flex items-center justify-center">
          <motion.img
            src={product.image}
            alt={product.name}
            className="w-[88%] h-[88%] object-cover object-center rounded-lg"
            animate={{ scale: isHovered ? 1.04 : 1 }}
            transition={{ duration: 0.6, ease: [0.16, 1, 0.3, 1] }}
          />

          {/* Badges */}
          <div className="absolute top-2.5 left-2.5 flex flex-col gap-1.5">
            {product.isNew && <Badge variant="new">Новинка</Badge>}
            {product.isBestseller && <Badge variant="bestseller">Хит</Badge>}
            {product.oldPrice && <Badge variant="sale">Скидка</Badge>}
          </div>

          {/* Favorite button */}
          <button
            onClick={(e) => {
              e.preventDefault();
              e.stopPropagation();
              toggleFavorite(product.id);
            }}
            className={`absolute top-2.5 right-2.5 w-8 h-8 rounded-full flex items-center justify-center transition-all z-10 ${
              liked
                ? 'bg-primary/10 text-primary'
                : 'bg-white/60 backdrop-blur-sm text-ash opacity-0 group-hover:opacity-100'
            } hover:scale-110 active:scale-95`}
          >
            <Heart className={`w-4 h-4 ${liked ? 'fill-primary' : ''}`} />
          </button>
        </div>

        {/* Info */}
        <div className="px-3.5 pb-4 pt-1.5 flex flex-col flex-grow">
          <span className="text-[10px] font-semibold tracking-wider text-ash uppercase mb-0.5">
            {product.brand}
          </span>
          <h3 className="text-sm text-graphite font-medium leading-snug mb-1.5 line-clamp-2">
            {product.name}
          </h3>
          <div className="mt-auto flex items-center gap-2">
            <span className="text-sm font-semibold text-graphite">{formatPrice(product.price)}</span>
            {product.oldPrice && (
              <span className="text-xs text-ash line-through">{formatPrice(product.oldPrice)}</span>
            )}
          </div>
        </div>
      </motion.div>
    </Link>
  );
}
