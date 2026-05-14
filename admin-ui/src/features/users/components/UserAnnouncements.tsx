"use client";

import { Plus, RotateCw } from "lucide-react";
import { useState } from "react";
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
  Sheet,
  SheetBody,
  SheetClose,
  SheetContent,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import {
  AnnouncementForm,
  type AnnouncementFormState,
} from "@/features/announcements/components/AnnouncementForm";

const initialForm: AnnouncementFormState = {
  scope: "USER",
  userId: "",
  title: "",
  body: "",
  link: "",
  priority: 0,
  isActive: true,
};

export function UserAnnouncements({ id }: { id: string }) {
  const [form, setForm] = useState<AnnouncementFormState>(initialForm);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const { data, error: loadError, isLoading, mutate } = useSWR(
    ["admin:user-announcements", id],
    () => getAnnouncements("USER", id),
  );

  const resetForm = () => {
    setEditingId(null);
    setForm(initialForm);
  };

  const submit = async () => {
    setError(null);
    if (!form.title.trim() || !form.body.trim()) {
      setError("Title and body are required");
      return;
    }

    try {
      setIsSubmitting(true);
      const payload = {
        scope: "USER" as const,
        userId: id,
        title: form.title.trim(),
        body: form.body.trim(),
        link: form.link.trim() || undefined,
        priority: form.priority,
        isActive: form.isActive,
      };

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
      setError(err instanceof Error ? err.message : "Failed to save announcement");
    } finally {
      setIsSubmitting(false);
    }
  };

  const edit = (announcementId: string) => {
    const current = data?.find((x) => x.id === announcementId);
    if (!current) return;
    setEditingId(announcementId);
    setForm({
      scope: "USER",
      userId: id,
      title: current.title,
      body: current.body,
      link: current.link ?? "",
      priority: current.priority,
      isActive: current.isActive,
    });
  };

  const toggle = async (announcementId: string) => {
    const current = data?.find((x) => x.id === announcementId);
    if (!current) return;
    setError(null);
    try {
      setBusyId(announcementId);
      await updateAnnouncement(announcementId, {
        scope: "USER",
        userId: id,
        title: current.title,
        body: current.body,
        link: current.link,
        priority: current.priority,
        isActive: !current.isActive,
        startsAt: current.startsAt,
        endsAt: current.endsAt,
      });
      toast.success(current.isActive ? "Announcement disabled" : "Announcement enabled");
      await mutate();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to update announcement");
    } finally {
      setBusyId(null);
    }
  };

  const remove = async (announcementId: string) => {
    setError(null);
    try {
      setBusyId(announcementId);
      const { res, body } = await deleteAnnouncement(announcementId);
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
      setError(err instanceof Error ? err.message : "Failed to delete announcement");
    } finally {
      setBusyId(null);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>User Announcements</CardTitle>
        <CardDescription>Manage this user's personal announcements.</CardDescription>
        <CardAction>
          <ButtonGroup>
            <Sheet>
              <Button intent="primary" onPress={resetForm}>
                <Plus data-slot="icon" />
                New announcement
              </Button>
              <SheetContent>
                <SheetHeader>
                  <SheetTitle>Create user announcement</SheetTitle>
                </SheetHeader>
                <SheetBody>
                  <AnnouncementForm
                    value={form}
                    onChange={setForm}
                    includeScope={false}
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
        {error ? <p className="text-sm text-danger">{error}</p> : null}
        {loadError ? <p className="text-sm text-danger">{loadError.message}</p> : null}

        {isLoading ? (
          <div className="flex items-center justify-center py-6">
            <Loader variant="spin" />
          </div>
        ) : null}

        {!isLoading && (!data || data.length === 0) ? (
          <div className="rounded-md border border-dashed px-4 py-6 text-center text-sm text-muted-fg">
            No user announcements.
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
                  <div className="mt-2 border-t border-border/70 pt-2" />
                  <p className="text-sm leading-snug text-muted-fg break-words">{item.body}</p>
                  <div className="grid grid-cols-1 gap-x-4 gap-y-1 pt-2 text-xs text-muted-fg sm:grid-cols-2">
                    <span>
                      Status:{" "}
                      <span className={item.isActive ? "text-success" : "text-danger"}>
                        {item.isActive ? "Active" : "Inactive"}
                      </span>
                    </span>
                    <span>Priority: {item.priority}</span>
                  </div>
                </div>

                <div className="w-full md:w-auto">
                  <ButtonGroup className="w-full md:w-auto">
                    <Sheet>
                      <Button
                        intent="outline"
                        onPress={() => edit(item.id)}
                        disabled={busyId === item.id}
                      >
                        Edit
                      </Button>
                      <SheetContent>
                        <SheetHeader>
                          <SheetTitle>Edit user announcement</SheetTitle>
                        </SheetHeader>
                        <SheetBody>
                          <AnnouncementForm
                            value={form}
                            onChange={setForm}
                            includeScope={false}
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
                    <Button
                      intent="outline"
                      onClick={() => void toggle(item.id)}
                      disabled={busyId === item.id}
                    >
                      {item.isActive ? "Disable" : "Enable"}
                    </Button>
                    <Button
                      intent="outline"
                      onClick={() => void remove(item.id)}
                      disabled={busyId === item.id}
                    >
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
  );
}
