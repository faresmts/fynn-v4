export type User = {
  id: number;
  household_id: number;
  name: string;
  email: string;
  csrf: string;
};
export type Account = {
  id: number;
  name: string;
  kind: string;
  opening_cents: number;
  opening_date: string;
  balance_cents: number;
};
export type Category = { id: number; name: string; type: string };
export type Transaction = {
  id: number;
  type: string;
  description: string;
  amount_cents: number;
  expected_cents: number;
  account_id: number;
  destination_id: number | null;
  category_id: number | null;
  date: string;
  competence: string;
  due_date: string;
  status: string;
  paid_date: string | null;
  visibility: string;
  recurring_rule_id: number | null;
  version: number;
  category: string;
  account: string;
  owner_id: number;
};
export type Totals = {
  income_cents: number;
  expense_cents: number;
  pending_cents: number;
  result_cents: number;
  cash_cents: number;
};
export type Dashboard = {
  month: string;
  context: string;
  totals: Totals;
  previous: Totals;
  balance_cents: number | null;
  available_cents: number | null;
  accounts: Account[];
  history: ({ month: string; count: number } & Totals)[];
  categories: { name: string; amount_cents: number }[];
  contributors: {
    user_id: number;
    name: string;
    paid_cents: number;
    responsibility_cents: number;
    balance_cents: number;
  }[];
  household_expenses: {
    id: number;
    description: string;
    category: string;
    amount_cents: number;
    status: string;
    due_date: string;
  }[];
  recent: Transaction[];
  upcoming: Transaction[];
  today: string;
};
export type Rule = {
  id: number;
  description: string;
  amount_cents: number;
  day: number;
  account_id: number;
  category_id: number;
  starts_on: string;
};
export type Settlement = {
  id: number;
  sender_id: number;
  sender: string;
  receiver_id: number;
  receiver: string;
  amount_cents: number;
  month: string;
  settled_on: string;
  note: string;
};
export type Goal = {
  id: number;
  owner_id: number;
  owner: string;
  name: string;
  target_cents: number;
  saved_cents: number;
  deadline: string | null;
  visibility: string;
  progress_bps: number;
};
export type IncomeSnapshot = {
  id: number;
  user_id: number;
  user_name: string;
  month: string;
  income_cents: number;
  rule_mode: 'monthly_income_ratio' | 'equal' | 'custom';
  confirmed: boolean;
  version: number;
};
export type SharingSummary = {
  month: string;
  status: 'ready' | 'needs_rule';
  total_income_cents: number;
  people: {
    id: number;
    name: string;
    income_cents: number;
    confirmed: boolean;
    ratio_bps: number;
  }[];
  message?: string;
};
