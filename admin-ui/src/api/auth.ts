import { API_BASE, getJson, postJson } from "./client";

const ADMIN_AUTH_BASE = `${API_BASE}/api/v1/admin/auth`;

export type AdminAuthStatus = {
  initialized: boolean;
};

export function getAdminStatus() {
  return getJson<AdminAuthStatus>(`${ADMIN_AUTH_BASE}/status`);
}

export function adminLogout() {
  return postJson<{ success: boolean }>(`${ADMIN_AUTH_BASE}/logout`);
}
