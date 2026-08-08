import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Loader2, Star } from 'lucide-react';
import { getCustomerReviews } from '@zamk/api-client/src/customer';
import { AccountLayout } from '../components/account/AccountLayout';
import { EmptyState } from '../components/ui/EmptyState';
import { Button } from '../components/ui/Button';

const STATUS_LABELS: Record<string, string> = {
  pending_moderation: 'На модерации',
  approved: 'Одобрен',
  published: 'Опубликован',
  rejected: 'Отклонён',
  hidden: 'Скрыт',
  blocked: 'Заблокирован',
};

export function CustomerReviews() {
  return (
    <AccountLayout title="Мои отзывы">
      <CustomerReviewsContent />
    </AccountLayout>
  );
}

function CustomerReviewsContent() {
  const [reviews, setReviews] = useState<any[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getCustomerReviews()
      .then((res) => setReviews(res?.items || []))
      .catch(() => setError('Не удалось загрузить отзывы.'))
      .finally(() => setIsLoading(false));
  }, []);  return (
    <>

        {isLoading ? (
          <div className="flex justify-center items-center h-48">
            <Loader2 className="w-8 h-8 animate-spin text-graphite/40" />
          </div>
        ) : error ? (
          <EmptyState icon="default" title="Ошибка загрузки" description={error} />
        ) : reviews.length === 0 ? (
          <div className="mt-8 bg-white/60 dark:bg-white/5 border border-border-soft dark:border-white/10 rounded-3xl p-8">
            <EmptyState
              icon="default"
              title="Отзывов пока нет"
              description="Оставляйте отзывы на товары из раздела «Мои заказы» после получения."
              action={
                <Link to="/orders">
                  <Button>Перейти к заказам</Button>
                </Link>
              }
            />
          </div>
        ) : (
          <div className="space-y-4 mt-6">
            {reviews.map((review) => (
              <article
                key={review.id}
                className="rounded-2xl border border-border-lighter dark:border-white/10 bg-white/60 dark:bg-white/5 p-5"
              >
                <div className="flex items-start justify-between gap-4 mb-3">
                  <div>
                    {review.productTitle && (
                      <Link
                        to={`/product/${review.productId}`}
                        className="text-[15px] font-medium text-graphite dark:text-white hover:underline"
                      >
                        {review.productTitle}
                      </Link>
                    )}
                    <p className="text-xs text-ash mt-1">
                      {new Date(review.createdAt).toLocaleDateString('ru-RU', {
                        day: 'numeric',
                        month: 'long',
                        year: 'numeric',
                      })}
                    </p>
                  </div>
                  <span className="text-xs px-2 py-1 rounded-full bg-graphite/5 dark:bg-white/10 text-ash">
                    {STATUS_LABELS[review.status] || review.status}
                  </span>
                </div>
                <div className="flex items-center gap-1 mb-2">
                  {Array.from({ length: 5 }).map((_, i) => (
                    <Star
                      key={i}
                      className={`w-4 h-4 ${i < review.rating ? 'fill-amber-400 text-amber-400' : 'text-gray-300'}`}
                    />
                  ))}
                </div>
                {review.title && (
                  <p className="text-sm font-medium text-graphite dark:text-white mb-1">{review.title}</p>
                )}
                {review.comment && (
                  <p className="text-sm text-ash dark:text-white/70">{review.comment}</p>
                )}
              </article>
            ))}
          </div>
        )}
    </>
  );
}
