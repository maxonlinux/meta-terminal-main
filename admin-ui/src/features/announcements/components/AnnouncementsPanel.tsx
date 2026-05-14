"use client";

import { useMemo, useState } from "react";
import useSWR from "swr";
import {
  createAnnouncement,
  getAnnouncements,
  updateAnnouncement,
} from "@/api/admin";
import { Button } from "@/components/ui/button";
import { CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
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

  const swrKey = useMemo(
    () => ["admin:announcements", scopeFilter] as const,
    [scopeFilter],
  );

  const { data, isLoading, mutate } = useSWR(swrKey, async ([, scope]) => {
    return getAnnouncements(scope || undefined);
  });

  const submit = async () => {
    const payload = toPayload(form);
    if (editingId) {
      await updateAnnouncement(editingId, payload);
    } else {
      await createAnnouncement(payload);
    }
    setEditingId(null);
    setForm(initialForm);
    await mutate();
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
    await mutate();
  };

  return (
    <div className="space-y-4">
      <div className="rounded-lg border p-4">
        <CardHeader>
          <CardTitle>Announcements</CardTitle>
          <CardDescription>
            Manage personal and global announcements.
          </CardDescription>
        </CardHeader>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3 mt-4">
          <select
            className="h-9 rounded-md border bg-transparent px-3"
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
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={form.isActive}
              onChange={(e) =>
                setForm((prev) => ({ ...prev, isActive: e.target.checked }))
              }
            />
            Active
          </label>
          <textarea
            className="md:col-span-2 min-h-24 rounded-md border bg-transparent px-3 py-2"
            placeholder="Body"
            value={form.body}
            onChange={(e) => setForm((prev) => ({ ...prev, body: e.target.value }))}
          />
        </div>
        <div className="mt-3 flex gap-2">
          <Button onClick={submit}>{editingId ? "Update" : "Create"}</Button>
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
        </div>
      </div>

      <div className="rounded-lg border p-4">
        <div className="mb-3 flex items-center gap-2">
          <select
            className="h-9 rounded-md border bg-transparent px-3"
            value={scopeFilter}
            onChange={(e) => setScopeFilter(e.target.value as "" | Scope)}
          >
            <option value="">ALL</option>
            <option value="GLOBAL">GLOBAL</option>
            <option value="USER">USER</option>
          </select>
          <Button intent="outline" onClick={() => void mutate()}>
            Refresh
          </Button>
        </div>
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
                    {item.userId ? ` • user ${item.userId}` : ""} • priority {item.priority}
                  </p>
                </div>
                <div className="flex gap-2">
                  <Button intent="outline" onClick={() => editItem(item)}>
                    Edit
                  </Button>
                  <Button intent="outline" onClick={() => void disableItem(item)}>
                    Disable
                  </Button>
                </div>
              </div>
              <p className="text-sm mt-2">{item.body}</p>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
