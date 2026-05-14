"use client";

import { useState } from "react";
import useSWR from "swr";
import { toast } from "sonner";
import {
  createAnnouncement,
  deleteAnnouncement,
  getAnnouncements,
  updateAnnouncement,
} from "@/api/admin";
import { ButtonGroup } from "@/components/ui/button-group";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Loader } from "@/components/ui/loader";
import { Separator } from "@/components/ui/separator";

type FormState = {
  title: string;
  body: string;
  link: string;
  priority: number;
  isActive: boolean;
};

const initialForm: FormState = {
  title: "",
  body: "",
  link: "",
  priority: 0,
  isActive: true,
};

export function UserAnnouncements({ id }: { id: string }) {
  const [form, setForm] = useState<FormState>(initialForm);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const { data, error: loadError, isLoading, mutate } = useSWR(
    ["admin:user-announcements", id],
    () => getAnnouncements("USER", id),
  );

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

      setForm(initialForm);
      setEditingId(null);
      await mutate();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save announcement");
    } finally {
      setIsSubmitting(false);
    }
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

  const edit = (announcementId: string) => {
    const current = data?.find((x) => x.id === announcementId);
    if (!current) {
      return;
    }
    setEditingId(announcementId);
    setForm({
      title: current.title,
      body: current.body,
      link: current.link ?? "",
      priority: current.priority,
      isActive: current.isActive,
    });
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
        <CardDescription>
          Personal announcements for this user only. Edit, enable, disable, or delete inline.
        </CardDescription>
        <CardAction>
          <Button intent="outline" onClick={() => void mutate()}>
            Refresh
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-2 rounded-md border border-border/70 bg-muted/25 p-3 md:grid-cols-3">
          <div>
            <p className="text-[11px] uppercase tracking-wide text-muted-fg">Target user</p>
            <p className="text-sm font-medium">{id}</p>
          </div>
          <div>
            <p className="text-[11px] uppercase tracking-wide text-muted-fg">Status</p>
            <p className="text-sm font-medium">{form.isActive ? "Active" : "Inactive"}</p>
          </div>
          <div>
            <p className="text-[11px] uppercase tracking-wide text-muted-fg">Priority</p>
            <p className="text-sm font-medium">{form.priority}</p>
          </div>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          <Input
            placeholder="Title"
            value={form.title}
            onChange={(e) => setForm((prev) => ({ ...prev, title: e.target.value }))}
          />
          <Input
            placeholder="Link (optional)"
            value={form.link}
            onChange={(e) => setForm((prev) => ({ ...prev, link: e.target.value }))}
          />
          <Input
            placeholder="Priority"
            type="number"
            value={String(form.priority)}
            onChange={(e) =>
              setForm((prev) => ({ ...prev, priority: Number(e.target.value) || 0 }))
            }
          />
          <Checkbox
            isSelected={form.isActive}
            onChange={(isSelected) =>
              setForm((prev) => ({ ...prev, isActive: Boolean(isSelected) }))
            }
          >
            Active
          </Checkbox>
          <textarea
            className="md:col-span-2 min-h-28 rounded-md border bg-transparent px-3 py-2 text-sm"
            placeholder="Body"
            value={form.body}
            onChange={(e) => setForm((prev) => ({ ...prev, body: e.target.value }))}
          />
        </div>

        <Separator />

        <ButtonGroup>
          <Button onClick={() => void submit()} disabled={isSubmitting}>
            {isSubmitting ? <Loader variant="spin" /> : null}
            {editingId ? "Update announcement" : "Create announcement"}
          </Button>
          {editingId ? (
            <Button
              intent="outline"
              onClick={() => {
                setEditingId(null);
                setForm(initialForm);
              }}
            >
              Cancel
            </Button>
          ) : null}
        </ButtonGroup>
        {error ? <p className="text-sm text-danger">{error}</p> : null}
        {loadError ? <p className="text-sm text-danger">{loadError.message}</p> : null}

        {isLoading && <p>Loading...</p>}
        {!isLoading && (!data || data.length === 0) && <p>No user announcements.</p>}
        <div className="space-y-2">
          {data?.map((item) => (
            <div
              key={item.id}
              className="rounded-lg border p-3 transition-colors hover:border-primary/40"
            >
              <div className="flex items-start justify-between gap-2">
                <div>
                  <div className="mb-1 flex flex-wrap items-center gap-1.5">
                    <span className="rounded-full bg-muted px-2 py-0.5 text-[11px] font-medium">
                      priority {item.priority}
                    </span>
                    <span
                      className={`rounded-full px-2 py-0.5 text-[11px] font-medium ${
                        item.isActive ? "bg-success/15 text-success" : "bg-danger/15 text-danger"
                      }`}
                    >
                      {item.isActive ? "active" : "inactive"}
                    </span>
                  </div>
                  <p className="text-sm font-semibold leading-snug">{item.title}</p>
                </div>
                <ButtonGroup>
                  <Button
                    intent="outline"
                    onClick={() => edit(item.id)}
                    disabled={busyId === item.id}
                  >
                    Edit
                  </Button>
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
              <p className="mt-2 text-sm leading-snug text-muted-fg">{item.body}</p>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
