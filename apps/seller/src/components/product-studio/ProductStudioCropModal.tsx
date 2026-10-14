import React, { useState, useRef } from 'react';
import ReactCrop, { type Crop, type PercentCrop, centerCrop, makeAspectCrop } from 'react-image-crop';
import 'react-image-crop/dist/ReactCrop.css';
import { Loader2, Crop as CropIcon, X } from 'lucide-react';

export interface ProductStudioCropModalProps {
  isOpen: boolean;
  onClose: () => void;
  imageUrl: string;
  imageIndex: number;
  initialCrop?: {
    cropX?: number;
    cropY?: number;
    cropWidth?: number;
    cropHeight?: number;
  };
  onApplyCrop: (crop: {
    cropX: number;
    cropY: number;
    cropWidth: number;
    cropHeight: number;
  }) => Promise<void> | void;
}

function centerAspectCrop(mediaWidth: number, mediaHeight: number, aspect: number): PercentCrop {
  return centerCrop(
    makeAspectCrop(
      {
        unit: '%',
        width: 90,
      },
      aspect,
      mediaWidth,
      mediaHeight
    ),
    mediaWidth,
    mediaHeight
  );
}

export const ProductStudioCropModal: React.FC<ProductStudioCropModalProps> = ({
  isOpen,
  onClose,
  imageUrl,
  imageIndex,
  initialCrop,
  onApplyCrop,
}) => {
  const [crop, setCrop] = useState<Crop>();
  const [completedPercentCrop, setCompletedPercentCrop] = useState<PercentCrop>();
  const [isSaving, setIsSaving] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const imageRef = useRef<HTMLImageElement | null>(null);

  if (!isOpen) return null;

  const onImageLoad = (e: React.SyntheticEvent<HTMLImageElement>) => {
    const { width, height, naturalWidth, naturalHeight } = e.currentTarget;
    imageRef.current = e.currentTarget;

    if (
      initialCrop &&
      typeof initialCrop.cropWidth === 'number' &&
      typeof initialCrop.cropHeight === 'number' &&
      initialCrop.cropWidth > 0 &&
      initialCrop.cropHeight > 0
    ) {
      const init: PercentCrop = {
        unit: '%',
        x: (initialCrop.cropX || 0) * 100,
        y: (initialCrop.cropY || 0) * 100,
        width: initialCrop.cropWidth * 100,
        height: initialCrop.cropHeight * 100,
      };
      setCrop(init);
      setCompletedPercentCrop(init);
    } else {
      const initial = centerAspectCrop(width, height, 4 / 5);
      setCrop(initial);
      setCompletedPercentCrop(initial);
    }

    if (naturalWidth < 800 || naturalHeight < 1000) {
      setErrorMessage('Внимание: разрешение исходного изображения ниже рекомендуемого (800×1000 px).');
    }
  };

  const handleSave = async () => {
    const activeCrop = completedPercentCrop || (crop?.unit === '%' ? (crop as PercentCrop) : undefined);
    if (!activeCrop) {
      setErrorMessage('Пожалуйста, выберите область для кадрирования.');
      return;
    }

    try {
      setIsSaving(true);
      setErrorMessage(null);
      await onApplyCrop({
        cropX: activeCrop.x / 100,
        cropY: activeCrop.y / 100,
        cropWidth: activeCrop.width / 100,
        cropHeight: activeCrop.height / 100,
      });
      onClose();
    } catch (err: any) {
      setErrorMessage(err?.message || 'Не удалось применить кадрирование.');
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <div
      data-testid="studio-crop-modal-overlay"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/75 p-4 backdrop-blur-xs animate-in fade-in"
    >
      <div
        data-testid="studio-crop-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="studio-crop-modal-title"
        className="bg-white dark:bg-[#1a1a1c] border border-gray-200 dark:border-white/10 rounded-2xl p-6 max-w-3xl w-full max-h-[90vh] flex flex-col shadow-2xl overflow-hidden"
      >
        {/* Header */}
        <div className="flex items-center justify-between pb-3 border-b border-gray-200 dark:border-white/10">
          <div className="flex items-center gap-2">
            <CropIcon className="w-5 h-5 text-indigo-600 dark:text-indigo-400" />
            <h2 id="studio-crop-modal-title" className="text-base font-semibold text-gray-900 dark:text-white">
              Кадрирование 4:5 · Фото {imageIndex + 1}
            </h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            disabled={isSaving}
            aria-label="Закрыть"
            className="p-1 rounded-lg text-gray-400 hover:text-gray-700 dark:hover:text-white transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        <p className="text-xs text-gray-500 dark:text-gray-400 mt-2 mb-3">
          Настройте область 4:5 для витрины маркетплейса. Пропорции фиксированы (4:5) без искажений и растягивания.
        </p>

        {/* Crop canvas container */}
        <div className="flex-1 overflow-auto bg-gray-950/90 rounded-xl flex items-center justify-center min-h-[350px] max-h-[55vh] p-4">
          <ReactCrop
            crop={crop}
            onChange={(_, percentCrop) => setCrop(percentCrop)}
            onComplete={(_, percentCrop) => setCompletedPercentCrop(percentCrop)}
            aspect={4 / 5}
            minHeight={30}
          >
            <img
              src={imageUrl}
              onLoad={onImageLoad}
              alt={`Фото ${imageIndex + 1}`}
              className="max-h-[50vh] w-auto object-contain select-none"
            />
          </ReactCrop>
        </div>

        {errorMessage && (
          <p className="text-xs text-red-600 dark:text-red-400 mt-2 font-medium" data-testid="crop-modal-error">
            {errorMessage}
          </p>
        )}

        {/* Footer */}
        <div className="flex items-center justify-end gap-3 mt-4 pt-3 border-t border-gray-200 dark:border-white/10">
          <button
            type="button"
            onClick={onClose}
            disabled={isSaving}
            data-testid="crop-modal-cancel-btn"
            className="px-4 py-2 text-xs font-medium rounded-lg border border-gray-300 dark:border-white/10 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-white/5 transition-colors cursor-pointer"
          >
            Отмена
          </button>
          <button
            type="button"
            onClick={handleSave}
            disabled={isSaving}
            data-testid="crop-modal-apply-btn"
            className="inline-flex items-center gap-1.5 px-4 py-2 text-xs font-semibold rounded-lg bg-indigo-600 text-white hover:bg-indigo-700 transition-colors shadow-sm disabled:opacity-50 cursor-pointer"
          >
            {isSaving && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
            <span>{isSaving ? 'Сохранение...' : 'Применить кадрирование'}</span>
          </button>
        </div>
      </div>
    </div>
  );
};
