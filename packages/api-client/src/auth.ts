import { request } from './client';
import { setAccessToken, clearAccessToken } from './tokenStore';

// Types specific to auth requests that we didn't export from types or we can just define here
export interface AuthResponse {
  accessToken: string;
  user: any; // UserDTO
}

export const register = async (input: any): Promise<AuthResponse> => {
  const res = await request<AuthResponse>('POST', '/auth/register', { body: input });
  if (res.accessToken) {
    setAccessToken(res.accessToken);
  }
  return res;
};

export const login = async (input: any): Promise<AuthResponse> => {
  const res = await request<AuthResponse>('POST', '/auth/login', { body: input });
  if (res.accessToken) {
    setAccessToken(res.accessToken);
  }
  return res;
};

export const refresh = async (): Promise<AuthResponse> => {
  const res = await request<AuthResponse>('POST', '/auth/refresh');
  if (res.accessToken) {
    setAccessToken(res.accessToken);
  }
  return res;
};

export const logout = async (): Promise<void> => {
  try {
    await request('POST', '/auth/logout');
  } finally {
    clearAccessToken();
  }
};

export const me = async (): Promise<{ user: any }> => {
  return request<{ user: any }>('GET', '/auth/me');
};

export const changePassword = async (input: any): Promise<void> => {
  await request('POST', '/auth/change-password', { body: input });
  clearAccessToken(); // usually implies needing to re-login
};
