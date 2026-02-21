import { UserTabs } from "./components/UserTabs";

export default async function UserPage({
  params,
}: {
  params: Promise<{ id: number }>;
}) {
  const { id } = await params;

  return (
    <div className="flex flex-col gap-4">
      <UserTabs id={id} />
    </div>
  );
}
