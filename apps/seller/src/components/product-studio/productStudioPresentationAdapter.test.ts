import { describe, it, expect } from "vitest";
import {
  mapStudioDraftToPresentation,
  computePresentationSizes,
  computePresentationColors,
  findMatchingDraftVariant,
  mapToPresentationSelectedVariant,
  findFirstMediaIndexForColor,
  getCanonicalColorIds,
  resolveDeterministicPreviewColorId,
} from "./productStudioPresentationAdapter";
import type { ProductStudioDraft } from "../../contexts/ProductStudioContext";

describe("productStudioPresentationAdapter", () => {
  it("maps title, brand, description, and empty title fallback", () => {
    const emptyDraft: ProductStudioDraft = {
      title: "",
      description: "",
      categoryId: "cat-1",
    };
    const emptyRes = mapStudioDraftToPresentation(emptyDraft);
    expect(emptyRes.product.name).toBe("Название товара");
    expect(emptyRes.product.brand).toBeUndefined();

    const filledDraft: ProductStudioDraft = {
      title: "  Летнее платье  ",
      brandName: "Acne Studios",
      brandId: "brand-123",
      categoryName: "Платья",
      description: "Легкое платье из шелка",
      material: "100% шелк",
      materialComposition: [{ materialName: "Шелк", percentage: 100 }],
      careInstructions: "Ручная стирка",
      categoryId: "cat-1",
    };
    const filledRes = mapStudioDraftToPresentation(filledDraft);
    expect(filledRes.product.name).toBe("  Летнее платье  ");
    expect(filledRes.product.brand).toBe("Acne Studios");
    expect(filledRes.product.brandId).toBe("brand-123");
    expect(filledRes.product.category).toBe("Платья");
    expect(filledRes.product.description).toBe("Легкое платье из шелка");
    expect(filledRes.product.materials).toBe("100% шелк");
    expect(filledRes.product.materialComposition).toEqual([
      { materialName: "Шелк", material: "Шелк", percentage: 100 },
    ]);
    expect(filledRes.product.careInstructions).toBe("Ручная стирка");
  });

  it("maps price cents to rubles exactly once without mixing currencies and follows semantic exact mapping", () => {
    const draft: ProductStudioDraft = {
      title: "Худи",
      description: "",
      categoryId: "cat-1",
      priceCents: 450000, // 4500 руб (new selling price)
      oldPriceCents: 600000, // 6000 руб (old base price)
    };
    const res = mapStudioDraftToPresentation(draft);
    expect(res.basePrice).toBe(6000);
    expect(res.product.price).toBe(6000);
    expect(res.product.discountPrice).toBe(4500);

    const noPriceDraft: ProductStudioDraft = {
      title: "Худи",
      description: "",
      categoryId: "cat-1",
    };
    const noPriceRes = mapStudioDraftToPresentation(noPriceDraft);
    expect(noPriceRes.basePrice).toBe(0);
    expect(noPriceRes.product.price).toBe(0);
    expect(noPriceRes.product.discountPrice).toBeUndefined();
  });

  it("sorts media respecting isMain and sortOrder, and handles empty media state", () => {
    const emptyDraft: ProductStudioDraft = {
      title: "Худи",
      description: "",
      categoryId: "cat-1",
      images: [],
    };
    const emptyRes = mapStudioDraftToPresentation(emptyDraft);
    expect(emptyRes.visibleImages).toEqual([]);

    const draftWithImages: ProductStudioDraft = {
      title: "Худи",
      description: "",
      categoryId: "cat-1",
      images: [
        {
          uiKey: 'img-3',
          sortOrder: 3,
          colorId: 'c-black',
          isMain: false,
          source: { kind: 'canonical', imageId: 'img-3', url: 'img-3.jpg' },
        },
        {
          uiKey: 'img-1',
          sortOrder: 1,
          isMain: true,
          colorId: 'c-white',
          source: { kind: 'canonical', imageId: 'img-1', url: 'img-1.jpg' },
        },
        {
          uiKey: 'img-2',
          sortOrder: 2,
          isMain: false,
          source: { kind: 'canonical', imageId: 'img-2', url: 'img-2.jpg' },
        },
      ],
    };
    const res = mapStudioDraftToPresentation(draftWithImages);
    expect(res.visibleImages.map((i) => i.url)).toEqual(["img-1.jpg", "img-2.jpg", "img-3.jpg"]);
    expect(res.visibleImages[0].colorId).toBe("c-white");
  });

  it("extracts colors and uniqueSizes keyed strictly by canonical IDs", () => {
    const draft: ProductStudioDraft = {
      title: "Худи",
      description: "",
      categoryId: "cat-1",
      colors: [
        { id: "c-black", name: "Черный", hex: "#000000" },
        { id: "c-white", name: "Белый", hex: "#ffffff" },
      ],
      variants: [
        { id: "v1", colorId: "c-black", sizeValueId: "s-m", size: "M" },
        { id: "v2", colorId: "c-black", sizeValueId: "s-l", size: "L" },
        { id: "v3", colorId: "c-white", sizeValueId: "s-m", size: "M" },
      ],
    };
    const res = mapStudioDraftToPresentation(draft);
    expect(res.colors).toHaveLength(2);
    expect(res.colors.map((c) => c.id)).toEqual(["c-black", "c-white"]);
    expect(res.colors.every((c) => c.hasInStock)).toBe(true);

    expect(res.uniqueSizes).toHaveLength(2);
    expect(res.uniqueSizes.map((s) => s.id)).toEqual(["s-m", "s-l"]);
    expect(res.uniqueSizes.map((s) => s.label)).toEqual(["M", "L"]);
  });

  it("derives dimensionType correctly for all dimension modes", () => {
    // COLOR_AND_SIZE
    const fullDraft: ProductStudioDraft = {
      title: "Худи",
      description: "",
      categoryId: "cat-1",
      variants: [
        { id: "v1", colorId: "c-black", sizeValueId: "s-m", size: "M" },
      ],
    };
    expect(mapStudioDraftToPresentation(fullDraft).dimensionType).toBe("COLOR_AND_SIZE");

    // COLOR_ONLY
    const colorOnlyDraft: ProductStudioDraft = {
      title: "Шарф",
      description: "",
      categoryId: "cat-1",
      colors: [{ id: "c-red", name: "Красный" }],
      variants: [{ id: "v1", colorId: "c-red" }],
    };
    expect(mapStudioDraftToPresentation(colorOnlyDraft).dimensionType).toBe("COLOR_ONLY");

    // SIZE_ONLY
    const sizeOnlyDraft: ProductStudioDraft = {
      title: "Кольцо",
      description: "",
      categoryId: "cat-1",
      variants: [
        { id: "v1", sizeValueId: "sz1", size: "16" },
        { id: "v2", sizeValueId: "sz2", size: "17" },
      ],
    };
    expect(mapStudioDraftToPresentation(sizeOnlyDraft).dimensionType).toBe("SIZE_ONLY");

    // SINGLE_VARIANT (1 variant)
    const singleDraft: ProductStudioDraft = {
      title: "Книга",
      description: "",
      categoryId: "cat-1",
      variants: [{ id: "v1" }],
    };
    expect(mapStudioDraftToPresentation(singleDraft).dimensionType).toBe("SINGLE_VARIANT");
    expect(mapStudioDraftToPresentation(singleDraft).hasVariants).toBe(true);

    // EMPTY DRAFT (0 variants)
    const emptyVariantsDraft: ProductStudioDraft = {
      title: "Пусто",
      description: "",
      categoryId: "cat-1",
      variants: [],
    };
    expect(mapStudioDraftToPresentation(emptyVariantsDraft).dimensionType).toBe("SINGLE_VARIANT");
    expect(mapStudioDraftToPresentation(emptyVariantsDraft).hasVariants).toBe(false);
  });

  it("computes size matrix keeping all sizes visible: incompatible sizes are marked disabled (PS.R4B3.1C4C3B2G1)", () => {
    const draft: ProductStudioDraft = {
      title: "Худи",
      description: "",
      categoryId: "cat-1",
      variants: [
        { id: "v1", colorId: "c1", sizeValueId: "s1", size: "S" },
        { id: "v2", colorId: "c1", sizeValueId: "s2", size: "M" },
        { id: "v3", colorId: "c2", sizeValueId: "s1", size: "S" }, // c2 has only S, no M
      ],
    };
    const uniqueSizes = [
      { id: "s1", label: "S" },
      { id: "s2", label: "M" },
    ];

    // Before color selection: all sizes are visible and enabled
    const beforeColor = computePresentationSizes(draft, "COLOR_AND_SIZE", uniqueSizes, null);
    expect(beforeColor).toEqual([
      { id: "s1", label: "S", state: "AVAILABLE", disabled: false },
      { id: "s2", label: "M", state: "AVAILABLE", disabled: false },
    ]);

    // After selecting c1: both S and M are compatible (enabled)
    const c1Sizes = computePresentationSizes(draft, "COLOR_AND_SIZE", uniqueSizes, "c1");
    expect(c1Sizes).toEqual([
      { id: "s1", label: "S", state: "AVAILABLE", disabled: false },
      { id: "s2", label: "M", state: "AVAILABLE", disabled: false },
    ]);

    // After selecting c2: both S and M remain visible; S is enabled, M is disabled
    const c2Sizes = computePresentationSizes(draft, "COLOR_AND_SIZE", uniqueSizes, "c2");
    expect(c2Sizes).toEqual([
      { id: "s1", label: "S", state: "AVAILABLE", disabled: false },
      { id: "s2", label: "M", state: "NOT_OFFERED", disabled: true },
    ]);
  });

  it("computes color options keeping all colors visible: incompatible colors are marked disabled (PS.R4B3.1C4C3B2G1)", () => {
    const draft: ProductStudioDraft = {
      title: "Худи",
      description: "",
      categoryId: "cat-1",
      variants: [
        { id: "v1", colorId: "c-black", sizeValueId: "s-m", size: "M" },
        { id: "v2", colorId: "c-black", sizeValueId: "s-l", size: "L" },
        { id: "v3", colorId: "c-white", sizeValueId: "s-m", size: "M" }, // white only has M
      ],
    };
    const allColors = [
      { id: "c-black", name: "Черный", hasInStock: true },
      { id: "c-white", name: "Белый", hasInStock: true },
    ];

    // No size selected -> all colors visible and enabled
    const noSizeColors = computePresentationColors(draft, allColors, null);
    expect(noSizeColors).toEqual([
      { id: "c-black", name: "Черный", hasInStock: true, state: "AVAILABLE", disabled: false },
      { id: "c-white", name: "Белый", hasInStock: true, state: "AVAILABLE", disabled: false },
    ]);

    // Size M selected -> both black and white offer M (both enabled)
    const mColors = computePresentationColors(draft, allColors, "s-m");
    expect(mColors).toEqual([
      { id: "c-black", name: "Черный", hasInStock: true, state: "AVAILABLE", disabled: false },
      { id: "c-white", name: "Белый", hasInStock: true, state: "AVAILABLE", disabled: false },
    ]);

    // Size L selected -> both black and white remain visible; black is enabled, white is disabled
    const lColors = computePresentationColors(draft, allColors, "s-l");
    expect(lColors).toEqual([
      { id: "c-black", name: "Черный", hasInStock: true, state: "AVAILABLE", disabled: false },
      { id: "c-white", name: "Белый", hasInStock: true, state: "UNAVAILABLE", disabled: true },
    ]);
  });

  it("findFirstMediaIndexForColor correctly locates matching media or falls back to 0", () => {
    const images = [
      { url: "main.jpg" },
      { url: "black-1.jpg", colorId: "black" },
      { url: "white-1.jpg", colorId: "white" },
      { url: "black-2.jpg", colorId: "black" },
    ];

    expect(findFirstMediaIndexForColor(images, null)).toBe(0);
    expect(findFirstMediaIndexForColor(images, "black")).toBe(1);
    expect(findFirstMediaIndexForColor(images, "white")).toBe(2);
    expect(findFirstMediaIndexForColor(images, "green")).toBe(0); // fallback
  });

  it("findMatchingDraftVariant and mapToPresentationSelectedVariant resolve correctly", () => {
    const draft: ProductStudioDraft = {
      title: "Худи",
      description: "",
      categoryId: "cat-1",
      variants: [
        { id: "v1", colorId: "c1", sizeValueId: "s1", size: "S", sellerSku: "SKU-S", priceCents: 200000 },
        { id: "v2", colorId: "c1", sizeValueId: "s2", size: "M", sellerSku: "SKU-M", priceCents: 220000 },
      ],
    };

    const matched = findMatchingDraftVariant(draft, "COLOR_AND_SIZE", "c1", "s1");
    expect(matched?.id).toBe("v1");

    const mapped = mapToPresentationSelectedVariant(matched);
    expect(mapped).toEqual({
      id: "v1",
      sellerSku: "SKU-S",
      sku: "SKU-S",
      priceCents: 200000,
      size: "S",
    });

    expect(mapToPresentationSelectedVariant(null)).toBeNull();
  });

  describe("PS.R4B3.1C4C3B2G — Peer Preview Selection Model", () => {
    it("0. getCanonicalColorIds extracts unique canonical color IDs from colors and variants", () => {
      const draft: ProductStudioDraft = {
        title: "Товар",
        description: "",
        categoryId: "cat-1",
        colors: [{ id: "c-black", name: "Черный", hex: "#000000" }],
        variants: [
          { id: "v1", colorId: "c-black", sizeValueId: "s-1", size: "M" },
          { id: "v2", colorId: "c-white", sizeValueId: "s-1", size: "M" },
        ],
      };
      expect(getCanonicalColorIds(draft)).toEqual(["c-black", "c-white"]);
    });

    it("1. initial / null state -> resolveDeterministicPreviewColorId returns null even for 1 color", () => {
      const singleColorDraft: ProductStudioDraft = {
        title: "Товар",
        description: "",
        categoryId: "cat-1",
        colors: [{ id: "c-black", name: "Черный", hex: "#000000" }],
      };
      expect(resolveDeterministicPreviewColorId(singleColorDraft, null)).toBeNull();
    });

    it("2. preserves current valid selectedColorId", () => {
      const draft: ProductStudioDraft = {
        title: "Товар",
        description: "",
        categoryId: "cat-1",
        colors: [
          { id: "c-black", name: "Черный", hex: "#000000" },
          { id: "c-white", name: "Белый", hex: "#FFFFFF" },
        ],
      };
      expect(resolveDeterministicPreviewColorId(draft, "c-white")).toBe("c-white");
      expect(resolveDeterministicPreviewColorId(draft, "c-black")).toBe("c-black");
    });

    it("3. clears selection when previous selection is no longer valid", () => {
      const draft: ProductStudioDraft = {
        title: "Товар",
        description: "",
        categoryId: "cat-1",
        colors: [
          { id: "c-black", name: "Черный", hex: "#000000" },
          { id: "c-white", name: "Белый", hex: "#FFFFFF" },
        ],
      };
      expect(resolveDeterministicPreviewColorId(draft, "c-blue")).toBeNull();
    });
  });
});
