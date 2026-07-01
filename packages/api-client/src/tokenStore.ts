// Access token is stored in memory to prevent XSS attacks extracting it from localStorage.
// Refresh token is handled entirely via HTTPOnly cookies by the browser.

let accessToken: string | null = null;

export const getAccessToken = (): string | null => {
  return accessToken;
};

export const setAccessToken = (token: string): void => {
  accessToken = token;
};

export const clearAccessToken = (): void => {
  accessToken = null;
};
