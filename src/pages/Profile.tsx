import React from 'react';
import { Package, MapPin, Heart, Settings } from 'lucide-react';
import { MOCK_PRODUCTS } from '../lib/mock-data';
import { ProductCard } from '../components/product/ProductCard';

export function Profile() {
  const wishlist = MOCK_PRODUCTS.slice(4, 6);

  return (
    <div className="container mx-auto px-6 max-w-7xl py-12">
      <div className="mb-16">
        <h1 className="text-4xl font-serif text-graphite mb-2">My Account</h1>
        <p className="text-ash">Welcome back, User.</p>
      </div>

      <div className="flex flex-col md:flex-row gap-12">
        {/* Profile Sidebar */}
        <aside className="w-full md:w-64 shrink-0">
          <ul className="space-y-2">
            <li>
              <button className="w-full flex items-center gap-3 text-sm font-medium p-3 rounded-md bg-graphite text-milk transition-colors">
                <Heart className="w-4 h-4" />
                Wishlist
              </button>
            </li>
            <li>
              <button className="w-full flex items-center gap-3 text-sm font-medium p-3 rounded-md text-graphite hover:bg-slate-50 transition-colors">
                <Package className="w-4 h-4" />
                Orders
              </button>
            </li>
            <li>
              <button className="w-full flex items-center gap-3 text-sm font-medium p-3 rounded-md text-graphite hover:bg-slate-50 transition-colors">
                <MapPin className="w-4 h-4" />
                Addresses
              </button>
            </li>
            <li>
              <button className="w-full flex items-center gap-3 text-sm font-medium p-3 rounded-md text-graphite hover:bg-slate-50 transition-colors">
                <Settings className="w-4 h-4" />
                Settings
              </button>
            </li>
          </ul>
        </aside>

        {/* Profile Content */}
        <div className="flex-1">
          <h2 className="text-2xl font-serif text-graphite mb-8">Wishlist</h2>
          {wishlist.length > 0 ? (
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
              {wishlist.map(product => (
                <ProductCard key={product.id} product={product} />
              ))}
            </div>
          ) : (
            <div className="text-center py-20 border border-border-soft border-dashed rounded-xl">
              <Heart className="w-8 h-8 mx-auto text-border-soft mb-4" />
              <p className="text-ash">Your wishlist is empty.</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
