import { useState, useMemo, useRef, useEffect } from "react";
import { ProductPresentationCore, type ProductPresentationMediaItem } from "@zamk/shared";
import {
  getSellerColors,
  getSellerCategorySchema,
  getSellerSizeValues,
  type SellerColor,
  type SellerSizeValue,
  type SellerSizeSystem,
  type SellerCategorySchema,
} from "@zamk/api-client";
import { Pencil, Info, Link2 } from "lucide-react";
import { useProductStudio } from "../../contexts/ProductStudioContext";
import { cn } from "../../lib/utils";
import { ProductStudioPhotoColorModal } from "./ProductStudioPhotoColorModal";
import { ProductStudioCompositionModal } from "./ProductStudioCompositionModal";
import { ProductStudioCareModal } from "./ProductStudioCareModal";
import { ProductStudioCharacteristicsModal } from "./ProductStudioCharacteristicsModal";
import { ProductStudioSizeChartModal } from "./ProductStudioSizeChartModal";
import {
  mapStudioDraftToPresentation,
  computePresentationSizes,
  findMatchingDraftVariant,
  mapToPresentationSelectedVariant,
  findFirstMediaIndexForColor,
} from "./productStudioPresentationAdapter";
import {
  addColorToMatrix,
  addSizeToMatrix,
  removeColorFromMatrix,
  removeSizeFromMatrix,
} from "./productStudioMatrixHelper";
import {
  validateImageFile,
  getMediaProgressText,
  MIN_PRODUCT_IMAGES,
  MAX_PRODUCT_IMAGES,
  ALLOWED_IMAGE_MIME_TYPES,
} from "./productStudioMediaHelper";
import { getSizeChartCompleteness, getOfferedSizes, getCompositionCompleteness } from "./productStudioReadinessHelper";

