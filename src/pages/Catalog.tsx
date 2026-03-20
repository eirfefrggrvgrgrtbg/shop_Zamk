import React, { useState } from 'react';
import { Filter, ChevronDown } from 'lucide-react';
import { ProductCard } from '../components/product/ProductCard';
import { MOCK_PRODUCTS, MOCK_CATEGORIES } from '../lib/mock-data';
import { Button } from '../components/ui/Button';

export function Catalog() {
  const [activeCategory, setActiveCategory] = useState("All");

  return (
    <div className="container mx-auto px-6 max-w-7xl py-12">
      <div className="mb-16">
        <h1 className="text-4xl md:text-5xl font-serif text-graphite mb-4">The Collection</h1>
        <p className="text-ash max-w-2xl text-lg">
          Browse our entire selection of carefully curated items.
        </p>
      </div>

      <div className="flex flex-col md:flex-row gap-12">
        {/* Sidebar Filter */}
        <aside className="w-full md:w-64 shrink-0">
          <div className="sticky top-32 space-y-10">
            <div>
              <h3 className="text-xs font-bold tracking-[0.2em] text-graphite uppercase mb-6 flex items-center justify-between">
                Categories
                <Filter className="w-4 h-4 md:hidden" />
              </h3>
              <ul className="space-y-4 text-sm">
                <li>
                  <button 
                    className={`transition-colors ${activeCategory === "All" ? "text-graphite font-medium" : "text-ash hover:text-graphite"}`}
                    onClick={() => setActiveCategory("All")}
                  >
                    All Items
                  </button>
                </li>
                {MOCK_CATEGORIES.map(cat => (
                  <li key={cat}>
                    <button 
                      className={`transition-colors ${activeCategory === cat ? "text-graphite font-medium" : "text-ash hover:text-graphite"}`}
                      onClick={() => setActiveCategory(cat)}
                    >
                      {cat}
                    </button>
                  </li>
                ))}
              </ul>
            </div>
            
            <div className="hidden md:block">
              <h3 className="text-xs font-bold tracking-[0.2em] text-graphite uppercase mb-6">Sort By</h3>
              <div className="relative">
                <select className="w-full appearance-none bg-transparent border-b border-border-soft pb-2 text-sm text-graphite focus:outline-none focus:border-graphite rounded-none cursor-pointer">
                  <option>Newest Arrivals</option>
                  <option>Price: High to Low</option>
                  <option>Price: Low to High</option>
                </select>
                <ChevronDown className="absolute right-0 top-1 text-ash w-4 h-4 pointer-events-none" />
              </div>
            </div>
          </div>
        </aside>

        {/* Product Grid */}
        <div className="flex-1">
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6 md:gap-x-8 md:gap-y-12">
            {MOCK_PRODUCTS.map(product => (
              <ProductCard key={product.id} product={product} />
            ))}
            {MOCK_PRODUCTS.map(product => (
              <ProductCard key={product.id + '-dup'} product={product} />
            ))}
          </div>
          
          <div className="mt-24 text-center">
            <Button variant="outline" className="px-12">Load More</Button>
          </div>
        </div>
      </div>
    </div>
  );
}
