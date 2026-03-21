import { Link } from 'react-router-dom';
import { Heart } from 'lucide-react';
import type { Product } from '../../lib/mock-data';
import { formatPrice } from '../../lib/utils';
import { Badge } from '../ui/Badge';
import { useFavorites } from '../../contexts/FavoritesContext';

interface ProductCardProps {
  product: Product;
}

export function ProductCard({ product }: ProductCardProps) {
  const { toggleFavorite, isFavorite } = useFavorites();
  const favorited = isFavorite(product.id);

  return (
    <div className="group capsule flex flex-col h-full">
      <Link to={`/product/${product.id}`} className="relative block overflow-hidden aspect-[4/5] p-2">
        {/* The image is contained within the glass capsule for the digital showcase effect */}
        <div className="relative w-full h-full rounded-[1.5rem] overflow-hidden">
          <img
            src={product.images?.[0] ?? product.image}
            alt={product.name}
            className="object-cover w-full h-full transition-transform duration-[1.5s] ease-out group-hover:scale-105"
            loading="lazy"
          />
          {/* Subtle noise over image */}
          <div className="absolute inset-0 bg-white/5 opacity-0 group-hover:opacity-100 transition-opacity mix-blend-overlay"></div>
        </div>
        
        {/* Badges */}
        <div className="absolute top-5 left-5 flex flex-col gap-2">
          {product.isNew && <Badge variant="new">Новинка</Badge>}
          {product.isBestseller && <Badge variant="bestseller">Хит</Badge>}
          {product.discountPrice && <Badge variant="sale">Скидка</Badge>}
        </div>

        {/* Floating favorite button */}
        <button 
          onClick={(e) => {
            e.preventDefault();
            toggleFavorite(product.id);
          }}
          className={`absolute top-5 right-5 w-10 h-10 rounded-full flex items-center justify-center transition-all duration-300 z-10 
            ${favorited 
              ? 'bg-primary text-white shadow-md' 
              : 'bg-white/40 backdrop-blur-md text-graphite hover:bg-white/80 hover:text-primary'
            }`}
        >
          <Heart className={`w-5 h-5 ${favorited ? 'fill-current' : ''}`} />
        </button>
      </Link>

      <div className="p-5 flex flex-col flex-grow">
        <div className="mb-auto">
          <p className="text-[10px] font-bold tracking-widest text-ash uppercase mb-1.5">{product.brand}</p>
          <Link to={`/product/${product.id}`} className="block group-hover:text-primary transition-colors duration-300">
            <h3 className="text-sm font-medium text-graphite line-clamp-2 leading-relaxed">{product.name}</h3>
          </Link>
        </div>
        
        <div className="mt-4 flex items-baseline gap-2">
          {product.discountPrice ? (
            <>
              <span className="text-base font-bold text-error">{formatPrice(product.discountPrice)}</span>
              <span className="text-xs text-ash-light line-through decoration-ash-light/50">{formatPrice(product.price)}</span>
            </>
          ) : (
            <span className="text-base font-bold text-graphite">{formatPrice(product.price)}</span>
          )}
        </div>
      </div>
    </div>
  );
}
