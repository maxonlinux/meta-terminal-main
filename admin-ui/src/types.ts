export type UserPlan = {
  id: number;
  userId: number;
  plan: string;
  isManual: boolean;
  createdAt: Date;
  updatedAt: Date;
};

export type User = {
  id: number;
  email: string;
  username: string;
  phone: string;
  name?: string;
  surname?: string;
  password: string;
  isActive: boolean;
  Plan?: UserPlan;
};

export type UserAddress = {
  id: number;
  country?: string;
  city?: string;
  address?: string;
  zip?: string;
};

export type UserTransaction = {
  id: number;
  userId: number;
  balanceId: number;
  type: string;
  status: string;
  amount: number;
  fee: number;
  destination?: string;
  reference?: string;
  message?: string;
  metadata?: JSON;
  createdBy?: string;
  createdAt: Date;
  updatedAt: Date;
};

export type Transaction = UserTransaction & {
  User: {
    username: string;
  };
};
