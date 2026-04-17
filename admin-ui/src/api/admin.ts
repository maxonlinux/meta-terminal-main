import type {
  KycListItem,
  KycRequest,
  Transaction,
  User,
  UserAddress,
  UserPlanProgress,
  UserTransaction,
  UserWallet,
  Wallet,
} from "@/types";
import { API_BASE, getJson, patchJson, postJson } from "./client";

const ADMIN_BASE = `${API_BASE}/api/v1/admin`;

export type PendingCount = {
  users: number;
  wallets: number;
  transactions: number;
  kyc: number;
};

export function getPendingCount() {
  return getJson<PendingCount>(`${ADMIN_BASE}/pending-count`);
}

export function getUsers() {
  return getJson<User[]>(`${ADMIN_BASE}/users`);
}

export function getUser(id: string) {
  return getJson<User>(`${ADMIN_BASE}/users/${id}`);
}

export function getUserAddress(id: string) {
  return getJson<UserAddress>(`${ADMIN_BASE}/users/${id}/address`);
}

export type UserProfilePayload = {
  email: string;
  phone: string;
  name?: string | null;
  surname?: string | null;
};

export function updateUserProfile(id: string, payload: UserProfilePayload) {
  return patchJson(`${ADMIN_BASE}/users/${id}/profile`, payload);
}

export function updateUserAddress(id: string, payload: UserAddress) {
  return patchJson<UserAddress>(`${ADMIN_BASE}/users/${id}/address`, payload);
}

export function getUserTransactions(id: string) {
  return getJson<UserTransaction[]>(`${ADMIN_BASE}/users/${id}/transactions`);
}

export function getUserImpersonation(id: string) {
  return getJson<{ code: string }>(`${ADMIN_BASE}/users/${id}/impersonate`);
}

export function setUserActive(id: string, active: boolean) {
  return patchJson(`${ADMIN_BASE}/users/${id}/active`, { active });
}

export type UserActiveOtp = {
  code?: string | null;
  expiresAt?: number | null;
};

export function getUserActiveOtp(id: string) {
  return getJson<UserActiveOtp>(`${ADMIN_BASE}/users/${id}/otp`);
}

export function getFundingTransactions() {
  return getJson<Transaction[]>(`${ADMIN_BASE}/funding`);
}

export function approveFunding(id: string) {
  return patchJson(`${ADMIN_BASE}/funding/${id}/approve`);
}

export function cancelFunding(id: string) {
  return patchJson(`${ADMIN_BASE}/funding/${id}/cancel`);
}

export function getPlans() {
  return getJson<string[]>(`${ADMIN_BASE}/existing-plans`);
}

export function getUserPlan(id: string) {
  return getJson<UserPlanProgress>(`${ADMIN_BASE}/users/${id}/plan`);
}

export function updateUserPlan(id: string, plan: string) {
  return patchJson(`${ADMIN_BASE}/users/${id}/plan`, { plan });
}

export function resetUserPlan(id: string) {
  return patchJson(`${ADMIN_BASE}/users/${id}/reset-plan`);
}

export function getWallets() {
  return getJson<Wallet[]>(`${ADMIN_BASE}/wallets`);
}

export type WalletPayload = {
  name: string;
  address: string;
  network: string;
  currency: string;
  custom: boolean;
  active: boolean;
};

export function createWallet(payload: WalletPayload) {
  return postJson(`${ADMIN_BASE}/wallets`, payload);
}

export function updateWallet(id: string, payload: WalletPayload) {
  return patchJson(`${ADMIN_BASE}/wallets/${id}`, payload);
}

export function getUserWallets(id: string) {
  return getJson<UserWallet[]>(`${ADMIN_BASE}/users/${id}/wallets`);
}

export function assignUserWallet(id: string, walletId: string) {
  return patchJson(`${ADMIN_BASE}/users/${id}/wallets`, { walletId });
}

export type KycQuery = {
  status?: string;
  q?: string;
  userId?: string;
};

export function getKycRequests(params?: KycQuery) {
  const query = new URLSearchParams();
  if (params?.status) query.set("status", params.status);
  if (params?.q) query.set("q", params.q);
  if (params?.userId) query.set("userId", params.userId);
  const suffix = query.toString();
  return getJson<KycListItem[]>(
    `${ADMIN_BASE}/kyc${suffix ? `?${suffix}` : ""}`,
  );
}

export function getKycRequest(id: string) {
  return getJson<KycRequest>(`${ADMIN_BASE}/kyc/${id}`);
}

export type KycUpdatePayload = {
  status: string;
  rejectReason?: string | null;
};

export function updateKycRequest(id: string, payload: KycUpdatePayload) {
  return patchJson<KycRequest>(`${ADMIN_BASE}/kyc/${id}`, payload);
}

export function getKycFileUrl(kycId: string, fileId: string) {
  return `${API_BASE}/api/v1/admin/kyc/${kycId}/files/${fileId}`;
}
