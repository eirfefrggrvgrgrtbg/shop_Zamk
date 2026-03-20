import React from 'react';
import { EditorialHero } from '../components/editorial/EditorialHero';
import { ProductCard } from '../components/product/ProductCard';
import { MOCK_PRODUCTS, MOCK_BRANDS } from '../lib/mock-data';
import { Button } from '../components/ui/Button';

export function Home() {
  const curatedProducts = MOCK_PRODUCTS.slice(0, 4);

  return (
    <div className="flex flex-col w-full">
      <EditorialHero />
      
      {/* Curated Selection Section */}
      <section className="py-24 sm:py-32 bg-white">
        <div className="container mx-auto px-6 max-w-7xl">
          <div className="flex flex-col md:flex-row md:items-end justify-between mb-16 gap-6">
            <div className="max-w-xl">
              <h2 className="text-3xl md:text-5xl font-serif text-graphite mb-4">Curated Editions.</h2>
              <p className="text-ash text-lg">A selection of our most coveted pieces this season, carefully chosen for the modern wardrobe.</p>
            </div>
            <Button variant="outline" className="shrink-0 group">
              View All 
              <span className="inline-block ml-2 transition-transform group-hover:translate-x-1">→</span>
            </Button>
          </div>
          
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6 md:gap-8">
            {curatedProducts.map(product => (
              <ProductCard key={product.id} product={product} />
            ))}
          </div>
        </div>
      </section>

      {/* Brand Highlights Minimalist Marquee */}
      <section className="py-24 bg-milk border-y border-border-soft overflow-hidden">
        <div className="container mx-auto px-6 max-w-7xl">
          <p className="text-xs font-bold tracking-[0.2em] text-center text-ash uppercase mb-12">
            Selected Brands
          </p>
          <div className="flex flex-wrap justify-center gap-x-12 gap-y-8 md:gap-x-24 opacity-60">
            {MOCK_BRANDS.map((brand, i) => (
              <span key={i} className="text-xl md:text-2xl font-serif text-graphite tracking-wide hover:opacity-100 transition-opacity cursor-pointer">
                {brand}
              </span>
            ))}
          </div>
        </div>
      </section>
      
      {/* Editorial Story Section */}
      <section className="py-32 bg-white flex items-center justify-center">
        <div className="container mx-auto px-6 max-w-7xl grid grid-cols-1 md:grid-cols-2 gap-16 items-center">
          <div className="relative aspect-[3/4] overflow-hidden">
            <img 
              src="https://images.unsplash.com/photo-1550614000-4b95d4158223?q=80&w=2000&auto=format&fit=crop" 
              alt="Editorial" 
              className="w-full h-full object-cover"
            />
          </div>
          <div className="max-w-md">
            <h2 className="text-4xl md:text-5xl font-serif text-graphite mb-6 leading-tight">The Art of <br/>Restraint.</h2>
            <p className="text-ash text-lg mb-8 leading-relaxed">
              True luxury whispers. It's found in the perfect drape of silk against the skin, the architectural precision of an oversized coat, and the quiet confidence of minimalist design. Explore our latest editorial focusing on transitional pieces.
            </p>
            <Button className="rounded-none border-b border-graphite pb-1 text-graphite px-0 hover:text-dusty-blue-dark hover:border-dusty-blue-dark bg-transparent shadow-none h-auto transition-colors">
              Read the Editorial
            </Button>
          </div>
        </div>
      </section>
    </div>
  );
}
