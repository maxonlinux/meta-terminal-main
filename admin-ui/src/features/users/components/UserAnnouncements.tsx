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
import { Input } from "@/components/ui/input";
import { Loader } from "@/components/ui/loader";

export function UserAnnouncements({ id }: { id: string }) {
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [link, setLink] = useState("");
  const [priority, setPriority] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [isCreating, setIsCreating] = useState(false);

  const { data, error: loadError, isLoading, mutate } = useSWR(
    ["admin:user-announcements", id],
    () => getAnnouncements("USER", id),
  );

  const create = async () => {
    setError(null);
    if (!title.trim() || !body.trim()) {
      setError("Title and body are required");
      return;
    }
    try {
      setIsCreating(true);
      await createAnnouncement({
        scope: "USER",
        userId: id,
        title: title.trim(),
        body: body.trim(),
        link: link.trim() || undefined,
        priority,
        isActive: true,
      });
      setTitle("");
      setBody("");
      setLink("");
      setPriority(0);
      toast.success("Announcement created");
      await mutate();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create announcement");
    } finally {
      setIsCreating(false);
    }
  };

  const disable = async (announcementId: string) => {
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
        isActive: false,
        startsAt: current.startsAt,
        endsAt: current.endsAt,
      });
      toast.success("Announcement disabled");
      await mutate();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to disable announcement");
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
        <CardDescription>Personal announcements for this user only.</CardDescription>
        <CardAction>
          <Button intent="outline" onClick={() => void mutate()}>
            Refresh
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          <Input
            placeholder="Title"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
          />
          <Input
            placeholder="Link (optional)"
            value={link}
            onChange={(e) => setLink(e.target.value)}
          />
          <Input
            placeholder="Priority"
            type="number"
            value={String(priority)}
            onChange={(e) => setPriority(Number(e.target.value) || 0)}
          />
          <textarea
            className="md:col-span-2 min-h-24 rounded-md border bg-transparent px-3 py-2"
            placeholder="Body"
            value={body}
            onChange={(e) => setBody(e.target.value)}
          />
        </div>
        <ButtonGroup>
          <Button onClick={() => void create()} disabled={isCreating}>
            {isCreating ? <Loader variant="spin" /> : null}
            Create announcement
          </Button>
        </ButtonGroup>
        {error ? <p className="text-sm text-danger">{error}</p> : null}
        {loadError ? <p className="text-sm text-danger">{loadError.message}</p> : null}

        {isLoading && <p>Loading...</p>}
        {!isLoading && (!data || data.length === 0) && <p>No user announcements.</p>}
        <div className="space-y-2">
          {data?.map((item) => (
            <div key={item.id} className="rounded-md border p-3">
              <div className="flex items-start justify-between gap-2">
                <div>
                  <p className="text-sm font-semibold">{item.title}</p>
                  <p className="text-xs opacity-70">
                    priority {item.priority} • {item.isActive ? "active" : "inactive"}
                  </p>
                </div>
                <div className="flex gap-2">
                  {item.isActive && (
                    <Button
                      intent="outline"
                      onClick={() => void disable(item.id)}
                      disabled={busyId === item.id}
                    >
                      Disable
                    </Button>
                  )}
                  <Button
                    intent="outline"
                    onClick={() => void remove(item.id)}
                    disabled={busyId === item.id}
                  >
                    Delete
                  </Button>
                </div>
              </div>
              <p className="text-sm mt-2">{item.body}</p>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
