import { describe, it, expect } from 'vitest';
import {
  findMediaIndexForColor,
  deduplicateGalleryImages,
  getVisibleGalleryImages,
  derivePresentationMediaMode,
  getDeterministicDefaultColorId,
  type GalleryMediaItem,
} from './mediaFocus';

describe('SHOP PDP.2E1 Color-Aware Media Focus Logic (mediaFocus.ts)', () => {
  describe('findMediaIndexForColor', () => {
    const sampleGallery: GalleryMediaItem[] = [
      { url: 'https://example.com/general-hero.jpg' }, // 0: general
      { url: 'https://example.com/black-1.jpg', colorId: 'color-black' }, // 1: black #1
      { url: 'https://example.com/white-1.jpg', colorId: 'color-white' }, // 2: white #1
      { url: 'https://example.com/black-2.jpg', colorId: 'color-black' }, // 3: black #2
      { url: 'https://example.com/general-fabric.jpg' }, // 4: general
    ];

    it('returns 0 if images array is empty or undefined', () => {
      expect(findMediaIndexForColor([], 'color-black')).toBe(0);
      expect(findMediaIndexForColor(undefined, 'color-black')).toBe(0);
      expect(findMediaIndexForColor(null, 'color-black')).toBe(0);
    });

    it('returns the FIRST image tagged with the selected color', () => {
      // Black: should pick index 1, not index 3
      expect(findMediaIndexForColor(sampleGallery, 'color-black')).toBe(1);
      // White: should pick index 2
      expect(findMediaIndexForColor(sampleGallery, 'color-white')).toBe(2);
    });

    it('falls back to the first GENERAL image when selected color has no tagged image', () => {
      // Red: no image in sampleGallery is tagged with 'color-red'
      // First general image is at index 0
      expect(findMediaIndexForColor(sampleGallery, 'color-red')).toBe(0);
    });

    it('falls back to index 0 as last-resort when selected color has no tagged image and no general image exists', () => {
      const allColorGallery: GalleryMediaItem[] = [
        { url: 'https://example.com/red.jpg', colorId: 'color-red' },
        { url: 'https://example.com/blue.jpg', colorId: 'color-blue' },
      ];
      // Yellow: no yellow image, and no general image exists
      expect(findMediaIndexForColor(allColorGallery, 'color-yellow')).toBe(0);
    });

    it('handles general-only gallery gracefully', () => {
      const generalGallery: GalleryMediaItem[] = [
        { url: 'https://example.com/img1.jpg' },
        { url: 'https://example.com/img2.jpg' },
        { url: 'https://example.com/img3.jpg' },
      ];
      expect(findMediaIndexForColor(generalGallery, 'color-black')).toBe(0);
      expect(findMediaIndexForColor(generalGallery, null)).toBe(0);
      expect(findMediaIndexForColor(generalGallery, undefined)).toBe(0);
    });

    it('falls back to general image when colorId is null or undefined', () => {
      expect(findMediaIndexForColor(sampleGallery, null)).toBe(0);
      expect(findMediaIndexForColor(sampleGallery, undefined)).toBe(0);
      expect(findMediaIndexForColor(sampleGallery, '')).toBe(0);
    });
  });

  describe('deduplicateGalleryImages', () => {
    it('deduplicates images by URL while preserving order', () => {
      const input = [
        { url: 'https://example.com/1.jpg', colorId: 'color-black' },
        { url: 'https://example.com/2.jpg', colorId: 'color-white' },
        { url: 'https://example.com/1.jpg', colorId: 'color-black' },
      ];
      const result = deduplicateGalleryImages(input);
      expect(result).toHaveLength(2);
      expect(result[0].url).toBe('https://example.com/1.jpg');
      expect(result[0].colorId).toBe('color-black');
      expect(result[1].url).toBe('https://example.com/2.jpg');
      expect(result[1].colorId).toBe('color-white');
    });

    it('preserves colorId metadata when first occurrence is untagged but duplicate is tagged', () => {
      const input = [
        { url: 'https://example.com/cover.jpg' }, // untagged first
        { url: 'https://example.com/cover.jpg', colorId: 'color-black' }, // tagged duplicate
        { url: 'https://example.com/other.jpg' },
      ];
      const result = deduplicateGalleryImages(input);
      expect(result).toHaveLength(2);
      expect(result[0].url).toBe('https://example.com/cover.jpg');
      expect(result[0].colorId).toBe('color-black');
      expect(result[1].url).toBe('https://example.com/other.jpg');
    });

    it('preserves existing colorId when first occurrence is already tagged', () => {
      const input = [
        { url: 'https://example.com/img.jpg', colorId: 'color-black' },
        { url: 'https://example.com/img.jpg' }, // untagged duplicate
      ];
      const result = deduplicateGalleryImages(input);
      expect(result).toHaveLength(1);
      expect(result[0].colorId).toBe('color-black');
    });

    it('falls back to singleImage if images array is missing or empty', () => {
      const result = deduplicateGalleryImages([], 'https://example.com/single.jpg');
      expect(result).toEqual([{ url: 'https://example.com/single.jpg' }]);
    });

    it('falls back to default placeholder if both images and singleImage are absent', () => {
      const result = deduplicateGalleryImages([], null);
      expect(result).toHaveLength(1);
      expect(result[0].url).toContain('placehold.co');
    });
  });

  describe('CATALOG VARIANTS.3C-R1: Two Media Modes & getVisibleGalleryImages', () => {
    const fullMixedCatalog: GalleryMediaItem[] = [
      { url: 'https://example.com/general-1.jpg' },
      { url: 'https://example.com/white-1.jpg', colorId: 'color-white' },
      { url: 'https://example.com/white-2.jpg', colorId: 'color-white' },
      { url: 'https://example.com/black-1.jpg', colorId: 'color-black' },
      { url: 'https://example.com/black-2.jpg', colorId: 'color-black' },
      { url: 'https://example.com/black-3.jpg', colorId: 'color-black' },
      { url: 'https://example.com/red-1.jpg', colorId: 'color-red' },
    ];

    it('A. only generic images => GENERAL_GALLERY', () => {
      const genericOnly: GalleryMediaItem[] = [
        { url: 'https://example.com/gen-1.jpg' },
        { url: 'https://example.com/gen-2.jpg' },
      ];
      expect(derivePresentationMediaMode(genericOnly)).toBe('GENERAL_GALLERY');
    });

    it('B. at least one color image => COLORWAY_GALLERIES', () => {
      expect(derivePresentationMediaMode(fullMixedCatalog)).toBe('COLORWAY_GALLERIES');
    });

    it('C & H. mixed legacy data does NOT mix generic + color imagery in COLORWAY_GALLERIES', () => {
      const result = getVisibleGalleryImages(fullMixedCatalog, 'color-white');
      expect(result.some((img) => !img.colorId)).toBe(false);
      expect(result.every((img) => img.colorId === 'color-white')).toBe(true);
    });

    it('D. common gallery unchanged by color click in GENERAL_GALLERY mode', () => {
      const genericCatalog: GalleryMediaItem[] = [
        { url: 'https://example.com/gen-1.jpg' },
        { url: 'https://example.com/gen-2.jpg' },
      ];
      const resNull = getVisibleGalleryImages(genericCatalog, null);
      const resColor = getVisibleGalleryImages(genericCatalog, 'color-red');
      expect(resNull).toEqual(genericCatalog);
      expect(resColor).toEqual(genericCatalog);
    });

    it('F. Black shows Black only', () => {
      const result = getVisibleGalleryImages(fullMixedCatalog, 'color-black');
      expect(result).toHaveLength(3);
      expect(result.every((img) => img.colorId === 'color-black')).toBe(true);
      expect(result.some((img) => img.colorId === 'color-red')).toBe(false);
      expect(result.some((img) => img.colorId === 'color-white')).toBe(false);
      expect(result.some((img) => !img.colorId)).toBe(false);
    });

    it('G. Yellow shows Yellow only', () => {
      const yellowCatalog: GalleryMediaItem[] = [
        ...fullMixedCatalog,
        { url: 'https://example.com/yellow-1.jpg', colorId: 'color-yellow' },
      ];
      const result = getVisibleGalleryImages(yellowCatalog, 'color-yellow');
      expect(result).toHaveLength(1);
      expect(result[0].url).toBe('https://example.com/yellow-1.jpg');
    });

    it('I & J. color with no images in COLORWAY mode => neutral placeholder (never generic, never other color)', () => {
      const result = getVisibleGalleryImages(fullMixedCatalog, 'color-green');
      expect(result).toHaveLength(1);
      expect(result[0].url).toContain('placehold.co');
      expect(result.some((img) => img.colorId === 'color-red')).toBe(false);
      expect(result.some((img) => img.colorId === 'color-black')).toBe(false);
      expect(result.some((img) => img.url === 'https://example.com/general-1.jpg')).toBe(false);
    });

    it('K & L. no color selected in COLORWAY mode does NOT merge colorways; previews deterministic default colorway', () => {
      // White is the first colorway in fullMixedCatalog
      const result = getVisibleGalleryImages(fullMixedCatalog, null);
      expect(result).toHaveLength(2);
      expect(result.every((img) => img.colorId === 'color-white')).toBe(true);
      expect(result.some((img) => img.colorId === 'color-black')).toBe(false);

      // If canonical main image is black-2, default colorway becomes black
      const resultWithMain = getVisibleGalleryImages(
        fullMixedCatalog,
        null,
        undefined,
        'https://example.com/black-2.jpg'
      );
      expect(resultWithMain).toHaveLength(3);
      expect(resultWithMain.every((img) => img.colorId === 'color-black')).toBe(true);
    });

    it('preserves initial sorted order of images within colorway', () => {
      const orderedBlack: GalleryMediaItem[] = [
        { url: 'https://example.com/black-front.jpg', colorId: 'color-black' },
        { url: 'https://example.com/black-back.jpg', colorId: 'color-black' },
        { url: 'https://example.com/black-detail.jpg', colorId: 'color-black' },
      ];
      const result = getVisibleGalleryImages(orderedBlack, 'color-black');
      expect(result.map((img) => img.url)).toEqual([
        'https://example.com/black-front.jpg',
        'https://example.com/black-back.jpg',
        'https://example.com/black-detail.jpg',
      ]);
    });
  });
});
