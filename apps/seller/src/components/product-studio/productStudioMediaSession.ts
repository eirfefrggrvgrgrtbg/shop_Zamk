/**
 * Product Studio Media Session
 *
 * Manages the lifecycle of local object URLs created during a Product Studio session.
 * Prevents premature revocation when unmounting Visual workspace (e.g. switching to Form),
 * and ensures proper cleanup when media items are removed or the Studio session unmounts.
 */

export interface StudioMediaRegistry {
  createObjectUrl: (file: File) => string;
  registerObjectUrl: (url: string) => void;
  revokeObjectUrl: (url: string) => void;
  revokeAll: () => void;
  getActiveUrls: () => string[];
  isRegistered: (url: string) => boolean;
}

export function createStudioMediaRegistry(): StudioMediaRegistry {
  const activeUrls = new Set<string>();

  return {
    createObjectUrl: (file: File): string => {
      const url = URL.createObjectURL(file);
      activeUrls.add(url);
      return url;
    },

    registerObjectUrl: (url: string): void => {
      if (url && (url.startsWith('blob:') || url.startsWith('data:'))) {
        activeUrls.add(url);
      }
    },

    revokeObjectUrl: (url: string): void => {
      if (activeUrls.has(url)) {
        activeUrls.delete(url);
        try {
          URL.revokeObjectURL(url);
        } catch {
          // ignore
        }
      }
    },

    revokeAll: (): void => {
      for (const url of activeUrls) {
        try {
          URL.revokeObjectURL(url);
        } catch {
          // ignore
        }
      }
      activeUrls.clear();
    },

    getActiveUrls: (): string[] => Array.from(activeUrls),

    isRegistered: (url: string): boolean => activeUrls.has(url),
  };
}
