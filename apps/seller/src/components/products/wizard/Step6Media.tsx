
import { useState, useEffect } from 'react';
import type { WizardState } from './WizardState';
import { getSellerColors, uploadSellerProductImage, cropSellerProductImage, setMainSellerProductImage, type SellerCategorySchema, type SellerColor } from '@zamk/api-client/src/seller';
import ReactCrop, { type Crop, type PercentCrop, centerCrop, makeAspectCrop } from 'react-image-crop';
import 'react-image-crop/dist/ReactCrop.css';

function centerAspectCrop(mediaWidth: number, mediaHeight: number, aspect: number): PercentCrop {
  return centerCrop(
    makeAspectCrop({ unit: '%', width: 90 }, aspect, mediaWidth, mediaHeight),
    mediaWidth,
    mediaHeight
  );
}

export function Step6Media({
  state,
  updateState,
  schema,
  onNext,
  onPrev,
  onError,
  onSaveDraft
}: {
  state: WizardState;
  updateState: (u: Partial<WizardState>) => void;
  schema: SellerCategorySchema | null;
  onNext: () => void;
  onPrev: () => void;
  onError?: (msg: string) => void;
  onSaveDraft?: () => Promise<string | undefined>;
}) {
  const [colors, setColors] = useState<SellerColor[]>([]);
  const [uploading, setUploading] = useState(false);
  const [cropModalOpen, setCropModalOpen] = useState(false);
  const [cropTarget, setCropTarget] = useState<{ id: string, url: string, colorId?: string } | null>(null);

  const [crop, setCrop] = useState<Crop>();
  const [cropError, setCropError] = useState<string | null>(null);
  const [completedPercentCrop, setCompletedPercentCrop] = useState<PercentCrop>();
  const [imageRef, setImageRef] = useState<HTMLImageElement | null>(null);

  useEffect(() => {
    getSellerColors().then(setColors).catch(console.error);
  }, []);

  const hasColor = (schema?.attributes || []).some(a => a.valueSource === 'VARIANT_COLOR') || false;

  const proceedUpload = async (file: File, colorId?: string) => {
    let productId = state.id;
    if (!productId && onSaveDraft) {
      setUploading(true);
      productId = await onSaveDraft();
      if (!productId) {
        setUploading(false);
        onError?.('Не удалось сохранить черновик. Попробуйте еще раз.');
        return;
      }
    } else if (!productId) {
      onError?.('Сначала сохраните черновик, чтобы загрузить фото.');
      return;
    }

    try {
      setUploading(true);
      const res = await uploadSellerProductImage(productId, file);

      if (colorId) {
        const nw = { ...state.colorImages };
        if (!nw[colorId]) nw[colorId] = [];
        nw[colorId] = [...nw[colorId], { id: res.id, url: res.imageUrl, sortOrder: nw[colorId].length, isMain: false, isReady: false }];
        updateState({ colorImages: nw });
      } else {
        updateState({ commonImages: [...state.commonImages, { id: res.id, url: res.imageUrl, sortOrder: state.commonImages.length, isMain: false, isReady: false }] });
      }
    } catch (err: any) {
      onError?.(err?.message || 'Не удалось загрузить изображение');
    } finally {
      setUploading(false);
    }
  };

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>, colorId?: string) => {
    const file = e.target.files?.[0];
    if (!file) return;

    // Client-side image pre-check for dimensions
    const img = new Image();
    const objectUrl = URL.createObjectURL(file);
    img.onload = () => {
      URL.revokeObjectURL(objectUrl);
      if (img.naturalWidth >= img.naturalHeight) {
        onError?.('Для товара нужны вертикальные фотографии.\nЗагрузите изображение в вертикальном формате.');
        e.target.value = '';
        return;
      }
      if (img.naturalWidth < 800 || img.naturalHeight < 1000) {
        onError?.('Изображение слишком маленькое.\nМинимальный размер — 800×1000 пикселей.');
        e.target.value = '';
        return;
      }
      proceedUpload(file, colorId);
      e.target.value = '';
    };
    img.onerror = () => {
      URL.revokeObjectURL(objectUrl);
      proceedUpload(file, colorId);
      e.target.value = '';
    };
    img.src = objectUrl;
  };

  const removeCommon = (idx: number) => {
    updateState({ commonImages: state.commonImages.filter((_, i) => i !== idx) });
  };

  const removeColorImg = (cId: string, idx: number) => {
    const nw = { ...state.colorImages };
    nw[cId] = nw[cId].filter((_, i) => i !== idx);
    updateState({ colorImages: nw });
  };

  const openCropModal = (img: { id: string, url: string }, colorId?: string) => {
    if (!img.id) {
      onError?.('Сначала сохраните черновик, чтобы настроить фото');
      return;
    }
    setCropTarget({ ...img, colorId });
    setCrop(undefined);
    setCompletedPercentCrop(undefined);
    setCropError(null);
    setCropModalOpen(true);
  };

  const onImageLoad = (e: React.SyntheticEvent<HTMLImageElement>) => {
    const { width, height } = e.currentTarget;
    setImageRef(e.currentTarget);
    const initialCrop = centerAspectCrop(width, height, 4 / 5);
    setCrop(initialCrop);
    setCompletedPercentCrop(initialCrop);
  };

  const handleSaveCrop = async () => {
    const activeCrop = completedPercentCrop || (crop?.unit === '%' ? (crop as PercentCrop) : undefined);
    if (!activeCrop || !cropTarget || !imageRef || !state.id) return;
    setCropError(null);

    const originalW = imageRef.naturalWidth;
    const originalH = imageRef.naturalHeight;
    if (originalW < 800 || originalH < 1000) {
      setCropError('Изображение слишком маленькое. Минимальный размер — 800×1000 пикселей.');
      return;
    }

    try {
      setUploading(true);
      const payload = {
        cropX: activeCrop.x / 100,
        cropY: activeCrop.y / 100,
        cropWidth: activeCrop.width / 100,
        cropHeight: activeCrop.height / 100
      };

      const res = await cropSellerProductImage(state.id, cropTarget.id, payload);

      // Just update isReady for this image
      const newCommon = state.commonImages.map(img =>
        img.id === cropTarget.id ? { ...img, isReady: res.renditionUrl != null } : img
      );

      const newColorImages = { ...state.colorImages };
      Object.keys(newColorImages).forEach(cId => {
        newColorImages[cId] = newColorImages[cId].map(img =>
          img.id === cropTarget.id ? { ...img, isReady: res.renditionUrl != null } : img
        );
      });

      updateState({ commonImages: newCommon, colorImages: newColorImages });
      setCropModalOpen(false);
    } catch(err: any) {
      setCropError(err.message || 'Не удалось сохранить кадр 4:5. Попробуйте выбрать область фотографии ещё раз.');
      onError?.(err.message || 'Не удалось сохранить кадр 4:5. Попробуйте выбрать область фотографии ещё раз.');
    } finally {
      setUploading(false);
    }
  };

  const handleSetMainImage = async (imgId: string) => {
    if (!state.id) return;
    try {
      setUploading(true);
      await setMainSellerProductImage(state.id, imgId);

      const newCommon = state.commonImages.map(img => ({
        ...img,
        isMain: img.id === imgId
      }));

      const newColorImages = { ...state.colorImages };
      Object.keys(newColorImages).forEach(cId => {
        newColorImages[cId] = newColorImages[cId].map(img => ({
          ...img,
          isMain: img.id === imgId
        }));
      });

      updateState({ commonImages: newCommon, colorImages: newColorImages });
    } catch(err: any) {
      onError?.(err.message);
    } finally {
      setUploading(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-medium">Фотографии товара</h2>
        <div className="text-sm text-gray-500">
          <p>Загружайте вертикальные фотографии. Все изображения товара показываются в формате 4:5.</p>
        </div>
      </div>

      <div>
        <h3 className="font-medium mb-2">Общие фото</h3>
        <div className="flex gap-2 flex-wrap">
          {state.commonImages.map((img, i) => (
            <div key={i} className="w-32 flex flex-col gap-1">
              <div className="relative w-32 h-40 border rounded bg-gray-100 flex items-center justify-center overflow-hidden group">
                {img.isMain && (
                  <div className="absolute top-0 left-0 right-0 bg-black/60 text-white text-xs text-center py-1 z-10">
                    ГЛАВНОЕ
                  </div>
                )}
                {!img.isMain && img.isReady && (
                  <div className="absolute top-0 left-0 right-0 bg-green-600/80 text-white text-xs text-center py-1 z-10">
                    ГОТОВО 4:5
                  </div>
                )}
                <img src={img.url} alt={`Фото ${i+1}`} className="w-full h-full object-cover" />
                <button onClick={() => removeCommon(i)} className="absolute top-1 right-1 bg-white rounded-full p-1 shadow text-red-500 hover:text-red-700 opacity-0 group-hover:opacity-100 z-10 transition-opacity">✕</button>
              </div>
              {!img.isReady && (
                 <button onClick={() => openCropModal(img)} className="text-xs text-blue-600 hover:underline text-center w-full">Настроить фото</button>
              )}
              {img.isReady && !img.isMain && (
                 <button onClick={() => handleSetMainImage(img.id)} className="text-xs text-blue-600 hover:underline text-center w-full">Сделать главным</button>
              )}
            </div>
          ))}
          <label className={`w-32 h-40 border border-dashed rounded flex flex-col items-center justify-center text-gray-500 hover:bg-gray-50 cursor-pointer ${uploading ? 'opacity-50 cursor-not-allowed' : ''}`}>
            {uploading ? (
              <span className="text-sm">Загрузка...</span>
            ) : (
              <span className="text-2xl">+</span>
            )}
            <input type="file" accept="image/*" className="hidden" onChange={(e) => handleUpload(e)} disabled={uploading} />
          </label>
        </div>
      </div>

      {hasColor && state.selectedColorIds.length > 0 && (
        <div className="mt-8 space-y-6">
          <h3 className="font-medium">Фото по цветам</h3>
          {state.selectedColorIds.map(cId => {
            const c = colors.find(x => x.id === cId);
            const imgs = state.colorImages[cId] || [];
            return (
              <div key={cId} className="p-4 border rounded">
                <h4 className="font-medium mb-2 flex items-center gap-2">
                  <span className="w-4 h-4 rounded-full border border-gray-200" style={{ backgroundColor: c?.hexValue }} />
                  {c?.nameRu}
                </h4>
                <div className="flex gap-2 flex-wrap">
                  {imgs.map((img, i) => (
                    <div key={i} className="w-32 flex flex-col gap-1">
                      <div className="relative w-32 h-40 border rounded bg-gray-100 flex items-center justify-center overflow-hidden group">
                        {img.isMain && (
                          <div className="absolute top-0 left-0 right-0 bg-black/60 text-white text-xs text-center py-1 z-10">
                            ГЛАВНОЕ
                          </div>
                        )}
                        {!img.isMain && img.isReady && (
                          <div className="absolute top-0 left-0 right-0 bg-green-600/80 text-white text-xs text-center py-1 z-10">
                            ГОТОВО 4:5
                          </div>
                        )}
                        <img src={img.url} alt={`Фото ${c?.nameRu} ${i+1}`} className="w-full h-full object-cover" />
                        <button onClick={() => removeColorImg(cId, i)} className="absolute top-1 right-1 bg-white rounded-full p-1 shadow text-red-500 hover:text-red-700 opacity-0 group-hover:opacity-100 z-10 transition-opacity">✕</button>
                      </div>
                      {!img.isReady && (
                        <button onClick={() => openCropModal(img, cId)} className="text-xs text-blue-600 hover:underline text-center w-full">Настроить фото</button>
                      )}
                      {img.isReady && !img.isMain && (
                        <button onClick={() => handleSetMainImage(img.id)} className="text-xs text-blue-600 hover:underline text-center w-full">Сделать главным</button>
                      )}
                    </div>
                  ))}
                  <label className={`w-32 h-40 border border-dashed rounded flex flex-col items-center justify-center text-gray-500 hover:bg-gray-50 cursor-pointer ${uploading ? 'opacity-50 cursor-not-allowed' : ''}`}>
                    {uploading ? (
                      <span className="text-sm">Загрузка...</span>
                    ) : (
                      <span className="text-2xl">+</span>
                    )}
                    <input type="file" accept="image/*" className="hidden" onChange={(e) => handleUpload(e, cId)} disabled={uploading} />
                  </label>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {cropModalOpen && cropTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80">
          <div className="bg-white rounded-lg p-6 max-w-4xl w-full max-h-[90vh] flex flex-col">
            <h3 className="text-lg font-medium mb-1">Настроить фото</h3>
            <p className="text-sm text-gray-500 mb-4">Выберите область, которая будет показана покупателю. Формат изображения — 4:5.</p>

            <div className="flex-1 overflow-auto bg-gray-50 flex items-center justify-center min-h-[400px]">
              <ReactCrop
                crop={crop}
                onChange={(_, percentCrop) => setCrop(percentCrop)}
                onComplete={(_, percentCrop) => setCompletedPercentCrop(percentCrop)}
                aspect={4 / 5}
                minHeight={20}
              >
                <img
                  src={cropTarget.url}
                  onLoad={onImageLoad}
                  className="max-h-[60vh] w-auto object-contain"
                  alt="Crop target"
                />
              </ReactCrop>
            </div>

            {cropError && <p className="text-red-500 text-sm mt-2 mb-1">{cropError}</p>}
            <div className="flex justify-end gap-2 mt-4 pt-4 border-t">
              <button onClick={() => setCropModalOpen(false)} className="px-4 py-2 border rounded">Отмена</button>
              <button onClick={handleSaveCrop} disabled={uploading} className="px-4 py-2 bg-black text-white rounded disabled:opacity-50">
                {uploading ? 'Сохранение...' : 'Готово 4:5'}
              </button>
            </div>
          </div>
        </div>
      )}

      <div className="flex justify-between pt-4">
        <button onClick={onPrev} className="px-4 py-2 border rounded">Назад</button>
        <button onClick={onNext} className="px-4 py-2 bg-black text-white rounded">Далее</button>
      </div>
    </div>
  );
}
