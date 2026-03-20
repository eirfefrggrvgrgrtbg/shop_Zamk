import React from 'react';
import { MOCK_PRODUCTS } from '../lib/mock-data';
import { Button } from '../components/ui/Button';
import { Minus, Plus, X } from 'lucide-react';
import { Link } from 'react-router-dom';

export function Cart() {
  const cartItems = MOCK_PRODUCTS.slice(0, 2);
  const subtotal = cartItems.reduce((acc, item) => acc + item.price, 0);
  const shipping = 15;
  const total = subtotal + shipping;

  return (
    <div className="container mx-auto px-6 max-w-7xl py-12">
      <h1 className="text-4xl font-serif text-graphite mb-12">Your Cart</h1>
      
      <div className="flex flex-col lg:flex-row gap-12 lg:gap-24">
        {/* Cart Items */}
        <div className="w-full lg:w-2/3">
          <div className="border-t border-border-soft">
            {cartItems.map(item => (
              <div key={item.id} className="py-8 border-b border-border-soft flex gap-6">
                <Link to={`/product/${item.id}`} className="w-24 h-32 md:w-32 md:h-40 shrink-0 bg-[#F8F9FA] rounded-md overflow-hidden">
                  <img src={item.image} alt={item.name} className="w-full h-full object-cover mix-blend-multiply" />
                </Link>
                
                <div className="flex-1 flex flex-col justify-between">
                  <div className="flex justify-between items-start">
                    <div>
                      <span className="text-xs font-bold tracking-widest text-ash uppercase mb-1 block">{item.brand}</span>
                      <Link to={`/product/${item.id}`} className="text-graphite font-medium hover:text-dusty-blue transition-colors">
                        {item.name}
                      </Link>
                      <p className="text-ash text-sm mt-1">Size: M</p>
                    </div>
                    <button className="text-ash hover:text-red-500 transition-colors">
                      <X className="w-5 h-5" />
                    </button>
                  </div>
                  
                  <div className="flex justify-between items-end mt-4">
                    <div className="flex items-center border border-border-soft rounded">
                      <button className="p-2 text-ash hover:text-graphite"><Minus className="w-3 h-3" /></button>
                      <span className="px-4 text-sm font-medium text-graphite">1</span>
                      <button className="p-2 text-ash hover:text-graphite"><Plus className="w-3 h-3" /></button>
                    </div>
                    <span className="font-medium text-graphite">${item.price}</span>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Order Summary */}
        <div className="w-full lg:w-1/3">
          <div className="bg-milk p-8 rounded-xl border border-border-soft sticky top-32">
            <h2 className="text-xl font-serif text-graphite mb-6">Order Summary</h2>
            
            <div className="space-y-4 text-sm text-ash mb-8">
              <div className="flex justify-between">
                <span>Subtotal</span>
                <span className="text-graphite">${subtotal}</span>
              </div>
              <div className="flex justify-between">
                <span>Shipping</span>
                <span className="text-graphite">${shipping}</span>
              </div>
            </div>
            
            <div className="border-t border-border-soft pt-4 mb-8">
              <div className="flex justify-between items-end">
                <span className="text-graphite font-medium">Total</span>
                <span className="text-2xl font-medium text-graphite">${total}</span>
              </div>
            </div>
            
            <Button className="w-full h-14 text-sm tracking-widest uppercase">
              Proceed to Checkout
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
