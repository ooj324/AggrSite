import axios from 'axios';
import { getAuthToken, extractErrorMessage } from './utils';

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
  (response) => {
    const res = response.data;
    if (res && typeof res === 'object' && 'success' in res) {
      if (res.success) {
        if ('total' in res || 'limit' in res || 'offset' in res) {
          return {
            items: res.data,
            total: res.total || 0,
            limit: res.limit || 0,
            offset: res.offset || 0,
          };
        }
        return res.data;
      } else {
        return Promise.reject(Object.assign(new Error(res.message || 'API Error'), { data: res }));
      }
    }
    return res;
  },
  (error) => {
    if (error.response?.status === 401) {
      // Clear token and reload if unauthorized
      localStorage.removeItem('aggrsite_token');
      if (window.location.pathname !== '/login') {
        window.location.href = '/login';
      }
    }
    return Promise.reject(extractErrorMessage(error));
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
  external_checkin_method?: string;
  external_checkin_auth_header?: string;
  external_checkin_auth_prefix?: string;
  external_checkin_body?: string;
  custom_headers?: string;
  created_at: string;
  total_balance?: number;
  sort_order?: number;
}

export interface Account {
  id: number;
  site_id: number;
  site_name: string;
  site_platform?: string;
  site_url?: string;
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
  extra_config?: string;
}

export interface FailureReason {
  code: string;
  category: string;
  title: string;
  actionHint: string;
  detailHint: string;
}

export interface CheckinLog {
  id: number;
  account_id: number;
  status: string;
  message: string;
  reward: string;
  created_at: string;
  account_username?: string;
  site_name?: string;
  site_url?: string;
  failureReason?: FailureReason;
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
  timezone?: string;
}

export const detectSite = (url: string) =>
  api.post('/api/sites/detect', { url });

export const pingSite = (url: string) =>
  api.post('/api/sites/ping', { url });
