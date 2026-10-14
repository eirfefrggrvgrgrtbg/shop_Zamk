import { useState } from 'react';
import {
  Sparkles,
  AlertTriangle,
  CheckCircle,
  HelpCircle,
  Clock,
  ChevronDown,
  ChevronUp,
  Image as ImageIcon,
  Type,
  FileQuestion,
  Info,
} from 'lucide-react';
import type {
  AdminProductVision,
  AdminVisionColor,
  AdminVisionEvidence,
  AdminVisionContradiction,
  AdminVisionConfidenceScore,
} from '@zamk/api-client/src/types';
import type { AdminProductGalleryImage } from '../../api/adminProducts';
import {
  formatVisionToken,
  formatContradictionField,
  formatEvidenceLabel,
  formatCategoryConsistency,
  formatModerationRecommendation,
  formatVisionStatus,
  formatFailureCode,
} from '../../utils/visionVocabularyMapper';

interface AdminProductVisionPanelProps {
  vision?: AdminProductVision | null;
  gallery?: AdminProductGalleryImage[];
  declaredCategory?: string | null;
  declaredTitle?: string | null;
}

export function AdminProductVisionPanel({
  vision,
  gallery = [],
  declaredCategory,
}: AdminProductVisionPanelProps) {
  const [showConfidence, setShowConfidence] = useState(false);
  const [selectedEvidencePhoto, setSelectedEvidencePhoto] = useState<string | null>(null);

  // 1. NON-SUCCEEDED STATES (Compact & Clear)
  if (!vision || vision.status !== 'succeeded') {
    const nonSucceededStatus = vision?.status || 'not_scheduled';
    const statusInfo = formatVisionStatus(nonSucceededStatus);

    return (
      <div
        data-testid="ai-vision-panel"
        data-status={nonSucceededStatus}
        className="bg-white dark:bg-slate-900 p-5 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm space-y-3"
      >
        <div className="flex items-center justify-between border-b pb-2">
          <div className="flex items-center gap-2">
            <Sparkles className="w-4 h-4 text-violet-600 dark:text-violet-400" />
            <h2 className="text-sm font-bold text-slate-900 dark:text-white">AI-анализ</h2>
            <span className="text-[11px] text-slate-400 font-normal">(рекомендательный)</span>
          </div>
          <span
            data-testid="ai-status-badge"
            className={`px-2.5 py-0.5 rounded-full text-[11px] font-bold ${
              nonSucceededStatus === 'failed'
                ? 'bg-rose-100 text-rose-800 dark:bg-rose-950/60 dark:text-rose-300 border border-rose-300'
                : nonSucceededStatus === 'stale'
                ? 'bg-amber-100 text-amber-800 dark:bg-amber-950/60 dark:text-amber-300 border border-amber-300'
                : nonSucceededStatus === 'processing'
                ? 'bg-violet-100 text-violet-800 dark:bg-violet-950/60 dark:text-violet-300 border border-violet-300 animate-pulse'
                : 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300 border border-slate-300'
            }`}
          >
            {statusInfo.title}
          </span>
        </div>

        <div className="p-3 bg-slate-50 dark:bg-slate-800/60 rounded-xl text-xs space-y-1">
          <div className="flex items-start gap-2.5">
            {nonSucceededStatus === 'failed' ? (
              <AlertTriangle className="w-4 h-4 text-rose-500 flex-shrink-0 mt-0.5" />
            ) : nonSucceededStatus === 'stale' ? (
              <AlertTriangle className="w-4 h-4 text-amber-500 flex-shrink-0 mt-0.5" />
            ) : nonSucceededStatus === 'processing' ? (
              <Clock className="w-4 h-4 text-violet-500 flex-shrink-0 mt-0.5 animate-spin" />
            ) : (
              <Info className="w-4 h-4 text-slate-400 flex-shrink-0 mt-0.5" />
            )}
            <div>
              <p className="font-medium text-slate-800 dark:text-slate-200">{statusInfo.description}</p>
              {nonSucceededStatus === 'failed' && vision?.failureCode && (
                <p className="text-[11px] text-rose-600 dark:text-rose-400 mt-1">
                  Причина: {formatFailureCode(vision.failureCode)}
                </p>
              )}
              {nonSucceededStatus === 'stale' && (
                <p className="text-[11px] text-amber-700 dark:text-amber-400 mt-1">
                  Карточка товара или состав изображений обновились после последнего анализа. Ожидается новый анализ.
                </p>
              )}
            </div>
          </div>
        </div>
      </div>
    );
  }

  // 2. SUCCEEDED STATE — FULL PANEL
  const categoryConsistency = formatCategoryConsistency(vision.categoryConsistency);
  const recommendation = formatModerationRecommendation(vision.moderation?.recommendation);
  const contradictions: AdminVisionContradiction[] = vision.moderation?.contradictions || [];
  const qualityIssues: string[] = vision.moderation?.qualityIssues || [];
  const ocrFindings: string[] = vision.moderation?.ocrFindings || [];
  const ocrText: string[] = vision.ocrText || [];
  const dominantColors: AdminVisionColor[] = vision.dominantColors || [];
  const secondaryColors: AdminVisionColor[] = vision.secondaryColors || [];
  const evidenceList: AdminVisionEvidence[] = vision.evidence || [];
  const confidenceMap: Record<string, AdminVisionConfidenceScore> = vision.confidence || {};

  // Find referenced photo in gallery
  const findGalleryPhoto = (imgId: string) => {
    return gallery.find((g) => g.id === imgId);
  };

  // Has issues/findings to display
  const hasIssues =
    contradictions.length > 0 ||
    qualityIssues.length > 0 ||
    ocrFindings.length > 0 ||
    ocrText.length > 0;

  // Visual attributes map (only non-empty)
  const visualAttributesList = [
    { label: 'Силуэт', value: vision.silhouette ? formatVisionToken(vision.silhouette) : null },
    { label: 'Посадка', value: vision.fitAppearance ? formatVisionToken(vision.fitAppearance) : null },
    { label: 'Длина', value: vision.length ? formatVisionToken(vision.length) : null },
    {
      label: 'Рукава',
      value:
        vision.sleeves && vision.sleeves.length > 0
          ? vision.sleeves.map(formatVisionToken).join(', ')
          : null,
    },
    { label: 'Вырез горловины', value: vision.neckline ? formatVisionToken(vision.neckline) : null },
    { label: 'Застёжка', value: vision.closure ? formatVisionToken(vision.closure) : null },
    { label: 'Узор / принт', value: vision.pattern ? formatVisionToken(vision.pattern) : null },
    { label: 'Фактура', value: vision.visibleTexture ? formatVisionToken(vision.visibleTexture) : null },
    {
      label: 'Детали',
      value:
        vision.decorativeDetails && vision.decorativeDetails.length > 0
          ? vision.decorativeDetails.map(formatVisionToken).join(', ')
          : null,
    },
    {
      label: 'Стиль',
      value:
        vision.styleDescriptors && vision.styleDescriptors.length > 0
          ? vision.styleDescriptors.map(formatVisionToken).join(', ')
          : null,
    },
    { label: 'Формальность', value: vision.formality ? formatVisionToken(vision.formality) : null },
    {
      label: 'Сезонность',
      value:
        vision.seasonalCues && vision.seasonalCues.length > 0
          ? vision.seasonalCues.map(formatVisionToken).join(', ')
          : null,
    },
  ].filter((attr) => Boolean(attr.value));

  const hasColors = dominantColors.length > 0 || secondaryColors.length > 0;

  return (
    <div
      data-testid="ai-vision-panel"
      data-status="succeeded"
      className="bg-white dark:bg-slate-900 p-5 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm space-y-5"
    >
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 border-b pb-3">
        <div className="flex items-center gap-2.5">
          <div className="w-7 h-7 rounded-lg bg-violet-100 dark:bg-violet-950 flex items-center justify-center text-violet-700 dark:text-violet-300">
            <Sparkles className="w-4 h-4" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h2 className="text-sm font-bold text-slate-900 dark:text-white">AI-анализ изображений</h2>
              <span className="text-[11px] font-medium text-slate-500 bg-slate-100 dark:bg-slate-800 px-2 py-0.5 rounded-full border border-slate-200 dark:border-slate-700">
                Рекомендательный
              </span>
            </div>
            <p className="text-[11px] text-slate-500 mt-0.5">
              ИИ помогает выявить возможные несоответствия. Итоговое решение принимает модератор.
            </p>
          </div>
        </div>

        {/* Moderation Recommendation Badge */}
        <div className="flex items-center gap-2">
          {vision.analyzedAt && (
            <span className="text-[10px] text-slate-400 hidden md:inline">
              {new Date(vision.analyzedAt).toLocaleString('ru-RU')}
            </span>
          )}
          <span
            data-testid="ai-recommendation-badge"
            className={`px-3 py-1 rounded-xl text-xs font-bold flex items-center gap-1.5 ${
              recommendation.variant === 'review_recommended'
                ? 'bg-amber-100 text-amber-900 dark:bg-amber-950/70 dark:text-amber-200 border border-amber-300 dark:border-amber-800'
                : recommendation.variant === 'clear'
                ? 'bg-emerald-100 text-emerald-900 dark:bg-emerald-950/70 dark:text-emerald-200 border border-emerald-300 dark:border-emerald-800'
                : 'bg-slate-100 text-slate-800 dark:bg-slate-800 dark:text-slate-300 border border-slate-300 dark:border-slate-700'
            }`}
          >
            {recommendation.variant === 'review_recommended' ? (
              <AlertTriangle className="w-3.5 h-3.5 text-amber-600 dark:text-amber-400" />
            ) : recommendation.variant === 'clear' ? (
              <CheckCircle className="w-3.5 h-3.5 text-emerald-600 dark:text-emerald-400" />
            ) : (
              <HelpCircle className="w-3.5 h-3.5 text-slate-500" />
            )}
            <span>{recommendation.label}</span>
          </span>
        </div>
      </div>

      {/* SECTION 1: ГЛАВНЫЙ ВЫВОД (Key Findings & Category) */}
      <div className="space-y-3">
        <h3 className="text-xs font-bold text-slate-500 uppercase tracking-wider">Главный вывод</h3>

        {/* Category Consistency Card */}
        <div
          data-testid="ai-category-consistency"
          className={`p-3.5 rounded-xl border flex items-start gap-3 text-xs ${
            categoryConsistency.variant === 'contradictory'
              ? 'bg-rose-50/70 text-rose-950 border-rose-200 dark:bg-rose-950/30 dark:border-rose-900 dark:text-rose-200'
              : categoryConsistency.variant === 'consistent'
              ? 'bg-emerald-50/70 text-emerald-950 border-emerald-200 dark:bg-emerald-950/30 dark:border-emerald-900 dark:text-emerald-200'
              : 'bg-slate-50 text-slate-900 border-slate-200 dark:bg-slate-800/50 dark:border-slate-800 dark:text-slate-200'
          }`}
        >
          {categoryConsistency.variant === 'contradictory' ? (
            <AlertTriangle className="w-4 h-4 text-rose-600 dark:text-rose-400 flex-shrink-0 mt-0.5" />
          ) : categoryConsistency.variant === 'consistent' ? (
            <CheckCircle className="w-4 h-4 text-emerald-600 dark:text-emerald-400 flex-shrink-0 mt-0.5" />
          ) : (
            <HelpCircle className="w-4 h-4 text-slate-500 flex-shrink-0 mt-0.5" />
          )}
          <div className="space-y-1 flex-1">
            <div className="flex flex-wrap items-center justify-between gap-1">
              <span className="font-bold text-xs">{categoryConsistency.label}</span>
              {declaredCategory && (
                <span className="text-[11px] opacity-80">
                  Заявлено продавцом: <strong>{declaredCategory}</strong>
                </span>
              )}
            </div>
            <p className="text-[11px] opacity-90 leading-relaxed">{categoryConsistency.hint}</p>

            {/* Observed Category Text */}
            {vision.observedCategoryText && (
              <div className="mt-2 pt-2 border-t border-current/10">
                <span className="text-[11px] font-semibold block text-slate-700 dark:text-slate-300">
                  Что распознано на изображениях:
                </span>
                <p
                  data-testid="ai-observed-category"
                  className="text-xs italic bg-white/70 dark:bg-slate-900/60 p-2 rounded-lg mt-1 border border-current/10 text-slate-800 dark:text-slate-200"
                >
                  «{vision.observedCategoryText}»
                </p>
              </div>
            )}
          </div>
        </div>

        {/* Image Set Consistency Warning */}
        {vision.imageSetConsistency === false && (
          <div
            data-testid="ai-image-set-warning"
            className="p-3 bg-amber-50 dark:bg-amber-950/40 border border-amber-300 dark:border-amber-800 rounded-xl text-xs text-amber-900 dark:text-amber-200 flex items-start gap-2.5"
          >
            <AlertTriangle className="w-4 h-4 text-amber-600 dark:text-amber-400 flex-shrink-0 mt-0.5" />
            <div>
              <span className="font-bold block">Внимание к набору фотографий</span>
              <p className="text-[11px] mt-0.5 leading-relaxed">
                Фотографии могут относиться к разным объектам или не показывать заявленный товар.
              </p>
            </div>
          </div>
        )}

        {/* Recommendation Reason */}
        {vision.moderation?.recommendationReason && (
          <div className="p-3 bg-slate-50 dark:bg-slate-800/50 rounded-xl border border-slate-200 dark:border-slate-800 text-xs">
            <span className="font-semibold text-slate-700 dark:text-slate-300 block mb-0.5">
              Пояснение ИИ:
            </span>
            <p className="text-slate-600 dark:text-slate-300 text-[11px] leading-relaxed">
              {vision.moderation.recommendationReason}
            </p>
          </div>
        )}
      </div>

      {/* SECTION 2: ПРОБЛЕМЫ И ЗАМЕЧАНИЯ (Contradictions, Quality, OCR) */}
      {hasIssues && (
        <div className="space-y-3 border-t pt-3">
          <h3 className="text-xs font-bold text-slate-500 uppercase tracking-wider">
            Замечания и несоответствия
          </h3>

          {/* Contradictions List */}
          {contradictions.length > 0 && (
            <div data-testid="ai-contradictions" className="space-y-2">
              <span className="text-[11px] font-semibold text-slate-700 dark:text-slate-300 block">
                Возможные противоречия:
              </span>
              <div className="space-y-2">
                {contradictions.map((c, idx) => (
                  <div
                    key={idx}
                    className="p-2.5 bg-rose-50/50 dark:bg-rose-950/20 border border-rose-200 dark:border-rose-900 rounded-xl text-xs space-y-1"
                  >
                    <div className="flex flex-wrap items-center justify-between gap-1">
                      <span className="font-bold text-rose-900 dark:text-rose-300">
                        {formatContradictionField(c.field)}
                      </span>
                    </div>
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 text-[11px]">
                      <div>
                        <span className="text-slate-500">Заявлено продавцом: </span>
                        <span className="font-medium text-slate-800 dark:text-slate-200">{c.declared}</span>
                      </div>
                      <div>
                        <span className="text-slate-500">Распознано AI: </span>
                        <span className="font-medium text-rose-700 dark:text-rose-400">{c.observed}</span>
                      </div>
                    </div>
                    {c.reason && (
                      <p className="text-[11px] text-slate-600 dark:text-slate-400 italic pt-1 border-t border-rose-200/50 dark:border-rose-900/50">
                        {c.reason}
                      </p>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Quality Issues */}
          {qualityIssues.length > 0 && (
            <div data-testid="ai-quality-issues" className="space-y-1.5">
              <span className="text-[11px] font-semibold text-slate-700 dark:text-slate-300 block">
                Замечания к качеству фото:
              </span>
              <div className="flex flex-wrap gap-1.5">
                {qualityIssues.map((issue, idx) => (
                  <span
                    key={idx}
                    className="px-2.5 py-1 bg-amber-50 dark:bg-amber-950/40 text-amber-900 dark:text-amber-200 border border-amber-200 dark:border-amber-800 rounded-lg text-xs font-medium flex items-center gap-1.5"
                  >
                    <AlertTriangle className="w-3 h-3 text-amber-600" />
                    <span>{formatVisionToken(issue)}</span>
                  </span>
                ))}
              </div>
            </div>
          )}

          {/* OCR Findings & Text */}
          {(ocrText.length > 0 || ocrFindings.length > 0) && (
            <div data-testid="ai-ocr-section" className="space-y-1.5">
              <span className="text-[11px] font-semibold text-slate-700 dark:text-slate-300 flex items-center gap-1.5">
                <Type className="w-3 h-3 text-indigo-500" />
                <span>Текст на изображениях (OCR):</span>
              </span>
              <div className="flex flex-wrap gap-1.5">
                {[...new Set([...ocrText, ...ocrFindings])].map((text, idx) => (
                  <span
                    key={idx}
                    className="px-2.5 py-1 bg-slate-100 dark:bg-slate-800 text-slate-800 dark:text-slate-200 rounded-lg text-xs font-mono border border-slate-200 dark:border-slate-700"
                  >
                    «{text}»
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {/* SECTION 3: ВИЗУАЛЬНЫЕ ПРИЗНАКИ И ЦВЕТА */}
      {(hasColors || visualAttributesList.length > 0) && (
        <div className="space-y-3 border-t pt-3">
          <h3 className="text-xs font-bold text-slate-500 uppercase tracking-wider">
            Визуальные признаки
          </h3>

          {/* Colors */}
          {hasColors && (
            <div data-testid="ai-colors-section" className="space-y-2">
              {dominantColors.length > 0 && (
                <div>
                  <span className="text-[11px] font-semibold text-slate-600 dark:text-slate-400 block mb-1">
                    Основные цвета:
                  </span>
                  <div className="flex flex-wrap gap-2">
                    {dominantColors.map((col, idx) => {
                      const isUnresolved = col.code === 'unresolved' || col.name === 'Неизвестный цвет';
                      return (
                        <div
                          key={col.id || idx}
                          className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-slate-100 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 text-xs"
                        >
                          {col.hex && !isUnresolved ? (
                            <span
                              className="w-3.5 h-3.5 rounded-full border border-slate-300 dark:border-slate-600 flex-shrink-0"
                              style={{ backgroundColor: col.hex }}
                              aria-hidden="true"
                            />
                          ) : null}
                          <span
                            className={
                              isUnresolved
                                ? 'text-slate-400 italic'
                                : 'font-medium text-slate-900 dark:text-white'
                            }
                          >
                            {col.name || 'Неизвестный цвет'}
                          </span>
                        </div>
                      );
                    })}
                  </div>
                </div>
              )}

              {secondaryColors.length > 0 && (
                <div>
                  <span className="text-[11px] font-semibold text-slate-600 dark:text-slate-400 block mb-1">
                    Дополнительные цвета:
                  </span>
                  <div className="flex flex-wrap gap-2">
                    {secondaryColors.map((col, idx) => {
                      const isUnresolved = col.code === 'unresolved' || col.name === 'Неизвестный цвет';
                      return (
                        <div
                          key={col.id || idx}
                          className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-slate-50 dark:bg-slate-800/60 border border-slate-200 dark:border-slate-700 text-xs"
                        >
                          {col.hex && !isUnresolved ? (
                            <span
                              className="w-3 h-3 rounded-full border border-slate-300 dark:border-slate-600 flex-shrink-0"
                              style={{ backgroundColor: col.hex }}
                              aria-hidden="true"
                            />
                          ) : null}
                          <span
                            className={
                              isUnresolved
                                ? 'text-slate-400 italic'
                                : 'font-medium text-slate-800 dark:text-slate-300'
                            }
                          >
                            {col.name || 'Неизвестный цвет'}
                          </span>
                        </div>
                      );
                    })}
                  </div>
                </div>
              )}
            </div>
          )}

          {/* Attributes Grid */}
          {visualAttributesList.length > 0 && (
            <div data-testid="ai-visual-attributes" className="grid grid-cols-2 sm:grid-cols-3 gap-2.5 text-xs">
              {visualAttributesList.map((attr, idx) => (
                <div
                  key={idx}
                  className="p-2 rounded-xl bg-slate-50 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-800"
                >
                  <span className="text-[10px] text-slate-400 uppercase tracking-wider block">
                    {attr.label}
                  </span>
                  <span className="font-semibold text-slate-900 dark:text-white mt-0.5 block leading-tight">
                    {attr.value}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* SECTION 4: ДОКАЗАТЕЛЬСТВА (Evidence & Photo Mapping) */}
      {evidenceList.length > 0 && (
        <div data-testid="ai-evidence-section" className="space-y-2.5 border-t pt-3">
          <h3 className="text-xs font-bold text-slate-500 uppercase tracking-wider flex items-center gap-1.5">
            <ImageIcon className="w-3.5 h-3.5 text-indigo-500" />
            <span>Доказательства (привязка к фото)</span>
          </h3>

          <div className="space-y-2">
            {evidenceList.map((ev, idx) => (
              <div
                key={idx}
                className="p-2.5 bg-slate-50 dark:bg-slate-800/40 border border-slate-200 dark:border-slate-800 rounded-xl text-xs space-y-1.5"
              >
                <div className="flex items-center justify-between">
                  <span className="font-semibold text-slate-800 dark:text-slate-200">
                    {formatEvidenceLabel(ev, {
                      declaredCategory,
                      colors: [...dominantColors, ...secondaryColors],
                    })}
                  </span>
                  <span className="text-[10px] text-slate-400 font-mono">
                    {ev.imageIds.length} {ev.imageIds.length === 1 ? 'фото' : 'фото'}
                  </span>
                </div>

                {/* Thumbnails strip */}
                <div className="flex flex-wrap items-center gap-2">
                  {ev.imageIds.map((imgId) => {
                    const photo = findGalleryPhoto(imgId);
                    const photoIndex = gallery.findIndex((g) => g.id === imgId);
                    return photo ? (
                      <button
                        key={imgId}
                        type="button"
                        onClick={() => setSelectedEvidencePhoto(photo.url)}
                        title={`Фото #${photoIndex >= 0 ? photoIndex + 1 : '—'}`}
                        className="relative group rounded-lg overflow-hidden border border-slate-300 dark:border-slate-700 hover:border-indigo-500 w-9 h-11 bg-slate-100 flex-shrink-0 transition-transform hover:scale-105"
                      >
                        <img
                          src={photo.url}
                          alt={`Фото ${photoIndex + 1}`}
                          className="w-full h-full object-cover"
                        />
                        <span className="absolute bottom-0 right-0 bg-black/70 text-white text-[9px] px-1 rounded-tl font-mono">
                          #{photoIndex >= 0 ? photoIndex + 1 : ''}
                        </span>
                      </button>
                    ) : (
                      <span
                        key={imgId}
                        className="px-2 py-1 bg-slate-200 dark:bg-slate-700 rounded text-[10px] text-slate-600 dark:text-slate-400"
                      >
                        Фото
                      </span>
                    );
                  })}
                </div>
              </div>
            ))}
          </div>

          {/* Modal / Preview for Selected Evidence Photo */}
          {selectedEvidencePhoto && (
            <div
              className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
              onClick={() => setSelectedEvidencePhoto(null)}
            >
              <div
                className="bg-white dark:bg-slate-900 rounded-2xl p-3 max-w-sm w-full space-y-2 border border-slate-200 dark:border-slate-800 shadow-2xl"
                onClick={(e) => e.stopPropagation()}
              >
                <div className="flex justify-between items-center pb-1 border-b">
                  <span className="text-xs font-bold text-slate-900 dark:text-white">Фото-доказательство</span>
                  <button
                    onClick={() => setSelectedEvidencePhoto(null)}
                    className="text-xs text-slate-400 hover:text-slate-600"
                  >
                    Закрыть ✕
                  </button>
                </div>
                <div className="aspect-[3/4] rounded-xl overflow-hidden bg-slate-100">
                  <img src={selectedEvidencePhoto} alt="Evidence" className="w-full h-full object-contain" />
                </div>
              </div>
            </div>
          )}
        </div>
      )}

      {/* SECTION 5: ПОДРОБНЕЕ ОБ УВЕРЕННОСТИ МОДЕЛИ (Collapsible) */}
      {Object.keys(confidenceMap).length > 0 && (
        <div data-testid="ai-confidence-section" className="border-t pt-2">
          <button
            type="button"
            onClick={() => setShowConfidence(!showConfidence)}
            className="flex items-center justify-between w-full text-left py-1 text-xs text-slate-500 hover:text-slate-800 dark:hover:text-slate-200 transition-colors"
          >
            <span className="font-semibold flex items-center gap-1.5">
              <FileQuestion className="w-3.5 h-3.5" />
              <span>Подробнее об уверенности модели ({Object.keys(confidenceMap).length})</span>
            </span>
            {showConfidence ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
          </button>

          {showConfidence && (
            <div className="mt-2.5 p-3 bg-slate-50 dark:bg-slate-800/40 rounded-xl border border-slate-200 dark:border-slate-800 grid grid-cols-2 sm:grid-cols-4 gap-2 text-xs">
              {Object.entries(confidenceMap).map(([key, score]) => {
                const percent = Math.round((score?.score ?? 0) * 100);
                return (
                  <div key={key} className="p-2 rounded-lg bg-white dark:bg-slate-900 border">
                    <span className="text-[10px] text-slate-400 uppercase tracking-wider block">
                      {formatContradictionField(key)}
                    </span>
                    <div className="flex items-baseline justify-between mt-1">
                      <span className="font-bold text-slate-900 dark:text-white">{percent}%</span>
                      <span className="text-[10px] text-slate-400">{score?.level || '—'}</span>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
