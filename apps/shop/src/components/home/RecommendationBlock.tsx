import { motion } from 'framer-motion';
import { ProductRail } from './ProductRail';
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

interface RecommendationBlockProps {
  block: UIHomeRecommendationBlock;
  description?: string;
}

export function RecommendationBlock({ block, description }: RecommendationBlockProps) {
  if (!block.items || block.items.length === 0) {
    return null;
  }

  return (
    <motion.section {...reveal} data-testid={`recommendation-block-${block.type}`}>
      <SectionHeader label={getBlockLabel(block.type)} title={block.title} description={description} />
      <ProductRail products={block.items} placement="home_recommendations" />
    </motion.section>
  );
}
