// @vitest-environment jsdom
import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { AdminProductVisionPanel } from './AdminProductVisionPanel';
import type { AdminProductVision } from '@zamk/api-client/src/types';
import type { AdminProductGalleryImage } from '../../api/adminProducts';

describe('AdminProductVisionPanel', () => {
  const mockGallery: AdminProductGalleryImage[] = [
    {
      id: 'img-11111111-1111-1111-1111-111111111111',
      url: 'https://example.com/photo1.jpg',
      altText: 'Фото 1',
    },
    {
      id: 'img-22222222-2222-2222-2222-222222222222',
      url: 'https://example.com/photo2.jpg',
      altText: 'Фото 2',
    },
  ];

  // A. vision absent / not_scheduled
  it('A: renders compact neutral state when vision is absent or not_scheduled', () => {
    const { rerender } = render(<AdminProductVisionPanel vision={null} />);
    expect(screen.getByTestId('ai-vision-panel').getAttribute('data-status')).toBe('not_scheduled');
    expect(screen.getByText('AI-анализ ещё не запускался')).toBeDefined();
    expect(screen.queryByTestId('ai-recommendation-badge')).toBeNull();

    rerender(<AdminProductVisionPanel vision={{ status: 'not_scheduled' }} />);
    expect(screen.getByTestId('ai-vision-panel').getAttribute('data-status')).toBe('not_scheduled');
    expect(screen.getByText('AI-анализ ещё не запускался')).toBeDefined();
  });

  // B. pending
  it('B: renders pending state with queue explanation', () => {
    const vision: AdminProductVision = { status: 'pending' };
    render(<AdminProductVisionPanel vision={vision} />);
    expect(screen.getByTestId('ai-vision-panel').getAttribute('data-status')).toBe('pending');
    expect(screen.getByText('AI-анализ ожидает обработки')).toBeDefined();
    expect(screen.getByText('Задача на анализ поставлена в очередь обработки.')).toBeDefined();
  });

  // C. processing
  it('C: renders processing state with active progress text', () => {
    const vision: AdminProductVision = { status: 'processing' };
    render(<AdminProductVisionPanel vision={vision} />);
    expect(screen.getByTestId('ai-vision-panel').getAttribute('data-status')).toBe('processing');
    expect(screen.getByText('AI анализирует изображения')).toBeDefined();
  });

  // D. failed
  it('D: renders safe failure state without raw error dump', () => {
    const vision: AdminProductVision = {
      status: 'failed',
      failureCode: 'provider_transient',
    };
    render(<AdminProductVisionPanel vision={vision} />);
    expect(screen.getByTestId('ai-vision-panel').getAttribute('data-status')).toBe('failed');
    expect(screen.getByText('AI-анализ не завершён')).toBeDefined();
    expect(screen.getByText(/Временный сбой провайдера AI/)).toBeDefined();
    expect(screen.queryByText(/panic/i)).toBeNull();
    expect(screen.queryByText(/trace/i)).toBeNull();
  });

  // E. stale
  it('E: renders stale warning state and does not show observation as current truth', () => {
    const vision: AdminProductVision = {
      status: 'stale',
      observedCategoryText: 'Старое пальто',
    };
    render(<AdminProductVisionPanel vision={vision} />);
    expect(screen.getByTestId('ai-vision-panel').getAttribute('data-status')).toBe('stale');
    expect(screen.getByText('Устаревший AI-анализ')).toBeDefined();
    expect(screen.getByText(/Есть более старый AI-анализ, но он не относится к текущей версии товара/)).toBeDefined();
    // Observation must NOT be displayed as current truth
    expect(screen.queryByText('«Старое пальто»')).toBeNull();
    expect(screen.queryByTestId('ai-observed-category')).toBeNull();
  });

  // F. succeeded consistent
  it('F: renders succeeded consistent category with positive advisory wording', () => {
    const vision: AdminProductVision = {
      status: 'succeeded',
      categoryConsistency: 'consistent',
      observedCategoryText: 'Платье женское миди',
      moderation: {
        recommendation: 'clear',
        recommendationReason: 'Признаки соответствуют заявленному товару',
      },
    };
    render(<AdminProductVisionPanel vision={vision} declaredCategory="Платья" />);
    expect(screen.getByTestId('ai-vision-panel').getAttribute('data-status')).toBe('succeeded');
    expect(screen.getByText('Категория соответствует изображениям')).toBeDefined();
    expect(screen.getByText('«Платье женское миди»')).toBeDefined();
    expect(screen.getByText('Явных проблем не обнаружено')).toBeDefined();
    expect(screen.getByText('Признаки соответствуют заявленному товару')).toBeDefined();
  });

  // G. succeeded contradictory
  it('G: renders succeeded contradictory category with warning and explanation', () => {
    const vision: AdminProductVision = {
      status: 'succeeded',
      categoryConsistency: 'contradictory',
      observedCategoryText: 'На фото изображена футболка с коротким рукавом, пальто не обнаружено',
      moderation: {
        recommendation: 'review_recommended',
        recommendationReason: 'Возможное расхождение между категорией и фотографиями',
      },
    };
    render(<AdminProductVisionPanel vision={vision} declaredCategory="Пальто" />);
    expect(screen.getByText('Возможное несоответствие категории')).toBeDefined();
    expect(screen.getByText('Рекомендуется дополнительная проверка')).toBeDefined();
    expect(screen.getByText(/На фото изображена футболка/)).toBeDefined();
    expect(screen.getByText('Пальто')).toBeDefined();
  });

  // H. succeeded uncertain
  it('H: renders succeeded uncertain state with neutral-warning wording', () => {
    const vision: AdminProductVision = {
      status: 'succeeded',
      categoryConsistency: 'uncertain',
      moderation: {
        recommendation: 'uncertain',
        recommendationReason: 'Низкая уверенность модели из-за ракурса',
      },
    };
    render(<AdminProductVisionPanel vision={vision} />);
    expect(screen.getByText('Недостаточно данных для проверки категории')).toBeDefined();
    expect(screen.getByText('Недостаточно данных для уверенной оценки')).toBeDefined();
    expect(screen.getByText('Низкая уверенность модели из-за ракурса')).toBeDefined();
  });

  // I. empty colors hidden cleanly
  it('I: cleanly hides color section when dominant and secondary colors are empty', () => {
    const vision: AdminProductVision = {
      status: 'succeeded',
      dominantColors: [],
      secondaryColors: [],
    };
    render(<AdminProductVisionPanel vision={vision} />);
    expect(screen.queryByTestId('ai-colors-section')).toBeNull();
  });

  // J. colors render name + swatch
  it('J: renders colors with name and hex swatch', () => {
    const vision: AdminProductVision = {
      status: 'succeeded',
      dominantColors: [
        { id: 'col-black', code: 'black', name: 'Черный', hex: '#000000' },
      ],
      secondaryColors: [
        { id: 'col-navy', code: 'navy', name: 'Темно-синий', hex: '#000080' },
      ],
    };
    render(<AdminProductVisionPanel vision={vision} />);
    expect(screen.getByTestId('ai-colors-section')).toBeDefined();
    expect(screen.getByText('Черный')).toBeDefined();
    expect(screen.getByText('Темно-синий')).toBeDefined();
    expect(screen.getByText('Основные цвета:')).toBeDefined();
    expect(screen.getByText('Дополнительные цвета:')).toBeDefined();
  });

  // K. unresolved color safe fallback
  it('K: renders neutral unresolved fallback without raw UUID', () => {
    const vision: AdminProductVision = {
      status: 'succeeded',
      dominantColors: [
        {
          id: '550e8400-e29b-41d4-a716-446655440000',
          code: 'unresolved',
          name: 'Неизвестный цвет',
          hex: undefined,
        },
      ],
    };
    render(<AdminProductVisionPanel vision={vision} />);
    expect(screen.getByText('Неизвестный цвет')).toBeDefined();
    expect(screen.queryByText(/550e8400/)).toBeNull();
  });

  // L. visual vocabulary tokens mapped to Russian labels
  it('L: maps vocabulary tokens to Russian labels', () => {
    const vision: AdminProductVision = {
      status: 'succeeded',
      silhouette: 'vision.silhouette.oversized',
      pattern: 'vision.pattern.striped',
      visibleTexture: 'vision.texture.knitted',
      sleeves: ['short'],
      closure: 'zipper',
      formality: 'casual',
      seasonalCues: ['summer'],
    };
    render(<AdminProductVisionPanel vision={vision} />);
    expect(screen.getByText('Оверсайз')).toBeDefined();
    expect(screen.getByText('В полоску')).toBeDefined();
    expect(screen.getByText('Вязаная / трикотаж')).toBeDefined();
    expect(screen.getByText('Короткий')).toBeDefined();
    expect(screen.getByText('Молния')).toBeDefined();
    expect(screen.getByText('Повседневный')).toBeDefined();
    expect(screen.getByText('Лето')).toBeDefined();
    // Verify no raw vision. tokens in DOM
    expect(screen.queryByText(/vision\.silhouette/)).toBeNull();
    expect(screen.queryByText(/vision\.pattern/)).toBeNull();
  });

  // M. empty attributes omitted
  it('M: omits empty attribute fields from visual attributes grid', () => {
    const vision: AdminProductVision = {
      status: 'succeeded',
      silhouette: 'oversized',
      sleeves: [],
      neckline: undefined,
      closure: undefined,
    };
    render(<AdminProductVisionPanel vision={vision} />);
    expect(screen.getByText('Оверсайз')).toBeDefined();
    expect(screen.queryByText('Вырез горловины')).toBeNull();
    expect(screen.queryByText('Застёжка')).toBeNull();
  });

  // N. imageSetConsistency=false warning
  it('N: renders clear warning banner when imageSetConsistency is false', () => {
    const vision: AdminProductVision = {
      status: 'succeeded',
      imageSetConsistency: false,
    };
    render(<AdminProductVisionPanel vision={vision} />);
    expect(screen.getByTestId('ai-image-set-warning')).toBeDefined();
    expect(screen.getByText(/Фотографии могут относиться к разным объектам/)).toBeDefined();
  });

  // O. moderation recommendation advisory wording
  it('O: uses strictly advisory language in recommendations', () => {
    const vision: AdminProductVision = {
      status: 'succeeded',
      moderation: {
        recommendation: 'review_recommended',
        recommendationReason: 'Рекомендуется проверить категорию',
      },
    };
    render(<AdminProductVisionPanel vision={vision} />);
    expect(screen.getByText('Рекомендуется дополнительная проверка')).toBeDefined();
    expect(screen.getByText(/ИИ помогает выявить возможные несоответствия/)).toBeDefined();
    // Forbidden words
    expect(screen.queryByText(/Товар неправильный/)).toBeNull();
    expect(screen.queryByText(/Отклонить товар/)).toBeNull();
    expect(screen.queryByText(/ИИ решил/)).toBeNull();
  });

  // P. evidence photo mapping
  it('P: maps evidence imageIds to gallery thumbnails', () => {
    const vision: AdminProductVision = {
      status: 'succeeded',
      evidence: [
        {
          field: 'silhouette',
          valueId: 'vision.silhouette.oversized',
          imageIds: ['img-11111111-1111-1111-1111-111111111111'],
        },
      ],
    };
    render(<AdminProductVisionPanel vision={vision} gallery={mockGallery} />);
    expect(screen.getByTestId('ai-evidence-section')).toBeDefined();
    expect(screen.getByText('Силуэт — Оверсайз')).toBeDefined();
    const photoBtn = screen.getByTitle('Фото #1');
    expect(photoBtn).toBeDefined();
    const img = photoBtn.querySelector('img');
    expect(img).not.toBeNull();
    expect(img?.getAttribute('src')).toBe('https://example.com/photo1.jpg');
  });

  // Q. unknown token does not crash
  it('Q: safely handles unknown tokens with humanized fallback without crash', () => {
    const vision: AdminProductVision = {
      status: 'succeeded',
      silhouette: 'vision.silhouette.rare_futuristic_fit',
      pattern: 'custom_zebra_lines',
      moderation: {
        recommendation: 'review_recommended',
        qualityIssues: ['vision.quality.unexpected_artifact'],
        contradictions: [
          {
            field: 'special_custom_field',
            declared: 'Специальный',
            observed: 'Необычный',
            reason: 'Обнаружено отличие',
          },
        ],
      },
    };
    render(<AdminProductVisionPanel vision={vision} />);
    expect(screen.getByText('Rare futuristic fit')).toBeDefined();
    expect(screen.getByText('Custom zebra lines')).toBeDefined();
    expect(screen.getByText('Unexpected artifact')).toBeDefined();
    expect(screen.getByText('Special custom field')).toBeDefined();
  });

  // OCR section test
  it('renders OCR findings and text when present', () => {
    const vision: AdminProductVision = {
      status: 'succeeded',
      ocrText: ['100% COTTON', 'MADE IN ITALY'],
    };
    render(<AdminProductVisionPanel vision={vision} />);
    expect(screen.getByTestId('ai-ocr-section')).toBeDefined();
    expect(screen.getByText('«100% COTTON»')).toBeDefined();
    expect(screen.getByText('«MADE IN ITALY»')).toBeDefined();
  });

  // Confidence toggle test
  it('renders and toggles confidence details', () => {
    const vision: AdminProductVision = {
      status: 'succeeded',
      confidence: {
        category: { level: 'high', score: 0.95 },
        silhouette: { level: 'medium', score: 0.8 },
      },
    };
    render(<AdminProductVisionPanel vision={vision} />);
    expect(screen.getByTestId('ai-confidence-section')).toBeDefined();
    const toggleBtn = screen.getByText(/Подробнее об уверенности модели/);
    fireEvent.click(toggleBtn);
    expect(screen.getByText('95%')).toBeDefined();
    expect(screen.getByText('80%')).toBeDefined();
  });

  describe('VISION A3.2A Evidence formatting and raw ID prevention', () => {
    const categoryUuid = '2d097e3c-f546-4af4-b7c1-2b954783a6b4';
    const randomUuid = '9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d';

    it('strictly prevents category evidence raw UUID from appearing in the DOM', () => {
      const vision: AdminProductVision = {
        status: 'succeeded',
        evidence: [
          {
            field: 'category',
            valueId: categoryUuid,
            imageIds: ['img-11111111-1111-1111-1111-111111111111'],
          },
        ],
      };
      const { container } = render(
        <AdminProductVisionPanel
          vision={vision}
          gallery={mockGallery}
          declaredCategory="Coats & Jackets"
        />
      );

      // Raw UUID must NEVER be present anywhere in rendered HTML
      expect(screen.queryByText(new RegExp(categoryUuid, 'i'))).toBeNull();
      expect(container.innerHTML).not.toContain(categoryUuid);

      // Human-readable category label must be present
      expect(screen.getByText('Категория — Coats & Jackets')).toBeDefined();
    });

    it('falls back to field label when declaredCategory is missing or is UUID', () => {
      const vision: AdminProductVision = {
        status: 'succeeded',
        evidence: [
          {
            field: 'category',
            valueId: categoryUuid,
            imageIds: ['img-11111111-1111-1111-1111-111111111111'],
          },
        ],
      };
      render(<AdminProductVisionPanel vision={vision} gallery={mockGallery} />);
      expect(screen.getByText('Категория')).toBeDefined();
      expect(screen.queryByText(new RegExp(categoryUuid, 'i'))).toBeNull();
    });

    it('never renders raw UUID for non-category fields with UUID valueId', () => {
      const vision: AdminProductVision = {
        status: 'succeeded',
        evidence: [
          {
            field: 'fit',
            valueId: randomUuid,
            imageIds: ['img-11111111-1111-1111-1111-111111111111'],
          },
        ],
      };
      const { container } = render(
        <AdminProductVisionPanel vision={vision} gallery={mockGallery} />
      );
      expect(screen.getByText('Посадка')).toBeDefined();
      expect(screen.queryByText(new RegExp(randomUuid, 'i'))).toBeNull();
      expect(container.innerHTML).not.toContain(randomUuid);
    });

    it('resolves color evidence to color name if available in dominant/secondary colors', () => {
      const colorUuid = '3c98b2c4-0000-4000-a000-000000000000';
      const vision: AdminProductVision = {
        status: 'succeeded',
        dominantColors: [
          { id: colorUuid, code: 'black', name: 'Черный', hex: '#000000' },
        ],
        evidence: [
          {
            field: 'dominantColors',
            valueId: colorUuid,
            imageIds: ['img-11111111-1111-1111-1111-111111111111'],
          },
        ],
      };
      render(<AdminProductVisionPanel vision={vision} gallery={mockGallery} />);
      expect(screen.getByText('Основные цвета — Черный')).toBeDefined();
      expect(screen.queryByText(new RegExp(colorUuid, 'i'))).toBeNull();
    });

    it('does not leak raw image ID if evidence photo is not found in gallery', () => {
      const missingImgId = 'd8f3a1b2-9999-4444-8888-777777777777';
      const vision: AdminProductVision = {
        status: 'succeeded',
        evidence: [
          {
            field: 'silhouette',
            valueId: 'vision.silhouette.oversized',
            imageIds: [missingImgId],
          },
        ],
      };
      const { container } = render(
        <AdminProductVisionPanel vision={vision} gallery={mockGallery} />
      );
      expect(screen.getByText('Фото')).toBeDefined();
      expect(screen.queryByText(new RegExp(missingImgId.slice(0, 8), 'i'))).toBeNull();
      expect(container.innerHTML).not.toContain(missingImgId);
    });
  });
});
