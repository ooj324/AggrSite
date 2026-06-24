import axios from 'axios';
import { getAuthToken } from './utils';

// When running in development, Vite proxy can be used, or we just point to localhost:4000
// When embedded, API is on the same host under /api
const baseURL = import.meta.env.DEV ? 'http://localhost:4000' : '';

export const api = axios.create({
  baseURL,
});

api.interceptors.request.use((config) => {
  const token = getAuthToken();
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

api.interceptors.response.use(
  (response) => response.data,
  (error) => {
    if (error.response?.status === 401) {
      // Clear token and reload if unauthorized
      localStorage.removeItem('aggrsite_token');
      if (window.location.pathname !== '/login') {
        window.location.href = '/login';
      }
    }
    return Promise.reject(error.response?.data?.message || error.message);
  }
);

// Types
export interface Site {
  id: number;
  name: string;
  url: string;
  platform: string;
  status: string;
  proxy_url?: string;
  use_system_proxy?: boolean;
  external_checkin_url?: string;
  custom_headers?: string;
  created_at: string;
}

export interface Account {
  id: number;
  site_id: number;
  site_name: string;
  username: string;
  access_token: string;
  api_token?: string;
  balance: number;
  balance_used: number;
  quota: number;
  status: string;
  checkin_enabled: boolean;
  last_checkin_at: string;
  last_balance_refresh: string;
}

export interface CheckinLog {
  id: number;
  account_id: number;
  status: string;
  message: string;
  reward: string;
  created_at: string;
}

export interface Event {
  id: number;
  type: string;
  title: string;
  message: string;
  level: string;
  created_at: string;
}

export interface SchedulerStatus {
  running: boolean;
  checkin_cron: string;
  next_checkin: string;
  balance_refresh_cron: string;
  next_balance_refresh: string;
}
