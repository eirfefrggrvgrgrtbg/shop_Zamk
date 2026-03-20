import React, { useState } from 'react';
import { motion } from 'framer-motion';
import { Heart } from 'lucide-react';
import { Link } from 'react-router-dom';

export interface Product {
  id: string;
  name: string;
  brand: string;
  price: number;
  image: string;
  category: string;
  isNew?: boolean;
}

interface ProductCardProps {
  product: Product;
}

export function ProductCard({ product }: ProductCardProps) {
  const [isHovered, setIsHovered] = useState(false);

  return (
    <Link to={`/product/${product.id}`} className="block">
      <motion.div
        className="group relative flex flex-col cursor-pointer border border-border-soft bg-white rounded-xl overflow-hidden shadow-[inset_0_-2px_20px_rgba(226,232,240,0.3)] transition-all duration-500 ease-out h-full"
        whileHover={{ y: -8, boxShadow: '0 20px 40px -10px rgba(0,0,0,0.05), inset 0 0 0 rgba(0,0,0,0)' }}
        onHoverStart={() => setIsHovered(true)}
        onHoverEnd={() => setIsHovered(false)}
      >
      {/* Product Image Stage (The "Shelf") */}
      <div className="relative aspect-[4/5] bg-[#F8F9FA] m-2 rounded-lg overflow-hidden flex items-center justify-center">
        <motion.img
          src={product.image}
          alt={product.name}
          className="w-[85%] h-[85%] object-cover object-center drop-shadow-md"
          animate={{ scale: isHovered ? 1.05 : 1 }}
          transition={{ duration: 0.6, ease: [0.16, 1, 0.3, 1] }}
        />
        
        {/* Badges & Actions */}
        <div className="absolute top-3 left-3 flex flex-col gap-2">
          {product.isNew && (
            <span className="bg-graphite text-white text-[10px] font-bold uppercase tracking-widest px-2 py-1 rounded">
              New
            </span>
          )}
        </div>
        
        <button className="absolute top-3 right-3 text-ash hover:text-dusty-blue-dark transition-colors z-10 bg-white/50 backdrop-blur-md p-2 rounded-full opacity-0 group-hover:opacity-100">
          <Heart className="w-4 h-4" />
        </button>
      </div>

      {/* Info Layer */}
      <div className="px-4 pb-5 pt-2 flex flex-col flex-grow">
        <div className="flex justify-between items-start mb-1">
          <span className="text-xs font-semibold tracking-wider text-ash uppercase">
            {product.brand}
          </span>
          <span className="text-sm font-medium text-graphite">${product.price}</span>
        </div>
        <h3 className="text-sm text-graphite font-medium leading-tight">
          {product.name}
        </h3>
      </div>
      </motion.div>
    </Link>
  );
}
