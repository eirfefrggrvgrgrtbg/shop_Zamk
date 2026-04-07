import { useMemo, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { ArrowLeft, Heart, SlidersHorizontal, Star } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { Modal } from '../components/ui/Modal';
import { ProductCard } from '../components/product/ProductCard';
import { InfoPanel, SectionHeader } from '../components/editorial/StudioKit';
import { getProductsBySellerId, getSellerBySlug } from '../lib/mock-data';

export function SellerDetail() {
  const [isReviewsOpen, setIsReviewsOpen] = useState(false);
  const { slug } = useParams<{ slug: string }>();
  const seller = getSellerBySlug(slug || '');
  const products = getProductsBySellerId(seller?.id || '');
  const sellerReviews = useMemo(
    () =>
      products.flatMap((product) =>
        (product.reviews || []).map((review) => ({
          ...review,
          productName: product.name,
        }))
      ),
    [products]
  );

  if (!seller) {
    return (
      <div className='relative z-10 min-h-screen pt-36'>
        <div className='container mx-auto px-4 sm:px-6 max-w-4xl text-center'>
          <h1 className='text-4xl font-serif text-graphite dark:text-white'>Продавец не найден</h1>
          <Link to='/brands' className='inline-block mt-6'>
            <Button>К продавцам</Button>
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className='relative z-10 min-h-screen pt-16 md:pt-20 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[1400px]'>
        <Link to='/brands' className='inline-flex items-center gap-2 text-sm text-ash hover:text-graphite dark:text-white/70 dark:hover:text-white'>
          <ArrowLeft className='w-4 h-4' /> Все продавцы
        </Link>

        <section className='mt-6 rounded-[2rem] border border-border-lighter dark:border-white/10 bg-white/70 dark:bg-white/5 backdrop-blur-md overflow-hidden'>
          <div className='relative h-[240px] md:h-[330px]'>
            <img src={seller.coverImage} alt={seller.name} className='absolute inset-0 h-full w-full object-cover' />
            <div className='absolute inset-0 bg-black/40 dark:bg-black/55' />
            <div className='absolute inset-x-0 bottom-0 h-24 bg-gradient-to-t from-white/95 to-transparent dark:from-[#0d0d10] dark:to-transparent' />
          </div>

          <div className='relative px-5 md:px-10 pb-8 md:pb-10 -mt-20 md:-mt-24 z-10'>
            <div className='mx-auto h-32 w-32 md:h-40 md:w-40 overflow-hidden rounded-full border-4 border-white dark:border-[#0d0d10] shadow-[0_10px_30px_rgba(0,0,0,0.22)]'>
              <img src={seller.avatar} alt={seller.name} className='h-full w-full object-cover' />
            </div>

            <div className='mt-5 text-center'>
              <h1 className='text-3xl md:text-5xl font-serif tracking-tight text-graphite dark:text-white'>{seller.name}</h1>
              <p className='mt-2 text-sm md:text-base text-ash dark:text-white/70'>
                {seller.city}, {seller.country} • на площадке с {seller.joinedAt}
              </p>
              <div className='mt-3 inline-flex items-center gap-2 rounded-full border border-border-lighter dark:border-white/15 bg-white/75 dark:bg-white/5 px-4 py-1.5 text-sm text-graphite dark:text-white/90'>
                <Star className='w-3.5 h-3.5 text-yellow-500 fill-yellow-500' />
                <span className='font-semibold'>{seller.rating.toFixed(1)}</span>
                <span className='text-ash/70 dark:text-white/40'>•</span>
                <span>{seller.reviewCount} отзывов</span>
              </div>
            </div>

            <div className='mt-6 flex flex-wrap items-center justify-center gap-2.5'>
              <button className='inline-flex items-center gap-2 rounded-full bg-graphite dark:bg-white text-white dark:text-black px-4 py-2 text-sm font-semibold transition-colors'>
                <Heart className='w-4 h-4' /> Нравится
              </button>
              <button
                onClick={() => setIsReviewsOpen(true)}
                className='rounded-full border border-border-lighter dark:border-white/15 bg-white/85 dark:bg-white/5 text-graphite dark:text-white/80 px-4 py-2 text-sm transition-colors hover:bg-white dark:hover:bg-white/10'
              >
                Отзывы
              </button>
              <span className='rounded-full border border-border-lighter dark:border-white/15 bg-white/85 dark:bg-white/5 text-graphite dark:text-white/80 px-4 py-2 text-sm'>Похожие</span>
            </div>

            <p className='mx-auto mt-6 max-w-3xl text-center text-sm md:text-base text-graphite-light dark:text-white/70 leading-relaxed'>
              {seller.shortDescription}
            </p>
          </div>
        </section>

        <div className='mt-8 grid grid-cols-1 md:grid-cols-2 gap-5'>
          <InfoPanel title='О продавце'>
            {seller.fullDescription}
          </InfoPanel>
          <InfoPanel title='Подход и селекция'>
            {`${seller.shortDescription} Проект из ${seller.city || 'неизвестного города'}, ${seller.country || 'неизвестной страны'}.`}
          </InfoPanel>
        </div>

        <section className='mt-12'>
          <div className='mb-6 flex items-center justify-between gap-4'>
            <SectionHeader
              label='Товары продавца'
              title={`${seller.name} в каталоге`}
              description={`Доступно ${products.length} позиций`}
            />
            <button className='shrink-0 hidden md:inline-flex items-center gap-2 rounded-full border border-border-lighter dark:border-white/15 bg-white/85 dark:bg-white/5 px-4 py-2 text-sm text-graphite dark:text-white/85 hover:bg-white dark:hover:bg-white/10 transition-colors'>
              <SlidersHorizontal className='w-4 h-4' /> Персонализация
            </button>
          </div>

          <button className='mb-5 md:hidden inline-flex items-center gap-2 rounded-full border border-border-lighter dark:border-white/15 bg-white/85 dark:bg-white/5 px-4 py-2 text-sm text-graphite dark:text-white/85 hover:bg-white dark:hover:bg-white/10 transition-colors'>
            <SlidersHorizontal className='w-4 h-4' /> Персонализация
          </button>

          <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6'>
            {products.map((product) => (
              <ProductCard key={product.id} product={product} />
            ))}
          </div>
        </section>

        <Modal isOpen={isReviewsOpen} onClose={() => setIsReviewsOpen(false)} title={`Отзывы о ${seller.name}`}>
          <div className='space-y-4'>
            <div className='rounded-2xl border border-border-lighter dark:border-white/10 bg-white/60 dark:bg-white/5 p-4'>
              <div className='flex items-center gap-2 text-sm text-graphite dark:text-white'>
                <Star className='w-4 h-4 text-yellow-500 fill-yellow-500' />
                <span className='font-semibold'>{seller.rating.toFixed(1)}</span>
                <span className='text-ash/70 dark:text-white/40'>•</span>
                <span>{seller.reviewCount} отзывов на профиль</span>
              </div>
              <p className='mt-2 text-xs text-ash dark:text-white/60'>
                Ниже показаны отзывы покупателей из товаров этого продавца.
              </p>
            </div>

            {sellerReviews.length > 0 ? (
              <div className='space-y-3'>
                {sellerReviews.map((review) => (
                  <article key={review.id} className='rounded-2xl border border-border-lighter dark:border-white/10 bg-white/75 dark:bg-white/[0.04] p-4'>
                    <div className='flex flex-wrap items-center gap-2 text-sm'>
                      <span className='font-semibold text-graphite dark:text-white'>{review.author}</span>
                      <span className='text-ash dark:text-white/40'>•</span>
                      <span className='inline-flex items-center gap-1 text-graphite dark:text-white/90'>
                        <Star className='w-3.5 h-3.5 text-yellow-500 fill-yellow-500' /> {review.rating.toFixed(1)}
                      </span>
                      <span className='text-ash dark:text-white/40'>•</span>
                      <span className='text-ash dark:text-white/60'>{review.date}</span>
                    </div>
                    <p className='mt-2 text-xs text-ash dark:text-white/50'>Товар: {review.productName}</p>
                    <p className='mt-2 text-sm leading-relaxed text-graphite-light dark:text-white/75'>{review.text}</p>
                  </article>
                ))}
              </div>
            ) : (
              <div className='rounded-2xl border border-dashed border-border-lighter dark:border-white/15 bg-white/50 dark:bg-white/[0.03] p-6 text-center'>
                <p className='text-sm text-ash dark:text-white/60'>
                  Пока нет текстовых отзывов. Число отзывов в профиле учитывает и короткие оценки без комментария.
                </p>
              </div>
            )}
          </div>
        </Modal>
      </div>
    </div>
  );
}
