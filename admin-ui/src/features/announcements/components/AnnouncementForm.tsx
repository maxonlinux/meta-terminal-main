"use client";

import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { TextField } from "@/components/ui/text-field";

export type AnnouncementScope = "GLOBAL" | "USER";

export type AnnouncementFormState = {
  scope: AnnouncementScope;
  userId: string;
  title: string;
  body: string;
  link: string;
  priority: number;
  isActive: boolean;
};

const scopeOptions = [
  { id: "GLOBAL", title: "GLOBAL" },
  { id: "USER", title: "USER" },
];

type Props = {
  value: AnnouncementFormState;
  submitLabel: string;
  includeScope: boolean;
  isSubmitting: boolean;
  onChange: (next: AnnouncementFormState) => void;
  onSubmit: () => void;
};

export function AnnouncementForm({
  value,
  submitLabel,
  includeScope,
  isSubmitting,
  onChange,
  onSubmit,
}: Props) {
  return (
    <div className="space-y-4">
      <div className="grid gap-4 sm:grid-cols-2">
        {includeScope ? (
          <Select
            value={value.scope}
            onChange={(next) =>
              onChange({
                ...value,
                scope: (next as AnnouncementScope) ?? "GLOBAL",
              })
            }
            placeholder="Select scope"
          >
            <Label>Scope</Label>
            <SelectTrigger />
            <SelectContent items={scopeOptions}>
              {(item) => (
                <SelectItem id={item.id} textValue={item.title}>
                  {item.title}
                </SelectItem>
              )}
            </SelectContent>
          </Select>
        ) : null}

        {includeScope && value.scope === "USER" ? (
          <TextField>
            <Label>User ID</Label>
            <Input
              value={value.userId}
              onChange={(event) => onChange({ ...value, userId: event.target.value })}
              placeholder="313158322683379712"
              required
            />
          </TextField>
        ) : null}

        <TextField>
          <Label>Title</Label>
          <Input
            value={value.title}
            onChange={(event) => onChange({ ...value, title: event.target.value })}
            placeholder="System notice"
            required
          />
        </TextField>

        <TextField>
          <Label>Link</Label>
          <Input
            value={value.link}
            onChange={(event) => onChange({ ...value, link: event.target.value })}
            placeholder="https://..."
          />
        </TextField>

        <TextField>
          <Label>Priority</Label>
          <Input
            type="number"
            value={String(value.priority)}
            onChange={(event) =>
              onChange({
                ...value,
                priority: Number(event.target.value) || 0,
              })
            }
          />
        </TextField>

        <div className="pt-1">
          <Switch
            className="max-w-fit"
            isSelected={value.isActive}
            onChange={(isSelected) =>
              onChange({ ...value, isActive: Boolean(isSelected) })
            }
          >
            Active
          </Switch>
        </div>
      </div>

      <TextField>
        <Label>Body</Label>
        <textarea
          data-slot="control"
          className="w-full min-h-32 rounded-md border bg-transparent px-3 py-2 text-sm"
          value={value.body}
          onChange={(event) => onChange({ ...value, body: event.target.value })}
          placeholder="Message content"
          required
        />
      </TextField>

      <div className="flex justify-end">
        <Button type="button" intent="primary" disabled={isSubmitting} onClick={onSubmit}>
          {isSubmitting ? "Saving..." : submitLabel}
        </Button>
      </div>
    </div>
  );
}
