"use client";

import { useMemo, useState } from "react";
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
import type { Announcement } from "@/types";

type Scope = "GLOBAL" | "USER";

type FormState = {
  scope: Scope;
  userId: string;
  title: string;
  body: string;
  link: string;
  priority: number;
  isActive: boolean;
};

const initialForm: FormState = {
  scope: "GLOBAL",
  userId: "",
  title: "",
  body: "",
  link: "",
  priority: 0,
  isActive: true,
};

function toPayload(form: FormState) {
  return {
    scope: form.scope,
    userId: form.scope === "USER" ? form.userId || undefined : undefined,
    title: form.title,
    body: form.body,
    link: form.link || undefined,
    priority: Number(form.priority) || 0,
    isActive: form.isActive,
  };
}

export function AnnouncementsPanel() {
  const [scopeFilter, setScopeFilter] = useState<"" | Scope>("");
  const [form, setForm] = useState<FormState>(initialForm);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const swrKey = useMemo(
    () => ["admin:announcements", scopeFilter] as const,
    [scopeFilter],
  );

  const { data, error, isLoading, mutate } = useSWR(
    swrKey,
    async ([, scope]) => {
      return getAnnouncements(scope || undefined);
    },
  );

  const submit = async () => {
    if (!form.title.trim() || !form.body.trim()) {
      toast.error("Title and body are required");
      return;
    }
    if (form.scope === "USER" && !form.userId.trim()) {
      toast.error("User ID is required for USER scope");
      return;
    }

    const payload = toPayload(form);
    try {
      setIsSubmitting(true);
      if (editingId) {
        await updateAnnouncement(editingId, payload);
        toast.success("Announcement updated");
      } else {
        await createAnnouncement(payload);
        toast.success("Announcement created");
      }
      setEditingId(null);
      setForm(initialForm);
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

  const disableItem = async (item: Announcement) => {
    try {
      await updateAnnouncement(item.id, {
        scope: item.scope,
        userId: item.userId,
        title: item.title,
        body: item.body,
        link: item.link,
        priority: item.priority,
        isActive: false,
        startsAt: item.startsAt,
        endsAt: item.endsAt,
      });
      toast.success("Announcement disabled");
      await mutate();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to disable announcement");
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
          <CardDescription>
            Manage global announcements and create targeted USER entries.
          </CardDescription>
          <CardAction>
            <ButtonGroup>
              <Button intent="outline" onClick={() => void mutate()}>
                Refresh
              </Button>
            </ButtonGroup>
          </CardAction>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <select
              className="h-9 rounded-md border bg-transparent px-3 text-sm"
              value={form.scope}
              onChange={(e) =>
                setForm((prev) => ({ ...prev, scope: e.target.value as Scope }))
              }
            >
              <option value="GLOBAL">GLOBAL</option>
              <option value="USER">USER</option>
            </select>
            {form.scope === "USER" && (
              <Input
                placeholder="User ID"
                value={form.userId}
                onChange={(e) =>
                  setForm((prev) => ({ ...prev, userId: e.target.value }))
                }
              />
            )}
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
              className="md:col-span-2 min-h-24 rounded-md border bg-transparent px-3 py-2 text-sm"
              placeholder="Body"
              value={form.body}
              onChange={(e) => setForm((prev) => ({ ...prev, body: e.target.value }))}
            />
          </div>
          <ButtonGroup>
            <Button onClick={submit} disabled={isSubmitting}>
              {editingId ? "Update" : "Create"}
            </Button>
            {editingId && (
              <Button
                intent="outline"
                onClick={() => {
                  setEditingId(null);
                  setForm(initialForm);
                }}
              >
                Cancel
              </Button>
            )}
          </ButtonGroup>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Existing announcements</CardTitle>
          <CardDescription>
            Verify what is currently visible and active in production.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex items-center gap-2">
            <select
              className="h-9 rounded-md border bg-transparent px-3 text-sm"
              value={scopeFilter}
              onChange={(e) => setScopeFilter(e.target.value as "" | Scope)}
            >
              <option value="">ALL</option>
              <option value="GLOBAL">GLOBAL</option>
              <option value="USER">USER</option>
            </select>
          </div>
          {error ? (
            <p className="text-sm text-danger">{error.message}</p>
          ) : null}
          {isLoading && <p>Loading...</p>}
          {!isLoading && data && data.length === 0 && <p>No announcements.</p>}
          <div className="space-y-2">
            {data?.map((item) => (
              <div key={item.id} className="rounded-md border p-3">
                <div className="flex items-center justify-between gap-2">
                  <div>
                    <p className="text-sm font-semibold">{item.title}</p>
                    <p className="text-xs opacity-70">
                      {item.scope}
                      {item.userId ? ` • user ${item.userId}` : ""} • priority {item.priority} • {" "}
                      {item.isActive ? "active" : "inactive"}
                    </p>
                  </div>
                  <ButtonGroup>
                    <Button intent="outline" onClick={() => editItem(item)}>
                      Edit
                    </Button>
                    {item.isActive ? (
                      <Button intent="outline" onClick={() => void disableItem(item)}>
                        Disable
                      </Button>
                    ) : null}
                    <Button intent="outline" onClick={() => void removeItem(item)}>
                      Delete
                    </Button>
                  </ButtonGroup>
                </div>
                <p className="text-sm mt-2">{item.body}</p>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
