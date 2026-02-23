import { UserTabs } from "@/features/users/components/UserTabs";

export default async function UserPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  return (
    <div className="flex flex-col gap-4">
      <UserTabs id={id} />
    </div>
  );
}
