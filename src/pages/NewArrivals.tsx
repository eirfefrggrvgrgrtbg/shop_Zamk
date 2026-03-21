import { Link } from 'react-router-dom';
import { ArrowRight } from 'lucide-react';
import { ProductCard } from '../components/product/ProductCard';
import { Button } from '../components/ui/Button';
import { PRODUCTS } from '../lib/mock-data';

export function NewArrivals() {
  const items = PRODUCTS.filter((product) => product.isNew);

  return (
    <div className="min-h-screen relative z-10 pt-28 pb-16">
      <div className="container mx-auto px-4 sm:px-6 max-w-[1200px]">
        <section className="studio-shell p-6 md:p-8 mb-8">
          <p className="studio-label mb-2">Новая волна</p>
          <h1 className="studio-title mb-3">Новинки сезона</h1>
          <p className="studio-subtitle max-w-3xl">
            Свежие поступления в духе кураторского fashion-showroom: чистый силуэт,
            холодная палитра и независимый характер.
          </p>
          <div className="mt-6 flex flex-wrap gap-3">
            <Link to="/catalog">
              <Button variant="primary">Полный каталог</Button>
            </Link>
            <Link to="/brands">
              <Button variant="outline">Бренды платформы</Button>
            </Link>
          </div>
        </section>

        {items.length > 0 ? (
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4 md:gap-6">
            {items.map((product) => (
              <ProductCard key={product.id} product={product} />
            ))}
          </div>
        ) : (
          <div className="studio-shell p-10 text-center">
            <h3 className="text-2xl font-serif text-graphite mb-3">Скоро появятся новые релизы</h3>
            <p className="text-ash mb-6">Команда ZAMK уже готовит следующую поставку независимых брендов.</p>
            <Link to="/catalog" className="inline-flex">
              <Button variant="primary" className="inline-flex items-center gap-2">
                Перейти в каталог
                <ArrowRight className="w-4 h-4" />
              </Button>
            </Link>
          </div>
        )}
      </div>
    </div>
  );
}
