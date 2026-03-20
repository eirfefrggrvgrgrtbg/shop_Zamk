import React, { useState } from 'react';
import { motion } from 'framer-motion';
import { ShoppingBag, Heart, ChevronRight } from 'lucide-react';
import { Button } from '../components/ui/Button';

// Using mock data for demo, normally this would use an ID from URL params
const MOCK_PRODUCT = {
  id: "p1",
  name: "Structured Minimalist Coat",
  brand: "STUDIO NICHOLSON",
  price: 890,
  image: "https://images.unsplash.com/photo-1539533113208-f6df8cc8b543?q=80&w=1974&auto=format&fit=crop",
  category: "Clothing",
  description: "An architectural approach to outerwear. This coat features a voluminous but controlled silhouette, crafted from a premium wool-cashmere blend. The hidden placket and minimal detailing allow the shape and fabric to take center stage.",
  details: [
    "Oversized fit",
    "Hidden button fastening",
    "Dropped shoulders",
    "Welt pockets",
    "Dry clean only"
  ],
  sizes: ["XS", "S", "M", "L", "XL"]
};

export function ProductDetail() {
  const [selectedSize, setSelectedSize] = useState<string | null>(null);

  return (
    <div className="bg-milk min-h-screen pb-24">
      {/* Breadcrumbs */}
      <div className="container mx-auto px-6 max-w-7xl py-6">
        <div className="flex items-center text-xs tracking-wider uppercase text-ash gap-2">
          <a href="/" className="hover:text-graphite transition-colors">Home</a>
          <ChevronRight className="w-3 h-3" />
          <a href="/shop" className="hover:text-graphite transition-colors">{MOCK_PRODUCT.category}</a>
          <ChevronRight className="w-3 h-3" />
          <span className="text-graphite font-medium">{MOCK_PRODUCT.brand}</span>
        </div>
      </div>

      <div className="container mx-auto px-6 max-w-7xl">
        <div className="flex flex-col lg:flex-row gap-12 lg:gap-24">
          
          {/* Left: Gallery (Sticky on Desktop) */}
          <div className="w-full lg:w-3/5">
            <div className="lg:sticky lg:top-32 flex flex-col gap-6">
              <motion.div 
                className="bg-[#F8F9FA] rounded-2xl overflow-hidden shadow-inner flex items-center justify-center p-4 h-[60vh] lg:h-[75vh]"
                initial={{ opacity: 0, scale: 0.95 }}
                animate={{ opacity: 1, scale: 1 }}
                transition={{ duration: 0.8, ease: "easeOut" }}
              >
                <img 
                  src={MOCK_PRODUCT.image} 
                  alt={MOCK_PRODUCT.name} 
                  className="w-[90%] h-[90%] object-cover object-center drop-shadow-xl"
                />
              </motion.div>
              {/* Thumbnails could go here */}
            </div>
          </div>

          {/* Right: Product Info */}
          <div className="w-full lg:w-2/5 flex flex-col">
            <motion.div
              initial={{ opacity: 0, x: 20 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ duration: 0.6, delay: 0.2 }}
              className="lg:mt-12"
            >
              <span className="text-sm font-bold tracking-[0.2em] text-dusty-blue-dark uppercase mb-2 block">
                {MOCK_PRODUCT.brand}
              </span>
              <h1 className="text-3xl lg:text-4xl font-serif text-graphite mb-4 leading-tight">
                {MOCK_PRODUCT.name}
              </h1>
              <p className="text-2xl font-medium text-graphite mb-10">${MOCK_PRODUCT.price}</p>

              <div className="mb-10">
                <div className="flex justify-between items-center mb-4">
                  <span className="text-sm font-medium text-graphite uppercase tracking-wider">Size</span>
                  <a href="#" className="text-xs text-ash underline hover:text-graphite">Size Guide</a>
                </div>
                <div className="grid grid-cols-5 gap-3">
                  {MOCK_PRODUCT.sizes.map(size => (
                    <button
                      key={size}
                      className={`h-12 border flex items-center justify-center text-sm font-medium transition-all ${
                        selectedSize === size 
                          ? 'border-graphite bg-graphite text-milk' 
                          : 'border-border-soft bg-transparent text-graphite hover:border-graphite'
                      }`}
                      onClick={() => setSelectedSize(size)}
                    >
                      {size}
                    </button>
                  ))}
                </div>
              </div>

              <div className="flex gap-4 mb-12">
                <Button className="flex-1 h-14 text-sm tracking-wider uppercase">
                  Add to Cart
                  <ShoppingBag className="w-4 h-4 ml-2" />
                </Button>
                <Button variant="outline" className="w-14 h-14 p-0 shrink-0">
                  <Heart className="w-5 h-5" />
                </Button>
              </div>

              <div className="prose prose-sm prose-slate mb-12 border-b border-border-soft pb-12">
                <p className="text-ash leading-relaxed">{MOCK_PRODUCT.description}</p>
                <ul className="text-ash mt-6 space-y-2">
                  {MOCK_PRODUCT.details.map((detail, idx) => (
                    <li key={idx} className="flex items-center before:content-[''] before:w-1 before:h-1 before:bg-graphite before:rounded-full before:mr-3">
                      {detail}
                    </li>
                  ))}
                </ul>
              </div>

              {/* Delivery info accordion (static for demo) */}
              <div className="space-y-6">
                <div className="flex justify-between items-center cursor-pointer group">
                  <span className="text-sm font-medium tracking-wide uppercase group-hover:text-dusty-blue transition-colors">Delivery & Returns</span>
                  <ChevronRight className="w-4 h-4 text-ash group-hover:text-dusty-blue transition-colors" />
                </div>
                <div className="flex justify-between items-center cursor-pointer group">
                  <span className="text-sm font-medium tracking-wide uppercase group-hover:text-dusty-blue transition-colors">Details & Care</span>
                  <ChevronRight className="w-4 h-4 text-ash group-hover:text-dusty-blue transition-colors" />
                </div>
              </div>

            </motion.div>
          </div>
          
        </div>
      </div>
    </div>
  );
}
