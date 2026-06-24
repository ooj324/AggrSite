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
