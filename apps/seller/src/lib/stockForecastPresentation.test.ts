import { describe, it, expect } from 'vitest';
import {
  formatDaysRussian,
  formatApproximateDaysOfCover,
  formatVariantDisplayLabel,
} from './stockForecastPresentation';

describe('stockForecastPresentation', () => {
  describe('formatDaysRussian', () => {
    it('handles 1 day forms', () => {
      expect(formatDaysRussian(1)).toBe('1 день');
      expect(formatDaysRussian(21)).toBe('21 день');
      expect(formatDaysRussian(101)).toBe('101 день');
    });

    it('handles 2-4 day forms', () => {
      expect(formatDaysRussian(2)).toBe('2 дня');
      expect(formatDaysRussian(3)).toBe('3 дня');
      expect(formatDaysRussian(4)).toBe('4 дня');
      expect(formatDaysRussian(22)).toBe('22 дня');
      expect(formatDaysRussian(24)).toBe('24 дня');
    });

    it('handles 5-20 and 0 day forms', () => {
      expect(formatDaysRussian(0)).toBe('0 дней');
      expect(formatDaysRussian(5)).toBe('5 дней');
      expect(formatDaysRussian(9)).toBe('9 дней');
      expect(formatDaysRussian(11)).toBe('11 дней');
      expect(formatDaysRussian(12)).toBe('12 дней');
      expect(formatDaysRussian(14)).toBe('14 дней');
      expect(formatDaysRussian(19)).toBe('19 дней');
      expect(formatDaysRussian(20)).toBe('20 дней');
    });

    it('treats negative numbers as 0', () => {
      expect(formatDaysRussian(-5)).toBe('0 дней');
    });
  });

  describe('formatApproximateDaysOfCover', () => {
    it('rounds and formats days of cover accurately', () => {
      expect(formatApproximateDaysOfCover(21.2)).toBe('≈ 21 день запаса');
      expect(formatApproximateDaysOfCover(9.1)).toBe('≈ 9 дней запаса');
      expect(formatApproximateDaysOfCover(4.2)).toBe('≈ 4 дня запаса');
      expect(formatApproximateDaysOfCover(1.4)).toBe('≈ 1 день запаса');
      expect(formatApproximateDaysOfCover(0.6)).toBe('≈ 1 день запаса');
      expect(formatApproximateDaysOfCover(0.2)).toBe('≈ 0 дней запаса');
    });
  });

  describe('formatVariantDisplayLabel', () => {
    it('combines color and size with a middle dot', () => {
      expect(formatVariantDisplayLabel('Красный', 'L')).toBe('Красный · L');
    });

    it('falls back to single available attribute', () => {
      expect(formatVariantDisplayLabel('Красный', '')).toBe('Красный');
      expect(formatVariantDisplayLabel('', 'L')).toBe('L');
      expect(formatVariantDisplayLabel(null, 'XL')).toBe('XL');
      expect(formatVariantDisplayLabel('Синий', null)).toBe('Синий');
    });

    it('returns empty string when both empty', () => {
      expect(formatVariantDisplayLabel('', '')).toBe('');
      expect(formatVariantDisplayLabel(null, null)).toBe('');
    });
  });
});
