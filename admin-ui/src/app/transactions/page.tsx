import { TransactionsTable } from "./components/TransactionsTable";

export default async function TransactionsPage() {
  return (
    <div className="flex flex-col gap-4">
      <TransactionsTable />
    </div>
  );
}
