import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Eye, ArrowLeft } from 'lucide-react';
import { ProductCard } from '../components/product/ProductCard';
import { fetchProductPreviewByToken } from '../api/publicCatalog';
import type { Product } from '../types/catalog';
import { PreviewPageMetadata } from '../components/PreviewPageMetadata';
import { Button } from '../components/ui/Button';

export function ProductCardPreview() {
  const { token } = useParams<{ token: string }>();
  const [product, setProduct] = useState<Product | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function loadPreview() {
      if (!token) return;
      try {
        setIsLoading(true);
        const data = await fetchProductPreviewByToken(token);
        setProduct(data);
        setError(null);
      } catch (err: any) {
        console.error('Failed to load product card preview:', err);
        if (err?.status === 404 || err?.code === 'invalid_preview_link' || err?.message?.includes('недействительна')) {
          setError('Ссылка предпросмотра недействительна');
        } else if (err?.status === 410 && err?.code === 'product_unavailable') {
          setError('Предпросмотр этого товара больше недоступен');
        } else {
          setError('Срок действия ссылки истёк или ссылка больше недоступна');
        }
      } finally {
        setIsLoading(false);
      }
    }
    loadPreview();
  }, [token]);

  if (isLoading) {
    return (
      <div className="relative z-10 min-h-screen pt-36 pb-20 flex justify-center">
        {token && <PreviewPageMetadata />}
        <div className="animate-spin w-8 h-8 border-2 border-black border-t-transparent rounded-full dark:border-white dark:border-t-transparent" />
      </div>
    );
  }

  if (error || !product) {
    return (
      <div className="relative z-10 min-h-screen pt-36 pb-20">
        {token && <PreviewPageMetadata />}
        <div className="container mx-auto px-4 sm:px-6 max-w-4xl text-center">
          <h1 className="text-3xl font-serif text-graphite dark:text-white mb-4">{error || 'Товар не найден'}</h1>
          <p className="text-ash mb-6">Ссылка предпросмотра больше недоступна или её срок действия истёк (15 минут).</p>
          <Link to="/catalog">
            <Button>Вернуться в каталог</Button>
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="relative z-10 min-h-screen pt-20 md:pt-24 pb-20">
      {token && <PreviewPageMetadata />}
      {/* Top Banner */}
      <div className="bg-amber-500 text-slate-950 font-bold px-4 py-3 text-center text-xs sm:text-sm sticky top-16 z-40 shadow-md flex items-center justify-center gap-2 mb-6">
        <Eye className="w-4 h-4 flex-shrink-0" />
        <span>Предпросмотр товара для модерации. Товар ещё не опубликован.</span>
      </div>

      <div className="container mx-auto px-4 sm:px-6 max-w-6xl">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-8 pb-4 border-b border-border-lighter dark:border-white/10">
          <div>
            <span className="text-xs uppercase tracking-widest font-mono text-amber-600 dark:text-amber-400">Сетка каталога</span>
            <h1 className="text-2xl font-serif text-graphite dark:text-white font-bold mt-1">Так товар будет выглядеть в каталоге</h1>
          </div>
          <Link
            to={`/preview/products/${token}`}
            className="inline-flex items-center gap-2 text-sm font-medium text-primary hover:underline"
          >
            <ArrowLeft className="w-4 h-4" />
            <span>Открыть страницу товара</span>
          </Link>
        </div>

        {/* Real Catalog Grid with ProductCard */}
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-6 max-w-xs sm:max-w-none mx-auto">
          <ProductCard
            product={product}
            previewUrl={`/preview/products/${token}`}
          />
        </div>
      </div>
    </div>
  );
}
