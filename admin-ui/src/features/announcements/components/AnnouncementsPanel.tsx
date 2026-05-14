"use client";

import { Plus, RotateCw } from "lucide-react";
import { useMemo, useState } from "react";
import useSWR from "swr";
import { toast } from "sonner";
import {
  createAnnouncement,
  deleteAnnouncement,
  getAnnouncements,
  updateAnnouncement,
} from "@/api/admin";
import { Button } from "@/components/ui/button";
import { ButtonGroup } from "@/components/ui/button-group";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Loader } from "@/components/ui/loader";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
} from "@/components/ui/select";
import {
  Sheet,
  SheetBody,
  SheetClose,
  SheetContent,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import type { Announcement } from "@/types";
import {
  AnnouncementForm,
  type AnnouncementFormState,
  type AnnouncementScope,
} from "./AnnouncementForm";

const initialForm: AnnouncementFormState = {
  scope: "GLOBAL",
  userId: "",
  title: "",
  body: "",
  link: "",
  priority: 0,
  isActive: true,
};

const filterOptions = [
  { id: "ALL", title: "All scopes" },
  { id: "GLOBAL", title: "GLOBAL" },
  { id: "USER", title: "USER" },
];

function toPayload(form: AnnouncementFormState) {
  return {
    scope: form.scope,
    userId: form.scope === "USER" ? form.userId || undefined : undefined,
    title: form.title.trim(),
    body: form.body.trim(),
    link: form.link.trim() || undefined,
    priority: Number(form.priority) || 0,
    isActive: form.isActive,
  };
}

export function AnnouncementsPanel() {
  const [scopeFilter, setScopeFilter] = useState<"ALL" | AnnouncementScope>("ALL");
  const [form, setForm] = useState<AnnouncementFormState>(initialForm);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const swrKey = useMemo(
    () => ["admin:announcements", scopeFilter] as const,
    [scopeFilter],
  );

  const { data, error, isLoading, mutate } = useSWR(
    swrKey,
    async ([, scope]) => getAnnouncements(scope === "ALL" ? undefined : scope),
  );

  const resetForm = () => {
    setEditingId(null);
    setForm(initialForm);
  };

  const submit = async () => {
    if (!form.title.trim() || !form.body.trim()) {
      toast.error("Title and body are required");
      return;
    }
    if (form.scope === "USER" && !form.userId.trim()) {
      toast.error("User ID is required for USER scope");
      return;
    }

    try {
      setIsSubmitting(true);
      const payload = toPayload(form);
      if (editingId) {
        await updateAnnouncement(editingId, payload);
        toast.success("Announcement updated");
      } else {
        await createAnnouncement(payload);
        toast.success("Announcement created");
      }
      resetForm();
      await mutate();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to save announcement");
    } finally {
      setIsSubmitting(false);
    }
  };

  const editItem = (item: Announcement) => {
    setEditingId(item.id);
    setForm({
      scope: item.scope,
      userId: item.userId ?? "",
      title: item.title,
      body: item.body,
      link: item.link ?? "",
      priority: item.priority,
      isActive: item.isActive,
    });
  };

  const toggleItem = async (item: Announcement) => {
    try {
      await updateAnnouncement(item.id, {
        scope: item.scope,
        userId: item.userId,
        title: item.title,
        body: item.body,
        link: item.link,
        priority: item.priority,
        isActive: !item.isActive,
        startsAt: item.startsAt,
        endsAt: item.endsAt,
      });
      toast.success(item.isActive ? "Announcement disabled" : "Announcement enabled");
      await mutate();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to update announcement");
    }
  };

  const removeItem = async (item: Announcement) => {
    try {
      const { res, body } = await deleteAnnouncement(item.id);
      if (!res.ok) {
        const message =
          body && typeof (body as { error?: string }).error === "string"
            ? (body as { error: string }).error
            : "Failed to delete announcement";
        throw new Error(message);
      }
      toast.success("Announcement deleted");
      await mutate();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to delete announcement");
    }
  };

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Announcements</CardTitle>
          <CardDescription>Manage global and targeted announcements.</CardDescription>
          <CardAction>
            <ButtonGroup>
              <Sheet>
                <Button intent="primary" onPress={resetForm}>
                  <Plus data-slot="icon" />
                  New announcement
                </Button>
                <SheetContent>
                  <SheetHeader>
                    <SheetTitle>Create announcement</SheetTitle>
                  </SheetHeader>
                  <SheetBody>
                    <AnnouncementForm
                      value={form}
                      onChange={setForm}
                      includeScope
                      isSubmitting={isSubmitting}
                      submitLabel="Create"
                      onSubmit={submit}
                    />
                  </SheetBody>
                  <SheetFooter>
                    <SheetClose>Close</SheetClose>
                  </SheetFooter>
                </SheetContent>
              </Sheet>
              <Button intent="outline" onClick={() => void mutate()}>
                <RotateCw data-slot="icon" />
                Refresh
              </Button>
            </ButtonGroup>
          </CardAction>
        </CardHeader>

        <CardContent className="space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            <Select
              value={scopeFilter}
              onChange={(value) =>
                setScopeFilter((value as "ALL" | AnnouncementScope | null) ?? "ALL")
              }
              placeholder="Filter scope"
            >
              <SelectTrigger className="w-full md:w-56" />
              <SelectContent items={filterOptions}>
                {(item) => (
                  <SelectItem id={item.id} textValue={item.title}>
                    {item.title}
                  </SelectItem>
                )}
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-fg">
              {data?.length ?? 0} item{(data?.length ?? 0) === 1 ? "" : "s"}
            </p>
          </div>

          {error ? (
            <div className="rounded-md border border-danger/40 bg-danger/10 px-3 py-2 text-sm text-danger">
              {error.message}
            </div>
          ) : null}

          {isLoading ? (
            <div className="flex items-center justify-center py-6">
              <Loader variant="spin" />
            </div>
          ) : null}

          {!isLoading && data && data.length === 0 ? (
            <div className="rounded-md border border-dashed px-4 py-6 text-center text-sm text-muted-fg">
              No announcements yet.
            </div>
          ) : null}

          <div className="space-y-2">
            {data?.map((item) => (
              <div
                key={item.id}
                className="rounded-lg border bg-muted/15 p-4"
              >
                <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                  <div className="min-w-0 space-y-1">
                    <p className="text-sm font-semibold leading-snug break-words">{item.title}</p>
                    <p className="pt-1 text-sm leading-snug text-muted-fg break-words">{item.body}</p>
                    <div className="grid grid-cols-1 gap-x-4 gap-y-1 pt-2 text-xs text-muted-fg sm:grid-cols-2">
                      <span className="truncate">Scope: {item.scope}</span>
                      <span>Status: {item.isActive ? "Active" : "Inactive"}</span>
                      <span>Priority: {item.priority}</span>
                      {item.userId ? <span className="truncate">User: {item.userId}</span> : null}
                    </div>
                  </div>

                  <div className="w-full md:w-auto">
                    <ButtonGroup className="w-full md:w-auto">
                      <Sheet>
                        <Button intent="outline" onPress={() => editItem(item)}>
                          Edit
                        </Button>
                        <SheetContent>
                          <SheetHeader>
                            <SheetTitle>Edit announcement</SheetTitle>
                          </SheetHeader>
                          <SheetBody>
                            <AnnouncementForm
                              value={form}
                              onChange={setForm}
                              includeScope
                              isSubmitting={isSubmitting}
                              submitLabel="Save"
                              onSubmit={submit}
                            />
                          </SheetBody>
                          <SheetFooter>
                            <SheetClose>Close</SheetClose>
                          </SheetFooter>
                        </SheetContent>
                      </Sheet>
                      <Button intent="outline" onClick={() => void toggleItem(item)}>
                        {item.isActive ? "Disable" : "Enable"}
                      </Button>
                      <Button intent="outline" onClick={() => void removeItem(item)}>
                        Delete
                      </Button>
                    </ButtonGroup>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
