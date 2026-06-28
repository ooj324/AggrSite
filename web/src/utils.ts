import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

// Simple local storage hook for auth token
export function getAuthToken() {
  return localStorage.getItem('aggrsite_token') || '';
}

export function setAuthToken(token: string) {
  localStorage.setItem('aggrsite_token', token);
}

export function extractErrorMessage(err: any): string {
  if (typeof err === 'string') return err;
  if (!err) return 'Unknown error';

  let msg = '';
  // Axios error response body
  const data = err.response?.data;
  if (data) {
    if (typeof data === 'string') {
      // If backend throws an HTML error (e.g. 502)
      if (data.toLowerCase().includes('<html') || data.toLowerCase().includes('<body')) {
        msg = `HTTP Error ${err.response?.status || 'Unknown'}`;
      } else {
        msg = data;
      }
    } else if (typeof data === 'object') {
      if (data.message && typeof data.message === 'string') msg = data.message;
      else if (data.error && typeof data.error === 'string') msg = data.error;
      else if (data.msg && typeof data.msg === 'string') msg = data.msg;
    }
  }

  // Fallback to error message from Error object
  if (!msg && err.message) {
    msg = err.message;
  }

  if (!msg) {
    try {
      msg = JSON.stringify(err);
    } catch {
      msg = 'Unknown error object';
    }
  }

  if (msg === '[object Object]') {
    msg = 'Unknown error object';
  }

  return msg;
}
