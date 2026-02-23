export type UserPlan = {
  id: string;
  userId: string;
  plan: string;
  isManual: boolean;
  createdAt: number;
  updatedAt: number;
};

export type User = {
  id: string;
  email: string;
  username: string;
  phone: string;
  name?: string;
  surname?: string;
  isActive: boolean;
  lastLogin: number;
  Plan?: UserPlan;
};

export type UserAddress = {
  id: string;
  country?: string;
  city?: string;
  address?: string;
  zip?: string;
};

export type UserTransaction = {
  id: string;
  userId: string;
  type: string;
  status: string;
  amount: string;
  destination?: string;
  message?: string;
  createdBy?: string;
  createdAt: number;
  updatedAt: number;
};

export type Transaction = UserTransaction & {
  User: {
    username: string;
  };
};

export type UserPlanProgress = {
  current: string | null;
  next: string | null;
  remaining: string;
  netDeposits: string;
};

export type Wallet = {
  id: string;
  name: string;
  address: string;
  network: string;
  currency: string;
  custom: boolean;
  active: boolean;
  created: number;
  updated: number;
};

export type UserWallet = {
  id: string;
  name: string;
  address: string;
  network: string;
  currency: string;
  custom: boolean;
  active: boolean;
  assigned: number;
  by: string;
};

export type KycFile = {
  id: string;
  kind: string;
  filename: string;
  contentType: string;
  size: number;
};

export type KycRequest = {
  id: string;
  userId: string;
  docType: string;
  country: string;
  status: string;
  rejectReason?: string | null;
  createdAt: number;
  updatedAt: number;
  files: KycFile[];
};

export type KycListItem = KycRequest & {
  user: {
    id: string;
    username: string;
    email: string;
    phone: string;
  };
};