export function ProductStudioVisualWorkspace() {
  const {
    draft,
    updateDraft,
    readiness,
    createMediaUrl,
    setCategoryModalOpen,
    markTouched,
    isFieldAttention,
  } = useProductStudio();

  const workspaceRef = useRef<HTMLDivElement>(null);

  // Local preview selection state (purely Visual-local, not persisted, no router)
  const [selectedColorId, setSelectedColorId] = useState<string | null>(null);
  const [selectedSizeId, setSelectedSizeId] = useState<string | null>(null);
  const [activeImage, setActiveImage] = useState<number>(0);

  // Popover state
  const [isColorPopoverOpen, setIsColorPopoverOpen] = useState(false);
  const [pendingSelectedColorIds, setPendingSelectedColorIds] = useState<Set<string>>(new Set());

  const [isSizePopoverOpen, setIsSizePopoverOpen] = useState(false);
  const [pendingSelectedSizeIds, setPendingSelectedSizeIds] = useState<Set<string>>(new Set());

  // In-place editing state
  const [isEditingTitle, setIsEditingTitle] = useState(false);
  const [titleValue, setTitleValue] = useState("");

  const [isEditingPrice, setIsEditingPrice] = useState(false);
  const [priceValue, setPriceValue] = useState("");

  const [isEditingDescription, setIsEditingDescription] = useState(false);
  const [descValue, setDescValue] = useState("");

  // Modals state
  const [isPhotoColorModalOpen, setIsPhotoColorModalOpen] = useState(false);
  const [isCompositionModalOpen, setIsCompositionModalOpen] = useState(false);
  const [isCareModalOpen, setIsCareModalOpen] = useState(false);
  const [isCharacteristicsModalOpen, setIsCharacteristicsModalOpen] = useState(false);
  const [isSizeChartModalOpen, setIsSizeChartModalOpen] = useState(false);
  const [showMediaInfo, setShowMediaInfo] = useState(false);

  // Reference dictionaries
  const [categorySchema, setCategorySchema] = useState<SellerCategorySchema | null>(null);
  const [colorsList, setColorsList] = useState<SellerColor[]>([]);
  const [allowedSizeSystems, setAllowedSizeSystems] = useState<SellerSizeSystem[]>([]);
  const [selectedSizeSystemId, setSelectedSizeSystemId] = useState<string | null>(null);
  const [sizeValuesList, setSizeValuesList] = useState<SellerSizeValue[]>([]);
  const [loadingSizes, setLoadingSizes] = useState(false);
  const [mediaError, setMediaError] = useState<string | null>(null);
  const [isValidatingPhoto, setIsValidatingPhoto] = useState(false);

  // Load colors once
  useEffect(() => {
    getSellerColors()
      .then((data) => setColorsList(data || []))
      .catch((err) => console.error("Failed to load seller colors:", err));
  }, []);

  // Load category schema and allowed size systems when categoryId changes
  useEffect(() => {
    if (!draft.categoryId) {
      setSizeValuesList([]);
      setAllowedSizeSystems([]);
      setSelectedSizeSystemId(null);
      setCategorySchema(null);
      return;
    }

    let isMounted = true;
    setLoadingSizes(true);

    getSellerCategorySchema(draft.categoryId)
      .then((schema) => {
        if (!isMounted) return;
        setCategorySchema(schema);
        const allowed = schema.allowedSizeSystems || [];
        setAllowedSizeSystems(allowed);
        if (allowed.length > 0) {
          const defaultSys = allowed.find((s: any) => s.isDefault) || allowed[0];
          setSelectedSizeSystemId(defaultSys.id);
        } else {
          setSelectedSizeSystemId(null);
          setSizeValuesList([]);
        }
      })
      .catch((err) => {
        console.error("Failed to load category schema:", err);
      })
      .finally(() => {
        if (isMounted) setLoadingSizes(false);
      });

    return () => {
      isMounted = false;
    };
  }, [draft.categoryId]);

  // Load size values when selectedSizeSystemId changes
  useEffect(() => {
    if (!selectedSizeSystemId) {
      setSizeValuesList([]);
      return;
    }

    let isMounted = true;
    setLoadingSizes(true);

    getSellerSizeValues(selectedSizeSystemId)
      .then((values) => {
        if (isMounted) {
          setSizeValuesList(values || []);
        }
      })
      .catch((err) => {
        console.error("Failed to load size values:", err);
      })
      .finally(() => {
        if (isMounted) setLoadingSizes(false);
      });

    return () => {
      isMounted = false;
    };
  }, [selectedSizeSystemId]);

  // Map draft to normalized presentation models
  const {
    product,
    visibleImages,
    colors,
    dimensionType,
    uniqueSizes,
    basePrice,
    hasVariants,
  } = useMemo(() => mapStudioDraftToPresentation(draft), [draft]);

  // Derive presentation sizes given current preview color selection
  const sizes = useMemo(
    () => computePresentationSizes(draft, dimensionType, uniqueSizes, selectedColorId),
    [draft, dimensionType, uniqueSizes, selectedColorId]
  );

  // Derive selected objects and matched variant
  const selectedColor = useMemo(
    () => (selectedColorId ? colors.find((c) => c.id === selectedColorId) || null : null),
    [colors, selectedColorId]
  );

  const selectedSize = useMemo(
    () => (selectedSizeId ? sizes.find((s) => s.id === selectedSizeId) || null : null),
    [sizes, selectedSizeId]
  );

  const matchingVariant = useMemo(
    () => findMatchingDraftVariant(draft, dimensionType, selectedColorId, selectedSizeId),
    [draft, dimensionType, selectedColorId, selectedSizeId]
  );

  const selectedVariant = useMemo(
    () => mapToPresentationSelectedVariant(matchingVariant),
    [matchingVariant]
  );

  // Display price: variant price if available, otherwise base draft price
  const displayPrice =
    selectedVariant?.priceCents !== undefined
      ? selectedVariant.priceCents / 100
      : basePrice;

  // Derive resolution & required flags
  const isExplicitOnlySize = draft.dimensionType === "ONLY_SIZE" || draft.dimensionType === "SIZE_ONLY";
  const isExplicitOnlyColor = draft.dimensionType === "ONLY_COLOR" || draft.dimensionType === "COLOR_ONLY";
  const isExplicitSingleVariant = draft.dimensionType === "SINGLE_VARIANT";

  const requiresColor = !isExplicitOnlySize && !isExplicitSingleVariant;
  const requiresSize = !isExplicitOnlyColor && !isExplicitSingleVariant;

  const isResolved = useMemo(() => {
    if (!hasVariants) return false;
    switch (dimensionType) {
      case "COLOR_AND_SIZE":
        return Boolean(selectedColorId && selectedSizeId);
      case "COLOR_ONLY":
        return Boolean(selectedColorId);
      case "SIZE_ONLY":
        return Boolean(selectedSizeId);
      case "SINGLE_VARIANT":
        return true;
    }
  }, [dimensionType, selectedColorId, selectedSizeId, hasVariants]);

  // Derive CTA text
  const ctaText = useMemo(() => {
    if (!hasVariants) return "Добавить в корзину";
    if (dimensionType === "COLOR_AND_SIZE") {
      if (!selectedColorId) return "Выберите цвет";
      if (!selectedSizeId) return "Выберите размер";
      return "Добавить в корзину";
    }
    if (dimensionType === "COLOR_ONLY") {
      if (!selectedColorId) return "Выберите цвет";
      return "Добавить в корзину";
    }
    if (dimensionType === "SIZE_ONLY") {
      if (!selectedSizeId) return "Выберите размер";
      return "Добавить в корзину";
    }
    return "Добавить в корзину";
  }, [dimensionType, selectedColorId, selectedSizeId, hasVariants]);

  const sizeSelectionNotice =
    dimensionType === "COLOR_AND_SIZE" && !selectedColorId
      ? "Сначала выберите цвет"
      : null;

  // Handlers for local preview interactions
  const handleColorChange = (colorId: string) => {
    setSelectedColorId(colorId);

    if (selectedSizeId) {
      const isOffered = (draft.variants || []).some(
        (v) => v.colorId === colorId && v.sizeValueId === selectedSizeId
      );
      if (!isOffered) {
        setSelectedSizeId(null);
      }
    }

    const targetIdx = findFirstMediaIndexForColor(visibleImages, colorId);
    setActiveImage(targetIdx);
  };

  const handleSizeChange = (sizeId: string) => {
    setSelectedSizeId(sizeId);
  };

  const handleActiveImageChange = (index: number) => {
    setActiveImage(index);
  };

  // Photo upload handler: local preview semantics only with validation
  const handlePhotoSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    setMediaError(null);
    setIsValidatingPhoto(true);

    try {
      const validation = await validateImageFile(file);
      if (!validation.valid) {
        setMediaError(validation.error || 'Недопустимый файл');
        return;
      }

      const objectUrl = createMediaUrl(file);

      const existingImages = draft.images || [];
      const isFirst = existingImages.length === 0;
      const newImage = {
        id: `local-media-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
        url: objectUrl,
        isMain: isFirst,
        sortOrder: existingImages.length,
        colorId: selectedColorId || null,
      };

      updateDraft({
        images: [...existingImages, newImage],
      });
    } finally {
      setIsValidatingPhoto(false);
      e.target.value = '';
    }
  };

  // Color popover open & atomic commit
  const handleOpenColorPopover = () => {
    setPendingSelectedColorIds(new Set((draft.colors || []).map((c: any) => c.id)));
    setIsColorPopoverOpen(true);
  };

  const handleApplyColors = () => {
    const currentColors = draft.colors || [];
    const removedColors = currentColors.filter((c: any) => !pendingSelectedColorIds.has(c.id));
    const addedColors = colorsList.filter(
      (c: any) => pendingSelectedColorIds.has(c.id) && !currentColors.some((dc: any) => dc.id === c.id)
    );

    let updatedVariants = draft.variants || [];
    for (const c of removedColors) {
      updatedVariants = removeColorFromMatrix(updatedVariants, c.id);
    }
    for (const c of addedColors) {
      updatedVariants = addColorToMatrix(updatedVariants, {
        id: c.id,
        name: c.nameRu,
        hex: c.hex || c.hexValue || "",
      });
    }

    const nextColors = colorsList
      .filter((c: any) => pendingSelectedColorIds.has(c.id))
      .map((c: any) => ({
        id: c.id,
        name: c.nameRu,
        hex: c.hex || c.hexValue || "",
      }));

    updateDraft({
      colors: nextColors,
      variants: updatedVariants,
    });

    if (selectedColorId && !pendingSelectedColorIds.has(selectedColorId)) {
      setSelectedColorId(nextColors[0]?.id || null);
    } else if (!selectedColorId && nextColors.length > 0) {
      setSelectedColorId(nextColors[0].id);
    }

    if (pendingSelectedColorIds.size === 0) {
      markTouched('color');
    }

    setIsColorPopoverOpen(false);
  };

  // Size popover open & atomic commit
  const handleOpenSizePopover = () => {
    setPendingSelectedSizeIds(
      new Set((draft.variants || []).map((v: any) => v.sizeValueId).filter(Boolean) as string[])
    );
    setIsSizePopoverOpen(true);
  };

  const handleApplySizes = () => {
    const existingSizeIds = new Set(
      (draft.variants || []).map((v: any) => v.sizeValueId).filter(Boolean) as string[]
    );
    const removedSizeIds = Array.from(existingSizeIds).filter(
      (id) => !pendingSelectedSizeIds.has(id)
    );
    const addedSizeIds = Array.from(pendingSelectedSizeIds).filter(
      (id) => !existingSizeIds.has(id)
    );

    let updatedVariants = draft.variants || [];
    for (const sId of removedSizeIds) {
      updatedVariants = removeSizeFromMatrix(updatedVariants, sId);
    }
    for (const sId of addedSizeIds) {
      const svObj = sizeValuesList.find((s: any) => s.id === sId);
      const label = svObj ? svObj.value : sId;
      updatedVariants = addSizeToMatrix(updatedVariants, {
        id: sId,
        label,
      });
    }

    updateDraft({
      variants: updatedVariants,
    });

    if (selectedSizeId && !pendingSelectedSizeIds.has(selectedSizeId)) {
      const remaining = Array.from(pendingSelectedSizeIds);
      setSelectedSizeId(remaining[0] || null);
    } else if (!selectedSizeId && pendingSelectedSizeIds.size > 0) {
      setSelectedSizeId(Array.from(pendingSelectedSizeIds)[0]);
    }

    if (pendingSelectedSizeIds.size === 0) {
      markTouched('size');
    }

    setIsSizePopoverOpen(false);
  };

  // Price commit helper
  const handlePriceCommit = (val: string) => {
    const clean = val.replace(/\s+/g, "").replace(",", ".");
    const rub = parseFloat(clean);
    if (!isNaN(rub) && rub >= 0) {
      updateDraft({ priceCents: Math.round(rub * 100) });
    }
  };

  const formatRub = (rub: number) =>
    new Intl.NumberFormat("ru-RU", {
      style: "currency",
      currency: "RUB",
      maximumFractionDigits: 0,
    }).format(rub);

  // Escape key handler
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setIsColorPopoverOpen(false);
        setIsSizePopoverOpen(false);
        setShowMediaInfo(false);
        setIsEditingTitle(false);
        setIsEditingPrice(false);
        setIsEditingDescription(false);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  // Global click capture to dismiss popovers when clicking outside
  const handleWorkspaceClickCapture = (e: React.MouseEvent) => {
    const target = e.target as HTMLElement;

    if (
      target.closest(".popover-content") ||
      target.closest(".popover-trigger") ||
      target.closest('[data-slot="title"]') ||
      target.closest('[data-slot="price"]') ||
      target.closest('[data-slot="description"]')
    ) {
      return;
    }

    setIsColorPopoverOpen(false);
    setIsSizePopoverOpen(false);
    setShowMediaInfo(false);
  };

  // Immediate blocker flags
  const isColorMissing = readiness?.blockingFields?.includes("color") ?? false;
  const isSizeMissing = readiness?.blockingFields?.includes("size") ?? false;
  const isCompositionMissing =
    readiness?.blockingFields?.includes("composition") ?? !getCompositionCompleteness(draft).isComplete;
  const isCharacteristicsMissing = readiness?.blockingFields?.includes("characteristics") ?? false;
  const offeredSizes = useMemo(() => getOfferedSizes(draft), [draft]);
  const sizeChartCompleteness = useMemo(() => {
    return getSizeChartCompleteness(draft, categorySchema);
  }, [draft, categorySchema]);

  // Interaction attention flags
  const isTitleAttention = isFieldAttention("title");
  const isPriceAttention = isFieldAttention("price");
  const isMediaAttention = isFieldAttention("media");
  const isColorAttention = isFieldAttention("color");
  const isSizeAttention = isFieldAttention("size");
  const isDescriptionAttention = isFieldAttention("description");
  const isCompositionAttention = isFieldAttention("composition");
  const isCharacteristicsAttention = isFieldAttention("characteristics");
  const isSizeChartAttention = isFieldAttention("sizeChart");

  const requiredProductAttrs = useMemo(() => {
    return categorySchema?.attributes?.filter((a) => a.scope === 'PRODUCT' && a.required) || [];
  }, [categorySchema]);

  const filledRequiredCharacteristicsCount = useMemo(() => {
    if (requiredProductAttrs.length === 0) return 0;
    return requiredProductAttrs.filter((reqAttr) => {
      const found = draft.attributes?.find(
        (a) =>
          a.attributeDefinitionId === reqAttr.id ||
          (reqAttr.nameRu && a.name === reqAttr.nameRu)
      );
      if (!found) return false;
      if (found.dictionaryValueId) return true;
      if (typeof found.value === 'string') return found.value.trim().length > 0;
      if (typeof found.value === 'number') return !isNaN(found.value);
      if (typeof found.value === 'boolean') return true;
      return false;
    }).length;
  }, [requiredProductAttrs, draft.attributes]);

  const titleSlot = isEditingTitle ? (
    <input
      type="text"
      data-slot="title"
      data-testid="visual-inline-input"
      autoFocus
      value={titleValue}
      placeholder="Например: Куртка-бомбер оверсайз"
      onChange={(e) => setTitleValue(e.target.value)}
      onKeyDown={(e) => {
        if (e.key === "Enter") {
          e.preventDefault();
          updateDraft({ title: titleValue.trim() });
          markTouched('title');
          setIsEditingTitle(false);
        } else if (e.key === "Escape") {
          e.preventDefault();
          setTitleValue(draft.title || "");
          setIsEditingTitle(false);
        }
      }}
      onBlur={() => {
        updateDraft({ title: titleValue.trim() });
        markTouched('title');
        setIsEditingTitle(false);
      }}
      className="w-full text-[26px] sm:text-[28px] min-[1200px]:text-[32px] leading-[32px] sm:leading-[34px] min-[1200px]:leading-[38px] font-serif text-graphite dark:text-white font-normal mt-1 bg-transparent border border-indigo-500 rounded px-1.5 py-0.5 outline-none ring-1 ring-indigo-500"
    />
  ) : (
    <div data-slot="title" className="group/title">
      <h1
        role="heading"
        aria-level={1}
        onClick={() => {
          setTitleValue(draft.title || "");
          setIsEditingTitle(true);
        }}
        className={cn(
          "text-[26px] sm:text-[28px] min-[1200px]:text-[32px] leading-[32px] sm:leading-[34px] min-[1200px]:leading-[38px] font-serif font-normal mt-1 cursor-text rounded-sm transition-colors",
          isTitleAttention
            ? "text-amber-700 dark:text-amber-400 border-b-2 border-dashed border-amber-400/80 hover:outline-dashed hover:outline-1 hover:outline-amber-400"
            : "text-graphite dark:text-white hover:outline-dashed hover:outline-1 hover:outline-ash/50"
        )}
        title="Нажмите, чтобы изменить название"
      >
        {draft.title?.trim() || "Название товара *"}
      </h1>
      {isTitleAttention && (
        <span data-testid="title-required-helper" className="block text-xs font-sans text-amber-700 dark:text-amber-400 font-medium mt-0.5">
          Укажите название
        </span>
      )}
    </div>
  );

  const priceSlot = isEditingPrice ? (
    <div className="mt-3.5 flex items-baseline gap-2" data-slot="price">
      <input
        type="text"
        data-slot="price"
        data-testid="visual-inline-input"
        autoFocus
        value={priceValue}
        placeholder="0"
        onChange={(e) => setPriceValue(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            handlePriceCommit(priceValue);
            markTouched('price');
            setIsEditingPrice(false);
          } else if (e.key === "Escape") {
            e.preventDefault();
            const currentRub = draft.priceCents !== undefined && draft.priceCents > 0 ? String(draft.priceCents / 100) : "";
            setPriceValue(currentRub);
            setIsEditingPrice(false);
          }
        }}
        onBlur={() => {
          handlePriceCommit(priceValue);
          markTouched('price');
          setIsEditingPrice(false);
        }}
        className="w-44 text-[26px] min-[1200px]:text-[28px] font-semibold text-graphite dark:text-white bg-transparent border border-indigo-500 rounded px-1.5 py-0.5 outline-none ring-1 ring-indigo-500"
      />
      <span className="text-base text-ash font-medium">₽</span>
    </div>
  ) : (
    <div
      data-slot="price"
      onClick={() => {
        const currentRub = draft.priceCents !== undefined && draft.priceCents > 0 ? String(draft.priceCents / 100) : "";
        setPriceValue(currentRub);
        setIsEditingPrice(true);
      }}
      className="mt-3.5 cursor-text group/price"
      title="Нажмите, чтобы изменить цену"
    >
      <div className="flex items-baseline gap-3">
        {product.discountPrice ? (
          <>
            <span
              data-testid="visual-display-price"
              className="text-[26px] min-[1200px]:text-[28px] font-semibold text-red-600 dark:text-red-400 group-hover/price:outline-dashed group-hover/price:outline-1 group-hover/price:outline-ash/50 rounded-sm px-1"
            >
              {formatRub(product.discountPrice)}
            </span>
            <span className="text-base text-ash line-through">
              {formatRub(displayPrice)}
            </span>
          </>
        ) : draft.priceCents !== undefined && draft.priceCents > 0 ? (
          <span
            data-testid="visual-display-price"
            className="text-[26px] min-[1200px]:text-[28px] font-semibold tracking-tight text-graphite dark:text-white font-sans group-hover/price:outline-dashed group-hover/price:outline-1 group-hover/price:outline-ash/50 rounded-sm px-1"
          >
            {formatRub(draft.priceCents / 100)}
          </span>
        ) : (
          <span
            data-testid="visual-display-price"
            className={cn(
              "text-[26px] min-[1200px]:text-[28px] font-semibold tracking-tight font-sans rounded-sm px-1 transition-colors",
              isPriceAttention
                ? "text-amber-700 dark:text-amber-400 border-b-2 border-dashed border-amber-400/80 group-hover/price:outline-dashed group-hover/price:outline-1 group-hover/price:outline-amber-400"
                : "text-ash/60 dark:text-white/40 group-hover/price:outline-dashed group-hover/price:outline-1 group-hover/price:outline-ash/50"
            )}
          >
            {displayPrice > 0 ? formatRub(displayPrice) : "Цена, ₽ *"}
          </span>
        )}
      </div>
      {isPriceAttention && (
        <span data-testid="price-required-helper" className="block text-xs font-sans text-amber-700 dark:text-amber-400 font-medium mt-0.5">
          Укажите цену
        </span>
      )}
    </div>
  );

  const descriptionSlot = isEditingDescription ? (
    <div data-slot="description" className="w-full">
      <textarea
        data-slot="description"
        data-testid="visual-inline-textarea"
        autoFocus
        value={descValue}
        placeholder="Расскажите о посадке, особенностях модели и важных деталях товара"
        onChange={(e) => setDescValue(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Escape") {
            e.preventDefault();
            setDescValue(draft.description || "");
            setIsEditingDescription(false);
          }
        }}
        onBlur={() => {
          updateDraft({ description: descValue });
          markTouched('description');
          setIsEditingDescription(false);
        }}
        className="w-full min-h-[140px] text-sm text-graphite-light dark:text-white/80 leading-relaxed bg-transparent border border-indigo-500 rounded p-2.5 outline-none ring-1 ring-indigo-500 resize-y"
      />
    </div>
  ) : (
    <div
      data-slot="description"
      data-testid="description-container"
      onClick={() => {
        setDescValue(draft.description || "");
        setIsEditingDescription(true);
      }}
      className={cn(
        "text-sm leading-relaxed whitespace-pre-line space-y-2 cursor-text rounded-xl p-3 transition-colors",
        isDescriptionAttention
          ? "bg-amber-50/40 dark:bg-amber-950/20 border border-dashed border-amber-300 dark:border-amber-800/70 hover:border-amber-400"
          : "text-graphite-light dark:text-white/80 hover:outline-dashed hover:outline-1 hover:outline-ash/50"
      )}
      title="Нажмите, чтобы изменить описание"
    >
      {draft.description?.trim() ? (
        <p className="text-graphite-light dark:text-white/80">{draft.description}</p>
      ) : (
        <div>
          <p className={cn(
            "text-sm font-medium",
            isDescriptionAttention ? "text-amber-700 dark:text-amber-400" : "text-ash hover:text-graphite dark:text-white/60 dark:hover:text-white"
          )}>
            Добавить описание *
          </p>
          {isDescriptionAttention ? (
            <span
              data-testid="description-required-helper"
              className="block text-xs text-amber-700/80 dark:text-amber-400/80 mt-1"
            >
              Добавьте описание товара
            </span>
          ) : (
            <span
              data-testid="description-required-helper"
              className="block text-xs text-ash mt-1"
            >
              Добавьте описание товара
            </span>
          )}
        </div>
      )}
    </div>
  );

  const renderMediaThumbnailOverlay = (image: ProductPresentationMediaItem, index: number) => {
    const assignedColor = (draft.colors || []).find((c: any) => c.id === image.colorId);
    if (!image.colorId || !assignedColor) return null;

    return (
      <span
        data-testid={`thumbnail-color-dot-${index}`}
        title={`Цвет: ${assignedColor.name}`}
        className="absolute bottom-1 right-1 w-2.5 h-2.5 rounded-full border border-white dark:border-black shadow-xs pointer-events-none z-10"
        style={{ backgroundColor: assignedColor.hex || "#000000" }}
      />
    );
  };

  const safeActiveImage = activeImage < visibleImages.length ? activeImage : 0;

  return (
    <div
      ref={workspaceRef}
      id="studio-workspace-visual"
      role="tabpanel"
      aria-labelledby="studio-tab-visual"
      data-testid="studio-visual-workspace"
      className="w-full relative"
      onClickCapture={handleWorkspaceClickCapture}
    >
      <div className="max-w-[1360px] mx-auto px-4 sm:px-6 lg:px-8 py-6 relative">
        {mediaError && visibleImages.length > 0 && (
          <div
            data-testid="media-upload-error-banner"
            className="mb-4 text-xs text-red-600 bg-red-50 dark:bg-red-950/40 p-3 rounded-lg border border-red-200 dark:border-red-900/50 flex items-center justify-between"
          >
            <span>{mediaError}</span>
            <button
              type="button"
              onClick={() => setMediaError(null)}
              className="text-red-500 hover:text-red-700 font-bold ml-2"
            >
              ✕
            </button>
          </div>
        )}

        <ProductPresentationCore
          product={product}
          visibleImages={visibleImages}
          activeImage={safeActiveImage}
          onActiveImageChange={handleActiveImageChange}
          displayPrice={displayPrice}
          emptyPricePlaceholder="Цена, ₽ *"
          colors={colors}
          sizes={sizes}
          colorLabelSuffix={requiresColor ? " *" : ""}
          colorLabelSuffixClassName={isColorAttention ? "text-amber-600 dark:text-amber-400" : "text-ash dark:text-gray-400"}
          sizeLabelSuffix={requiresSize ? " *" : ""}
          sizeLabelSuffixClassName={isSizeAttention ? "text-amber-600 dark:text-amber-400" : "text-ash dark:text-gray-400"}
          selectedColorId={selectedColorId}
          selectedSizeId={selectedSizeId}
          selectedColor={selectedColor}
          selectedSize={selectedSize}
          selectedVariant={selectedVariant}
          isResolved={isResolved}
          canAddToCart={isResolved}
          requiresColor={requiresColor}
          requiresSize={requiresSize}
          isAddingToCart={false}
          isProductUnavailable={false}
          ctaText={ctaText}
          sizeSelectionNotice={sizeSelectionNotice}
          refreshErrorNotice={null}
          sizeError=""
          titleSlot={titleSlot}
          priceSlot={priceSlot}
          descriptionSlot={descriptionSlot}
          renderThumbnailOverlay={renderMediaThumbnailOverlay}
          onColorChange={handleColorChange}
          onSizeChange={handleSizeChange}
          onAddToCart={() => {
            // Preview only: NO cart side effect, NO network request, NO toast
          }}
          isFavorite={false}
          onToggleFavorite={() => {
            // Preview only: NO favorite side effect
          }}
          onScrollToReviews={() => {
            // Preview only: NO review navigation
          }}
          onBrandClick={() => {
            // Preview only: NO navigation
          }}
          onDeliveryClick={() => {
            // Preview only: NO navigation
          }}
          onReturnsClick={() => {
            // Preview only: NO navigation
          }}
          onSellerClick={() => {
            // Preview only: NO navigation
          }}
          emptyMediaSlot={
            <label
              data-testid="presentation-empty-media-slot"
              className={cn(
                "w-full aspect-[4/5] max-h-[640px] rounded-xl flex flex-col items-center justify-center p-8 text-center cursor-pointer transition-colors group relative",
                isMediaAttention
                  ? "bg-amber-50/20 dark:bg-amber-950/10 border-2 border-dashed border-amber-300 dark:border-amber-700/50 hover:border-amber-400"
                  : "bg-paper-light dark:bg-paper-dark border-2 border-dashed border-ash/30 hover:border-graphite dark:hover:border-white/50"
              )}
            >
              <input
                type="file"
                accept={ALLOWED_IMAGE_MIME_TYPES.join(',')}
                className="hidden"
                aria-label="Добавить фото"
                data-testid="empty-stage-photo-input"
                onChange={handlePhotoSelect}
              />
              <div className={cn(
                "w-16 h-16 rounded-full flex items-center justify-center text-3xl font-light group-hover:scale-110 transition-transform mb-4",
                isMediaAttention
                  ? "bg-amber-100/60 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400"
                  : "bg-ash/10 dark:bg-white/10 text-graphite dark:text-white"
              )}>
                +
              </div>
              <span className={cn(
                "text-base font-semibold mb-1",
                isMediaAttention
                  ? "text-amber-700 dark:text-amber-400"
                  : "text-graphite dark:text-white"
              )}>
                Добавить фото *
              </span>
              <span className="text-xs font-medium text-graphite dark:text-white mb-1" data-testid="media-progress-badge">
                {getMediaProgressText(visibleImages.length)}
              </span>
              {isMediaAttention && (
                <span data-testid="media-required-helper" className="text-xs font-semibold text-amber-700 dark:text-amber-400 mb-1">
                  Нужно минимум 3 фото
                </span>
              )}
              <span className="text-xs text-ash dark:text-gray-400 mb-4">
                Первая — обложка
              </span>
              <div className="flex items-center gap-1.5 text-[11px] text-ash/80 dark:text-gray-400">
                <span>Вертикальные · от 800×1000 · JPG/PNG/WebP · до 10 МБ</span>
                <button
                  type="button"
                  onClick={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    setShowMediaInfo((prev) => !prev);
                  }}
                  title="Рекомендация по формату"
                  className="p-0.5 rounded-full hover:bg-black/10 dark:hover:bg-white/10 text-ash hover:text-graphite transition-colors"
                >
                  <Info className="w-3.5 h-3.5" />
                </button>
              </div>
              {showMediaInfo && (
                <div
                  className="mt-2 text-[10px] text-ash/90 dark:text-gray-300 max-w-xs bg-black/5 dark:bg-white/5 p-2 rounded-lg"
                  onClick={(e) => e.stopPropagation()}
                >
                  Лучше использовать формат 4:5 — фото лучше заполняет карточку товара
                </div>
              )}
              {isValidatingPhoto && (
                <p className="mt-3 text-xs text-indigo-600 dark:text-indigo-400 font-medium">Проверка файла...</p>
              )}
              {mediaError && (
                <p className="mt-3 text-xs text-red-500 font-medium" data-testid="media-upload-error">
                  {mediaError}
                </p>
              )}
            </label>
          }
          galleryExtraSlot={
            draft.images && draft.images.length > 0 ? (
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  data-testid="bind-photos-to-colors-btn"
                  disabled={!draft.colors || draft.colors.length === 0}
                  title={!draft.colors || draft.colors.length === 0 ? "Сначала добавьте цвета" : "Привязать фото к цветам"}
                  onClick={() => setIsPhotoColorModalOpen(true)}
                  className={cn(
                    "inline-flex items-center gap-1.5 text-xs font-medium py-1 transition-colors",
                    !draft.colors || draft.colors.length === 0
                      ? "text-ash/60 cursor-not-allowed"
                      : "text-indigo-600 dark:text-indigo-400 hover:text-indigo-700 dark:hover:text-indigo-300 cursor-pointer hover:underline"
                  )}
                >
                  <Link2 className="w-3.5 h-3.5" />
                  <span>Привязать фото к цветам</span>
                  {(!draft.colors || draft.colors.length === 0) && (
                    <span className="text-[11px] text-ash/80">(Сначала добавьте цвета)</span>
                  )}
                </button>
              </div>
            ) : null
          }
          mediaAddSlot={
            visibleImages.length < MAX_PRODUCT_IMAGES ? (
              <label
                className="w-16 h-20 min-[1200px]:w-[72px] min-[1200px]:h-[90px] flex-shrink-0 rounded-lg border-2 border-dashed border-ash/30 flex flex-col items-center justify-center cursor-pointer hover:border-graphite transition-colors hover:bg-gray-50/50 text-center p-1 group"
                title={getMediaProgressText(visibleImages.length)}
                data-testid="media-add-thumb-slot"
              >
                <input
                  type="file"
                  accept={ALLOWED_IMAGE_MIME_TYPES.join(',')}
                  className="hidden"
                  aria-label="Добавить фото"
                  onChange={handlePhotoSelect}
                />
                <span className="text-xl text-ash group-hover:text-graphite leading-none">+</span>
                <span className="text-[10px] text-ash group-hover:text-graphite font-medium mt-1 leading-tight">
                  {visibleImages.length} из {MIN_PRODUCT_IMAGES}
                </span>
              </label>
            ) : null
          }
          colorAddSlot={
            <div className="relative popover-trigger inline-flex items-center">
              <button
                type="button"
                data-testid="color-manage-btn"
                aria-label={colors.length > 0 ? "Управление цветами" : "Добавить цвет"}
                title={colors.length > 0 ? "Управление цветами" : isColorMissing ? "Выберите цвет" : "Добавить цвет"}
                onClick={(e) => {
                  e.stopPropagation();
                  if (isColorPopoverOpen) {
                    setIsColorPopoverOpen(false);
                  } else {
                    handleOpenColorPopover();
                  }
                }}
                className={cn(
                  "w-11 h-11 rounded-full border border-dashed flex items-center justify-center transition-colors cursor-pointer",
                  isColorAttention
                    ? "border-amber-400 text-amber-700 bg-amber-50/30 dark:bg-amber-950/20 hover:border-amber-500 hover:text-amber-800"
                    : "border-ash/50 text-ash hover:text-graphite hover:border-graphite"
                )}
              >
                {colors.length > 0 ? <Pencil className="w-4 h-4" /> : <span className="text-xl font-light leading-none">+</span>}
              </button>
              {isColorAttention && (
                <span data-testid="color-required-helper" className="text-xs text-amber-700 dark:text-amber-400 font-medium ml-2 shrink-0">
                  Выберите цвет
                </span>
              )}
              {isColorPopoverOpen && (
                <div
                  data-testid="color-popover"
                  className="absolute top-12 left-0 z-50 bg-white dark:bg-[#1a1a1c] border border-border-soft dark:border-white/10 rounded-xl shadow-xl p-4 w-72 popover-content"
                  onClick={(e) => e.stopPropagation()}
                >
                  <div className="flex items-center justify-between mb-3">
                    <h3 className="font-medium text-sm text-graphite dark:text-white">
                      Выберите цвет
                    </h3>
                    <button
                      type="button"
                      onClick={() => setIsColorPopoverOpen(false)}
                      className="text-xs text-ash hover:text-graphite dark:hover:text-white"
                    >
                      Отмена
                    </button>
                  </div>
                  <div className="max-h-60 overflow-y-auto space-y-1 mb-3">
                    {colorsList.map((color) => {
                      const hex = color.hex || color.hexValue || "#cccccc";
                      const isChecked = pendingSelectedColorIds.has(color.id);
                      return (
                        <label
                          key={color.id}
                          className="w-full flex items-center gap-3 px-2.5 py-1.5 rounded-lg hover:bg-black/5 dark:hover:bg-white/5 transition-colors cursor-pointer text-left text-sm"
                        >
                          <input
                            type="checkbox"
                            checked={isChecked}
                            onChange={() => {
                              const next = new Set(pendingSelectedColorIds);
                              if (next.has(color.id)) next.delete(color.id);
                              else next.add(color.id);
                              setPendingSelectedColorIds(next);
                            }}
                            className="rounded border-gray-300 dark:border-white/20 text-indigo-600 focus:ring-indigo-500"
                          />
                          <span
                            className="w-5 h-5 rounded-full border border-gray-300 dark:border-white/20 shrink-0 shadow-sm"
                            style={{ backgroundColor: hex }}
                          />
                          <span className="font-medium text-graphite dark:text-white truncate">
                            {color.nameRu}
                          </span>
                        </label>
                      );
                    })}
                  </div>
                  <button
                    type="button"
                    data-testid="color-popover-apply-btn"
                    onClick={handleApplyColors}
                    className="w-full h-9 bg-graphite text-white dark:bg-white dark:text-black rounded-lg text-xs font-semibold hover:opacity-90 transition-opacity cursor-pointer flex items-center justify-center"
                  >
                    Готово
                  </button>
                </div>
              )}
            </div>
          }
          sizeAddSlot={
            <div className="relative popover-trigger inline-flex items-center">
              <button
                type="button"
                data-testid="size-manage-btn"
                aria-label={sizes.length > 0 ? "Управление размерами" : "Добавить размер"}
                title={sizes.length > 0 ? "Управление размерами" : isSizeMissing ? "Выберите размер" : "Добавить размер"}
                onClick={(e) => {
                  e.stopPropagation();
                  if (isSizePopoverOpen) {
                    setIsSizePopoverOpen(false);
                  } else {
                    handleOpenSizePopover();
                  }
                }}
                className={cn(
                  "min-w-[48px] h-11 px-3.5 rounded-md border border-dashed flex items-center justify-center transition-colors cursor-pointer",
                  isSizeAttention
                    ? "border-amber-400 text-amber-700 bg-amber-50/30 dark:bg-amber-950/20 hover:border-amber-500 hover:text-amber-800"
                    : "border-ash/50 text-ash hover:text-graphite hover:border-graphite"
                )}
              >
                {sizes.length > 0 ? <Pencil className="w-4 h-4" /> : <span className="text-xl font-light leading-none">+</span>}
              </button>
              {isSizeAttention && (
                <span data-testid="size-required-helper" className="text-xs text-amber-700 dark:text-amber-400 font-medium ml-2 shrink-0">
                  Выберите размер
                </span>
              )}
              {isSizePopoverOpen && (
                <div
                  data-testid="size-popover"
                  className="absolute top-12 left-0 z-50 bg-white dark:bg-[#1a1a1c] border border-border-soft dark:border-white/10 rounded-xl shadow-xl p-4 w-80 popover-content"
                  onClick={(e) => e.stopPropagation()}
                >
                  <div className="flex items-center justify-between mb-3">
                    <h3 className="font-medium text-sm text-graphite dark:text-white">
                      Выберите размер
                    </h3>
                    <button
                      type="button"
                      onClick={() => setIsSizePopoverOpen(false)}
                      className="text-xs text-ash hover:text-graphite dark:hover:text-white"
                    >
                      Отмена
                    </button>
                  </div>

                  {!draft.categoryId ? (
                    <div className="space-y-3">
                      <p className="text-xs text-ash">Сначала выберите категорию товара</p>
                      <button
                        type="button"
                        onClick={() => {
                          setIsSizePopoverOpen(false);
                          markTouched('category');
                          setCategoryModalOpen(true);
                        }}
                        className="w-full h-10 border border-indigo-200 dark:border-indigo-900 bg-indigo-50 dark:bg-indigo-900/30 text-indigo-600 dark:text-indigo-400 rounded-md text-xs font-medium hover:bg-indigo-100 dark:hover:bg-indigo-900/50 transition-colors"
                      >
                        Выбрать категорию
                      </button>
                    </div>
                  ) : loadingSizes ? (
                    <p className="text-xs text-ash py-2">Загрузка размеров...</p>
                  ) : allowedSizeSystems.length === 0 ? (
                    <div className="py-2 space-y-2">
                      <p className="text-xs text-ash">
                        Для этой категории пока не настроены размеры
                      </p>
                      <button
                        type="button"
                        onClick={() => {
                          setIsSizePopoverOpen(false);
                          setCategoryModalOpen(true);
                        }}
                        className="w-full h-9 px-3 border border-indigo-200 dark:border-indigo-900 bg-indigo-50 dark:bg-indigo-900/30 text-indigo-600 dark:text-indigo-400 rounded-md text-xs font-medium hover:bg-indigo-100 dark:hover:bg-indigo-900/50 transition-colors"
                      >
                        Выбрать другую категорию
                      </button>
                    </div>
                  ) : (
                    <div>
                      {/* Size system switcher tabs if multiple */}
                      {allowedSizeSystems.length > 1 && (
                        <div className="flex items-center gap-1.5 mb-3 border-b border-gray-100 dark:border-white/10 pb-2 overflow-x-auto">
                          {allowedSizeSystems.map((sys) => (
                            <button
                              key={sys.id}
                              type="button"
                              onClick={() => setSelectedSizeSystemId(sys.id)}
                              className={`px-2.5 py-1 text-xs font-medium rounded-md transition-colors ${
                                selectedSizeSystemId === sys.id
                                  ? "bg-black text-white dark:bg-white dark:text-black font-semibold"
                                  : "text-ash hover:text-graphite dark:hover:text-white bg-gray-100 dark:bg-white/5"
                              }`}
                            >
                              {sys.name}
                            </button>
                          ))}
                        </div>
                      )}

                      {sizeValuesList.length === 0 ? (
                        <p className="text-xs text-ash py-2">
                          Нет доступных значений размера в выбранной системе.
                        </p>
                      ) : (
                        <div className="grid grid-cols-3 gap-1.5 max-h-60 overflow-y-auto mb-3">
                          {sizeValuesList.map((sv) => {
                            const isChecked = pendingSelectedSizeIds.has(sv.id);
                            return (
                              <button
                                key={sv.id}
                                type="button"
                                onClick={() => {
                                  const next = new Set(pendingSelectedSizeIds);
                                  if (next.has(sv.id)) next.delete(sv.id);
                                  else next.add(sv.id);
                                  setPendingSelectedSizeIds(next);
                                }}
                                className={`h-10 border rounded-md flex items-center justify-center text-xs font-medium transition-colors cursor-pointer ${
                                  isChecked
                                    ? "border-indigo-600 bg-indigo-50 dark:bg-indigo-950/40 text-indigo-700 dark:text-indigo-300 font-semibold ring-1 ring-indigo-600"
                                    : "border-border-soft dark:border-white/10 text-graphite dark:text-white hover:border-graphite dark:hover:border-white"
                                }`}
                              >
                                {sv.value}
                              </button>
                            );
                          })}
                        </div>
                      )}

                      <button
                        type="button"
                        data-testid="size-popover-apply-btn"
                        onClick={handleApplySizes}
                        className="w-full h-9 bg-graphite text-white dark:bg-white dark:text-black rounded-lg text-xs font-semibold hover:opacity-90 transition-opacity cursor-pointer flex items-center justify-center"
                      >
                        Готово
                      </button>
                    </div>
                  )}
                </div>
              )}
            </div>
          }
          sizeChartSlot={
            sizeChartCompleteness.isNeeded && (
              isSizeChartAttention ? (
                <button
                  type="button"
                  data-testid="size-chart-required-btn"
                  onClick={() => setIsSizeChartModalOpen(true)}
                  className="text-xs font-semibold text-amber-700 dark:text-amber-400 hover:text-amber-800 underline decoration-dashed underline-offset-2 transition-colors cursor-pointer"
                >
                  {sizeChartCompleteness.filledRequiredCellCount === 0
                    ? "Таблица размеров * · Не заполнена"
                    : `Таблица размеров * · заполнено ${sizeChartCompleteness.filledRequiredCellCount} из ${sizeChartCompleteness.requiredCellCount}`}
                </button>
              ) : !sizeChartCompleteness.isComplete ? (
                <button
                  type="button"
                  data-testid="size-chart-required-btn"
                  onClick={() => setIsSizeChartModalOpen(true)}
                  className="text-xs text-ash hover:text-graphite dark:hover:text-white underline underline-offset-2 transition-colors cursor-pointer"
                >
                  Таблица размеров *
                </button>
              ) : (
                <button
                  type="button"
                  data-testid="size-chart-btn"
                  onClick={() => setIsSizeChartModalOpen(true)}
                  className="text-xs text-ash hover:text-graphite dark:hover:text-white underline underline-offset-2 transition-colors cursor-pointer"
                >
                  {sizeChartCompleteness.requiredCellCount > 0
                    ? `Таблица размеров · заполнено ${sizeChartCompleteness.requiredCellCount} из ${sizeChartCompleteness.requiredCellCount}`
                    : "Таблица размеров"}
                </button>
              )
            )
          }
          characteristicsAddSlot={
            <div className="mt-4 pt-3 border-t border-border-lighter dark:border-white/5 flex items-center justify-between flex-wrap gap-2">
              <div>
                {!draft.categoryId ? (
                  <span className="text-xs text-ash">
                    Сначала выберите категорию товара
                  </span>
                ) : requiredProductAttrs.length > 0 ? (
                  <span
                    data-testid="characteristics-progress-badge"
                    className={cn(
                      "text-xs font-medium",
                      isCharacteristicsAttention
                        ? "text-amber-700 dark:text-amber-400 font-semibold"
                        : filledRequiredCharacteristicsCount === requiredProductAttrs.length
                        ? "text-emerald-600 dark:text-emerald-400"
                        : "text-ash"
                    )}
                  >
                    {filledRequiredCharacteristicsCount === requiredProductAttrs.length
                      ? `Характеристики · заполнено ${requiredProductAttrs.length} из ${requiredProductAttrs.length}`
                      : isCharacteristicsAttention
                      ? `Характеристики * · заполнено ${filledRequiredCharacteristicsCount} из ${requiredProductAttrs.length}`
                      : `Характеристики * · заполнено ${filledRequiredCharacteristicsCount} из ${requiredProductAttrs.length}`}
                  </span>
                ) : (
                  <span className="text-xs text-ash">Характеристики</span>
                )}
                {isCharacteristicsMissing && (
                  <p
                    data-testid="characteristics-required-helper"
                    className={cn(
                      "text-[11px]",
                      isCharacteristicsAttention
                        ? "text-amber-700/80 dark:text-amber-400/80 font-medium"
                        : "text-ash"
                    )}
                  >
                    Заполните обязательные характеристики категории
                  </p>
                )}
              </div>
              {!draft.categoryId ? (
                <button
                  type="button"
                  data-testid="choose-category-characteristics-btn"
                  onClick={() => {
                    markTouched('category');
                    setCategoryModalOpen(true);
                  }}
                  className="px-3 py-1.5 text-xs font-medium rounded-lg border transition-colors cursor-pointer bg-paper-light dark:bg-white/5 text-graphite dark:text-white border-border-soft dark:border-white/10 hover:bg-black/5 dark:hover:bg-white/10"
                >
                  Выбрать категорию
                </button>
              ) : (
                <button
                  type="button"
                  data-testid="manage-characteristics-btn"
                  onClick={() => {
                    setIsCharacteristicsModalOpen(true);
                  }}
                  className={cn(
                    "px-3 py-1.5 text-xs font-medium rounded-lg border transition-colors cursor-pointer",
                    isCharacteristicsAttention
                      ? "bg-amber-50 dark:bg-amber-950/20 text-amber-800 dark:text-amber-300 border-amber-300 dark:border-amber-800 hover:bg-amber-100"
                      : "bg-paper-light dark:bg-white/5 text-graphite dark:text-white border-border-soft dark:border-white/10 hover:bg-black/5 dark:hover:bg-white/10"
                  )}
                >
                  {isCharacteristicsAttention
                    ? "Заполнить характеристики"
                    : requiredProductAttrs.length > 0
                    ? "Изменить характеристики"
                    : "Настроить характеристики"}
                </button>
              )}
            </div>
          }
          compositionSlot={
            <div className="text-sm text-graphite-light dark:text-white/80 leading-relaxed space-y-3" data-testid="visual-composition-section">
              {/* Composition */}
              {isCompositionMissing ? (
                <div
                  onClick={() => setIsCompositionModalOpen(true)}
                  data-testid="composition-required-container"
                  className={cn(
                    "py-2 px-3 rounded-lg border-l-2 cursor-pointer transition-colors space-y-0.5",
                    isCompositionAttention
                      ? "border-amber-500 bg-amber-50/30 dark:bg-amber-950/15 hover:bg-amber-50/50 dark:hover:bg-amber-950/25"
                      : "border-border-soft bg-paper-light/40 dark:bg-white/[0.02] hover:bg-paper-light dark:hover:bg-white/[0.04]"
                  )}
                >
                  <div className="flex items-center justify-between">
                    <span className={cn(
                      "text-xs font-semibold",
                      isCompositionAttention
                        ? "text-amber-700 dark:text-amber-400"
                        : "text-graphite dark:text-white"
                    )}>
                      Состав *
                    </span>
                    <button
                      type="button"
                      data-testid="add-composition-btn"
                      onClick={(e) => {
                        e.stopPropagation();
                        setIsCompositionModalOpen(true);
                      }}
                      className={cn(
                        "text-xs font-semibold underline decoration-dashed cursor-pointer",
                        isCompositionAttention
                          ? "text-amber-700 dark:text-amber-400 hover:text-amber-800"
                          : "text-graphite dark:text-white hover:text-indigo-600"
                      )}
                    >
                      Добавить состав
                    </button>
                  </div>
                  <p
                    data-testid="composition-required-helper"
                    className={cn(
                      "text-xs",
                      isCompositionAttention
                        ? "text-amber-700/80 dark:text-amber-400/80"
                        : "text-ash"
                    )}
                  >
                    Укажите состав товара
                  </p>
                </div>
              ) : (
                <div className="space-y-1">
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <span className="text-xs font-medium text-ash block mb-0.5">Состав</span>
                      <p className="text-sm text-graphite dark:text-white font-medium">
                        {draft.materialComposition && draft.materialComposition.length > 0
                          ? draft.materialComposition.map((mc: any) => `${mc.materialName || mc.material} ${mc.percentage}%`).join(', ')
                          : draft.material}
                      </p>
                    </div>
                    <button
                      type="button"
                      data-testid="edit-composition-btn"
                      onClick={() => setIsCompositionModalOpen(true)}
                      className="text-xs text-ash hover:text-graphite dark:hover:text-white underline underline-offset-2 transition-colors cursor-pointer shrink-0 mt-0.5"
                    >
                      Изменить
                    </button>
                  </div>
                </div>
              )}

              {/* Care instructions */}
              <div className="pt-3 border-t border-border-lighter dark:border-white/5 space-y-2" data-testid="visual-care-section">
                {draft.careInstructions?.trim() ? (
                  <div className="space-y-1">
                    <div className="flex items-start justify-between gap-2">
                      <div>
                        <span className="text-xs font-medium text-ash block mb-0.5">Уход</span>
                        <p className="text-xs text-graphite dark:text-white">
                          {draft.careInstructions}
                        </p>
                      </div>
                      <button
                        type="button"
                        data-testid="edit-care-btn"
                        onClick={() => setIsCareModalOpen(true)}
                        className="text-xs text-ash hover:text-graphite dark:hover:text-white underline underline-offset-2 transition-colors cursor-pointer shrink-0 mt-0.5"
                      >
                        Изменить
                      </button>
                    </div>
                  </div>
                ) : (
                  <div className="space-y-1.5 p-2.5 rounded-lg bg-paper-light/60 dark:bg-white/[0.02] border border-border-soft/60 dark:border-white/5">
                    <div className="flex items-center justify-between">
                      <span className="text-xs font-medium text-graphite dark:text-white">
                        Уход
                      </span>
                      <span className="text-[11px] text-ash">
                        Необязательно
                      </span>
                    </div>
                    <p className="text-xs text-ash">
                      Рекомендации по стирке, сушке и глажке
                    </p>
                    <button
                      type="button"
                      data-testid="add-care-btn"
                      onClick={() => setIsCareModalOpen(true)}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-graphite dark:text-white bg-white dark:bg-white/10 border border-border-soft dark:border-white/10 rounded-lg hover:bg-black/5 dark:hover:bg-white/20 transition-colors cursor-pointer"
                    >
                      Добавить уход
                    </button>
                  </div>
                )}
              </div>
            </div>
          }
        />
      </div>

      <ProductStudioPhotoColorModal
        isOpen={isPhotoColorModalOpen}
        onClose={() => setIsPhotoColorModalOpen(false)}
        images={draft.images || []}
        colors={draft.colors || []}
        onSave={(updatedImages) => {
          updateDraft({ images: updatedImages });
          markTouched('media');
        }}
      />

      <ProductStudioCompositionModal
        isOpen={isCompositionModalOpen}
        onClose={() => {
          setIsCompositionModalOpen(false);
          markTouched('composition');
        }}
        materialComposition={draft.materialComposition}
        onSave={({ materialComposition, material }) => {
          updateDraft({ materialComposition, material });
          markTouched('composition');
        }}
      />

      <ProductStudioCareModal
        isOpen={isCareModalOpen}
        onClose={() => setIsCareModalOpen(false)}
        careInstructions={draft.careInstructions}
        onSave={(careInstructions) => updateDraft({ careInstructions })}
      />

      <ProductStudioCharacteristicsModal
        isOpen={isCharacteristicsModalOpen}
        onClose={() => {
          setIsCharacteristicsModalOpen(false);
          markTouched('characteristics');
        }}
        schema={categorySchema}
        attributes={draft.attributes}
        onSave={(attributes) => {
          updateDraft({ attributes });
          markTouched('characteristics');
        }}
      />

      <ProductStudioSizeChartModal
        isOpen={isSizeChartModalOpen}
        onClose={() => {
          setIsSizeChartModalOpen(false);
          markTouched('sizeChart');
        }}
        schema={categorySchema}
        draftSizes={offeredSizes.map((s) => ({ id: s.sizeValueId, label: s.sizeValueName }))}
        sizeChart={draft.sizeChart}
        onSaveSizeChart={(sizeChart) => {
          updateDraft({ sizeChart });
          markTouched('sizeChart');
        }}
      />
    </div>
  );
}
