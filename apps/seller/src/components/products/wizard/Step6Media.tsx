import { useState, useEffect } from 'react';
import type { WizardState } from './WizardState';
import { getSellerColors, uploadSellerProductImage, type SellerCategorySchema, type SellerColor } from '@zamk/api-client/src/seller';

export function Step6Media({ 
  state, 
  updateState, 
  schema, 
  onNext, 
  onPrev,
  onError
}: { 
  state: WizardState; 
  updateState: (u: Partial<WizardState>) => void; 
  schema: SellerCategorySchema | null;
  onNext: () => void; 
  onPrev: () => void;
  onError?: (msg: string) => void; 
}) {
  const [colors, setColors] = useState<SellerColor[]>([]);
  const [uploading, setUploading] = useState(false);

  useEffect(() => {
    getSellerColors().then(setColors).catch(console.error);
  }, []);

  const hasColor = (schema?.attributes || []).some(a => a.valueSource === 'VARIANT_COLOR') || false;

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>, colorId?: string) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (!state.id) {
      onError?.('Сначала сохраните черновик, чтобы загрузить фото.');
      return;
    }

    try {
      setUploading(true);
      const res = await uploadSellerProductImage(state.id, file);
      
      if (colorId) {
        const nw = { ...state.colorImages };
        if (!nw[colorId]) nw[colorId] = [];
        nw[colorId] = [...nw[colorId], { url: res.imageUrl, sortOrder: nw[colorId].length }];
        updateState({ colorImages: nw });
      } else {
        updateState({ commonImages: [...state.commonImages, { url: res.imageUrl, sortOrder: state.commonImages.length }] });
      }
    } catch (err: any) {
      onError?.('Ошибка загрузки: ' + err.message);
    } finally {
      setUploading(false);
      e.target.value = ''; // reset input
    }
  };

  const removeCommon = (idx: number) => {
    updateState({ commonImages: state.commonImages.filter((_, i) => i !== idx) });
  };
  
  const removeColorImg = (cId: string, idx: number) => {
    const nw = { ...state.colorImages };
    nw[cId] = nw[cId].filter((_, i) => i !== idx);
    updateState({ colorImages: nw });
  };

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-medium">Фото</h2>
      
      {!state.id && (
        <div className="bg-yellow-50 p-4 border border-yellow-200 rounded text-yellow-800 text-sm">
          Пожалуйста, сохраните черновик товара перед загрузкой фотографий.
        </div>
      )}

      <div>
        <h3 className="font-medium mb-2">Общие фото</h3>
        <div className="flex gap-2 flex-wrap">
          {state.commonImages.map((img, i) => (
            <div key={i} className="relative w-24 h-24 border rounded bg-gray-100 flex items-center justify-center overflow-hidden group">
              <img src={img.url} alt={`Фото ${i+1}`} className="w-full h-full object-cover" />
              <button onClick={() => removeCommon(i)} className="absolute top-1 right-1 bg-white rounded-full p-1 shadow text-red-500 hover:text-red-700 opacity-0 group-hover:opacity-100 transition-opacity">✕</button>
            </div>
          ))}
          <label className={`w-24 h-24 border border-dashed rounded flex flex-col items-center justify-center text-gray-500 hover:bg-gray-50 cursor-pointer ${uploading || !state.id ? 'opacity-50 cursor-not-allowed' : ''}`}>
            <span className="text-2xl">+</span>
            <input type="file" accept="image/*" className="hidden" onChange={(e) => handleUpload(e)} disabled={uploading || !state.id} />
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
                    <div key={i} className="relative w-24 h-24 border rounded bg-gray-100 flex items-center justify-center overflow-hidden group">
                      <img src={img.url} alt={`Фото ${c?.nameRu} ${i+1}`} className="w-full h-full object-cover" />
                      <button onClick={() => removeColorImg(cId, i)} className="absolute top-1 right-1 bg-white rounded-full p-1 shadow text-red-500 hover:text-red-700 opacity-0 group-hover:opacity-100 transition-opacity">✕</button>
                    </div>
                  ))}
                  <label className={`w-24 h-24 border border-dashed rounded flex flex-col items-center justify-center text-gray-500 hover:bg-gray-50 cursor-pointer ${uploading || !state.id ? 'opacity-50 cursor-not-allowed' : ''}`}>
                    <span className="text-2xl">+</span>
                    <input type="file" accept="image/*" className="hidden" onChange={(e) => handleUpload(e, cId)} disabled={uploading || !state.id} />
                  </label>
                </div>
              </div>
            );
          })}
        </div>
      )}

      <div className="flex justify-between pt-4">
        <button onClick={onPrev} className="px-4 py-2 border rounded">Назад</button>
        <button onClick={onNext} className="px-4 py-2 bg-black text-white rounded">Далее</button>
      </div>
    </div>
  );
}
