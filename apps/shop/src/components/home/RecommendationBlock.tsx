import { motion } from 'framer-motion';
import { ProductCard } from '../product/ProductCard';
import { SectionHeader } from '../editorial/StudioKit';
import type { UIHomeRecommendationBlock } from '../../api/publicCatalog';

const reveal = {
  initial: { opacity: 0, y: 24 },
  whileInView: { opacity: 1, y: 0 },
  viewport: { once: true, margin: '-60px' },
  transition: { duration: 0.8, ease: [0.16, 1, 0.3, 1] as const },
};

function getBlockLabel(type: string): string {
  switch (type) {
    case 'for_you':
      return 'Рекомендации';
    case 'popular':
      return 'Популярное';
    case 'new':
      return 'Новинки';
    default:
      return 'Рекомендации';
  }
}

export function RecommendationBlock({ block }: { block: UIHomeRecommendationBlock }) {
  if (!block.items || block.items.length === 0) {
    return null;
  }

  const label = getBlockLabel(block.type);

  return (
    <motion.section
      {...reveal}
      data-testid={`recommendation-block-${block.type}`}
      className="glass-panel p-7 md:p-10 relative overflow-hidden"
    >
      <div className="absolute inset-0 bg-gradient-to-r from-primary/5 via-transparent to-transparent opacity-30" />
      <SectionHeader label={label} title={block.title} />

      <div className="flex gap-4 overflow-x-auto pb-4 hide-scrollbar">
        {block.items.map((product) => (
          <div key={product.id} className="w-[200px] md:w-[240px] flex-shrink-0">
            <ProductCard product={product} placement="home_recommendations" />
          </div>
        ))}
      </div>
    </motion.section>
  );
}
