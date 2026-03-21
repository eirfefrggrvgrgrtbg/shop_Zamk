import { useParams, Link } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';
import { motion } from 'framer-motion';
import { Button } from '../components/ui/Button';
import { ProductCard } from '../components/product/ProductCard';
import { getBrandById, getProductsByBrand } from '../lib/mock-data';

export function BrandDetail() {
  const { id } = useParams<{ id: string }>();
  const brand = getBrandById(id || '');
  const products = getProductsByBrand(id || '');

  if (!brand) {
    return (
      <div className="min-h-screen bg-milk flex items-center justify-center">
        <div className="text-center">
          <h2 className="text-2xl font-serif text-graphite mb-4">Бренд не найден</h2>
          <Link to="/brands"><Button variant="primary">Все бренды</Button></Link>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-milk">
      {/* Hero cover */}
      <div className="relative h-64 sm:h-80 overflow-hidden">
        <img src={brand.image} alt={brand.name} className="w-full h-full object-cover" />
        <div className="absolute inset-0 bg-gradient-to-t from-milk via-milk/30 to-transparent" />
        <div className="absolute bottom-0 left-0 right-0">
          <div className="container mx-auto px-4 sm:px-6 max-w-7xl pb-8">
            <Link to="/brands" className="inline-flex items-center gap-1.5 text-sm text-white/80 hover:text-white transition-colors mb-3">
              <ArrowLeft className="w-4 h-4" /> Все бренды
            </Link>
            <h1 className="text-4xl sm:text-5xl font-serif text-white">{brand.name}</h1>
            <p className="text-sm text-white/70 mt-1">{brand.country}</p>
          </div>
        </div>
      </div>

      <div className="container mx-auto px-4 sm:px-6 max-w-7xl py-10">
        {/* Description */}
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          className="max-w-2xl mb-12"
        >
          <h2 className="text-xl font-semibold text-graphite mb-3">О бренде</h2>
          <p className="text-ash leading-relaxed">{brand.description}</p>
        </motion.div>

        {/* Products */}
        {products.length > 0 && (
          <section>
            <h2 className="text-2xl font-serif text-graphite mb-8">
              Товары {brand.name} <span className="text-ash text-lg font-normal">({products.length})</span>
            </h2>
            <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4 md:gap-6">
              {products.map(p => (
                <ProductCard key={p.id} product={p} />
              ))}
            </div>
          </section>
        )}
      </div>
    </div>
  );
}
