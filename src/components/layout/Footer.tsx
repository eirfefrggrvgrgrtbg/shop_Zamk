import React from 'react';

export function Footer() {
  return (
    <footer className="bg-milk border-t border-border-soft pt-16 pb-8 mt-auto">
      <div className="container mx-auto px-6 max-w-7xl">
        <div className="grid grid-cols-1 md:grid-cols-4 gap-12 mb-16">
          <div className="space-y-4">
            <h3 className="font-serif text-xl font-bold">ZAMK</h3>
            <p className="text-sm text-ash max-w-xs">
              A curated digital gallery for premium fashion and lifestyle products.
            </p>
          </div>
          
          <div>
            <h4 className="font-medium mb-4">Shop</h4>
            <ul className="space-y-3 text-sm text-ash">
              <li><a href="#" className="hover:text-graphite transition-colors">New Arrivals</a></li>
              <li><a href="#" className="hover:text-graphite transition-colors">All Brands</a></li>
              <li><a href="#" className="hover:text-graphite transition-colors">Clothing</a></li>
              <li><a href="#" className="hover:text-graphite transition-colors">Accessories</a></li>
            </ul>
          </div>
          
          <div>
            <h4 className="font-medium mb-4">Customer Care</h4>
            <ul className="space-y-3 text-sm text-ash">
              <li><a href="#" className="hover:text-graphite transition-colors">Contact Us</a></li>
              <li><a href="#" className="hover:text-graphite transition-colors">Shipping & Returns</a></li>
              <li><a href="#" className="hover:text-graphite transition-colors">Size Guide</a></li>
              <li><a href="#" className="hover:text-graphite transition-colors">FAQ</a></li>
            </ul>
          </div>
          
          <div>
            <h4 className="font-medium mb-4">Newsletter</h4>
            <p className="text-sm text-ash mb-4">Subscribe to receive updates, access to exclusive deals, and more.</p>
            <div className="flex">
              <input 
                type="email" 
                placeholder="Enter your email" 
                className="bg-transparent border-b border-border-soft px-0 py-2 text-sm focus:outline-none focus:border-graphite w-full transition-colors"
              />
              <button className="text-sm font-medium uppercase tracking-wider ml-4 hover:text-dusty-blue transition-colors">
                Subscribe
              </button>
            </div>
          </div>
        </div>
        
        <div className="pt-8 border-t border-border-soft flex flex-col md:flex-row items-center justify-between text-xs text-ash">
          <p>&copy; {new Date().getFullYear()} ZAMK. All rights reserved.</p>
          <div className="flex gap-4 mt-4 md:mt-0">
            <a href="#" className="hover:text-graphite transition-colors">Privacy Policy</a>
            <a href="#" className="hover:text-graphite transition-colors">Terms of Service</a>
          </div>
        </div>
      </div>
    </footer>
  );
}
