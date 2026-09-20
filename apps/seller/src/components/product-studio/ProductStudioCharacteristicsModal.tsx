import { useState, useEffect } from 'react';
import { X } from 'lucide-react';
import {
  getSellerDictionaryValues,
  type SellerCategorySchema,
  type SellerDictionaryValue,
} from '@zamk/api-client/src/seller';

export interface ProductStudioAttributeItem {
  attributeDefinitionId?: string;
  name?: string;
  code?: string;
  value?: any;
  dictionaryValueId?: string;
}

export interface ProductStudioCharacteristicsModalProps {
  isOpen: boolean;
  onClose: () => void;
  schema: SellerCategorySchema | null;
  attributes?: ProductStudioAttributeItem[];
  onSave: (attributes: ProductStudioAttributeItem[]) => void;
}

export function ProductStudioCharacteristicsModal({
  isOpen,
  onClose,
  schema,
  attributes = [],
  onSave,
}: ProductStudioCharacteristicsModalProps) {
  const [pendingAttributes, setPendingAttributes] = useState<Record<string, ProductStudioAttributeItem>>({});
  const [dictionaryValues, setDictionaryValues] = useState<Record<string, SellerDictionaryValue[]>>({});
  const [loadingDicts, setLoadingDicts] = useState<Record<string, boolean>>({});

  const productAttrs = (schema?.attributes || []).filter(
    (a) => a.scope === 'PRODUCT' && a.valueSource !== 'MATERIAL_COMPOSITION'
  );

  // Load dictionary values for any DICTIONARY attribute
  useEffect(() => {
    if (!isOpen || !schema) return;

    productAttrs.forEach((attr) => {
      if (attr.dictionaryId && !dictionaryValues[attr.dictionaryId] && !loadingDicts[attr.dictionaryId]) {
        setLoadingDicts((prev) => ({ ...prev, [attr.dictionaryId!]: true }));
        getSellerDictionaryValues(attr.dictionaryId)
          .then((vals) => {
            setDictionaryValues((prev) => ({ ...prev, [attr.dictionaryId!]: vals || [] }));
          })
          .catch(() => {
            setDictionaryValues((prev) => ({ ...prev, [attr.dictionaryId!]: [] }));
          })
          .finally(() => {
            setLoadingDicts((prev) => ({ ...prev, [attr.dictionaryId!]: false }));
          });
      }
    });
  }, [isOpen, schema]);

  // Sync initial state on open
  useEffect(() => {
    if (isOpen) {
      const initial: Record<string, ProductStudioAttributeItem> = {};
      attributes.forEach((attr) => {
        const key = attr.attributeDefinitionId || attr.name || '';
        if (key) {
          initial[key] = { ...attr };
        }
      });
      setPendingAttributes(initial);
    }
  }, [isOpen, attributes]);

  // Escape key handler
  useEffect(() => {
    if (!isOpen) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        onClose();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  const handleTextChange = (attrDef: typeof productAttrs[0], val: string) => {
    setPendingAttributes((prev) => ({
      ...prev,
      [attrDef.id]: {
        attributeDefinitionId: attrDef.id,
        name: attrDef.nameRu,
        code: attrDef.code,
        value: val,
      },
    }));
  };

  const handleNumberChange = (attrDef: typeof productAttrs[0], val: string) => {
    setPendingAttributes((prev) => ({
      ...prev,
      [attrDef.id]: {
        attributeDefinitionId: attrDef.id,
        name: attrDef.nameRu,
        code: attrDef.code,
        value: val === '' ? '' : Number(val),
      },
    }));
  };

  const handleBooleanChange = (attrDef: typeof productAttrs[0], checked: boolean) => {
    setPendingAttributes((prev) => ({
      ...prev,
      [attrDef.id]: {
        attributeDefinitionId: attrDef.id,
        name: attrDef.nameRu,
        code: attrDef.code,
        value: checked,
      },
    }));
  };

  const handleDictionaryChange = (attrDef: typeof productAttrs[0], dictValId: string) => {
    const dictOptions = (attrDef.dictionaryId && dictionaryValues[attrDef.dictionaryId]) || [];
    const matched = dictOptions.find((d) => d.id === dictValId);

    setPendingAttributes((prev) => ({
      ...prev,
      [attrDef.id]: {
        attributeDefinitionId: attrDef.id,
        name: attrDef.nameRu,
        code: attrDef.code,
        value: matched ? matched.nameRu : '',
        dictionaryValueId: dictValId || undefined,
      },
    }));
  };

  const handleApply = () => {
    const list: ProductStudioAttributeItem[] = Object.values(pendingAttributes).filter((attr) => {
      if (attr.dictionaryValueId) return true;
      if (typeof attr.value === 'boolean') return true;
      if (typeof attr.value === 'number') return !isNaN(attr.value);
      if (typeof attr.value === 'string') return attr.value.trim().length > 0;
      return false;
    });

    onSave(list);
    onClose();
  };

  const requiredAttrs = productAttrs.filter((a) => a.required);
  const optionalAttrs = productAttrs.filter((a) => !a.required);

  const renderAttributeInput = (attr: typeof productAttrs[0]) => {
    const current = pendingAttributes[attr.id];
    const isDict = attr.valueType === 'DICTIONARY' || attr.valueSource === 'DICTIONARY';

    if (isDict) {
      const options = (attr.dictionaryId && dictionaryValues[attr.dictionaryId]) || [];
      const selectedId = current?.dictionaryValueId || '';

      return (
        <select
          value={selectedId}
          onChange={(e) => handleDictionaryChange(attr, e.target.value)}
          data-testid={`characteristic-select-${attr.code || attr.id}`}
          className="w-full px-3 py-2 text-sm bg-white dark:bg-zinc-900 border border-border-soft dark:border-white/10 rounded-xl text-graphite dark:text-white focus:outline-hidden focus:border-graphite dark:focus:border-white"
        >
          <option value="">Не выбрано</option>
          {options.map((opt) => (
            <option key={opt.id} value={opt.id}>
              {opt.nameRu}
            </option>
          ))}
        </select>
      );
    }

    if (attr.valueType === 'BOOLEAN') {
      const isChecked = Boolean(current?.value);
      return (
        <label className="flex items-center gap-2.5 cursor-pointer py-1">
          <input
            type="checkbox"
            checked={isChecked}
            onChange={(e) => handleBooleanChange(attr, e.target.checked)}
            data-testid={`characteristic-checkbox-${attr.code || attr.id}`}
            className="w-4 h-4 rounded-sm border border-border-soft dark:border-white/20 text-graphite focus:ring-0 cursor-pointer"
          />
          <span className="text-sm text-graphite dark:text-white">Да</span>
        </label>
      );
    }

    if (attr.valueType === 'NUMBER') {
      const val = current?.value !== undefined ? String(current.value) : '';
      return (
        <input
          type="number"
          value={val}
          onChange={(e) => handleNumberChange(attr, e.target.value)}
          placeholder="Введите число"
          data-testid={`characteristic-input-${attr.code || attr.id}`}
          className="w-full px-3 py-2 text-sm bg-white dark:bg-zinc-900 border border-border-soft dark:border-white/10 rounded-xl text-graphite dark:text-white placeholder:text-ash/60 focus:outline-hidden focus:border-graphite dark:focus:border-white"
        />
      );
    }

    // Default TEXT
    const val = current?.value !== undefined ? String(current.value) : '';
    return (
      <input
        type="text"
        value={val}
        onChange={(e) => handleTextChange(attr, e.target.value)}
        placeholder="Введите значение"
        data-testid={`characteristic-input-${attr.code || attr.id}`}
        className="w-full px-3 py-2 text-sm bg-white dark:bg-zinc-900 border border-border-soft dark:border-white/10 rounded-xl text-graphite dark:text-white placeholder:text-ash/60 focus:outline-hidden focus:border-graphite dark:focus:border-white"
      />
    );
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-xs"
      data-testid="characteristics-modal"
      onClick={onClose}
    >
      <div
        className="w-full max-w-lg bg-white dark:bg-[#1a1a1c] border border-border-soft dark:border-white/10 rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[85vh]"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-start justify-between p-5 border-b border-border-soft dark:border-white/10">
          <div>
            <h2 className="text-base font-semibold text-graphite dark:text-white">
              Характеристики товара
            </h2>
            <p className="text-xs text-ash mt-1 leading-relaxed">
              {schema
                ? `Категория: ${schema.name}. Заполните канонические свойства модели.`
                : 'Категория не выбрана.'}
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Закрыть"
            className="p-1 rounded-lg text-ash hover:text-graphite dark:hover:text-white hover:bg-black/5 dark:hover:bg-white/5 transition-colors cursor-pointer ml-3 shrink-0"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Content */}
        <div className="p-5 overflow-y-auto space-y-6 flex-1">
          {!schema ? (
            <div className="p-8 text-center text-ash text-sm">
              Сначала выберите категорию товара для настройки характеристик.
            </div>
          ) : productAttrs.length === 0 ? (
            <div className="p-8 text-center text-ash text-sm">
              Для данной категории нет дополнительных характеристик.
            </div>
          ) : (
            <>
              {/* Required attributes */}
              {requiredAttrs.length > 0 && (
                <div className="space-y-4">
                  <div className="flex items-center gap-2 border-b border-border-soft dark:border-white/10 pb-2">
                    <span className="text-xs font-semibold uppercase tracking-wider text-graphite dark:text-white">
                      Обязательные характеристики
                    </span>
                    <span className="text-xs text-ash dark:text-gray-400 font-medium">*</span>
                  </div>
                  <div className="space-y-3">
                    {requiredAttrs.map((attr) => (
                      <div key={attr.id} data-testid={`characteristic-field-${attr.code || attr.id}`}>
                        <label className="block text-xs font-medium text-graphite dark:text-white mb-1.5">
                          {attr.nameRu} <span className="text-ash dark:text-gray-400 font-medium">*</span>
                        </label>
                        {renderAttributeInput(attr)}
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Optional attributes */}
              {optionalAttrs.length > 0 && (
                <div className="space-y-4">
                  <div className="flex items-center gap-2 border-b border-border-soft dark:border-white/10 pb-2">
                    <span className="text-xs font-semibold uppercase tracking-wider text-ash">
                      Дополнительные характеристики
                    </span>
                  </div>
                  <div className="space-y-3">
                    {optionalAttrs.map((attr) => (
                      <div key={attr.id} data-testid={`characteristic-field-${attr.code || attr.id}`}>
                        <label className="block text-xs font-medium text-graphite dark:text-white mb-1.5">
                          {attr.nameRu}
                        </label>
                        {renderAttributeInput(attr)}
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </>
          )}
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-border-soft dark:border-white/10 bg-gray-50/50 dark:bg-white/[0.02] flex items-center justify-end gap-3">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 text-xs font-medium text-graphite dark:text-white rounded-xl border border-border-soft dark:border-white/10 hover:bg-black/5 dark:hover:bg-white/5 transition-colors cursor-pointer"
          >
            Отмена
          </button>
          <button
            type="button"
            onClick={handleApply}
            data-testid="characteristics-modal-apply"
            className="px-5 py-2 text-xs font-medium text-white bg-graphite dark:bg-white dark:text-graphite rounded-xl hover:opacity-90 transition-opacity cursor-pointer shadow-xs"
          >
            Применить
          </button>
        </div>
      </div>
    </div>
  );
}
