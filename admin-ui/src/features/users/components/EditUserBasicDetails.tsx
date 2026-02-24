"use client";

import { Edit3 } from "lucide-react";
import { useState } from "react";
import { setUserActive, updateUserProfile } from "@/api/admin";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
  Sheet,
  SheetBody,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Switch } from "@/components/ui/switch";
import { TextField } from "@/components/ui/text-field";
import type { User } from "@/types";

export function EditUserBasicDetails({
  user,
  onSaved,
}: {
  user: User;
  onSaved: () => Promise<void>;
}) {
  const [active, setActive] = useState(user.isActive);
  const [email, setEmail] = useState(user.email ?? "");
  const [phone, setPhone] = useState(user.phone ?? "");
  const [name, setName] = useState(user.name ?? "");
  const [surname, setSurname] = useState(user.surname ?? "");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSave = async (close: () => void) => {
    setSaving(true);
    setError(null);
    try {
      await updateUserProfile(user.id, {
        email,
        phone,
        name: name || null,
        surname: surname || null,
      });
      await setUserActive(user.id, active);
      await onSaved();
      close();
    } catch (err) {
      setError(err instanceof Error ? err.message : "SAVE_FAILED");
      await onSaved();
    } finally {
      setSaving(false);
    }
  };

  return (
    <Sheet>
      <Button intent="outline">
        <Edit3 data-slot="icon" />
        Edit
      </Button>
      <SheetContent>
        {({ close }) => (
          <>
            <SheetHeader>
              <SheetTitle>Update User Details</SheetTitle>
              <SheetDescription>Manage profile details and status.</SheetDescription>
            </SheetHeader>
            <SheetBody className="space-y-4">
              <div className="grid gap-4 sm:grid-cols-2">
                <TextField>
                  <Label>First name</Label>
                  <Input
                    type="text"
                    value={name}
                    onChange={(event) => setName(event.target.value)}
                    placeholder="John"
                  />
                </TextField>
                <TextField>
                  <Label>Last name</Label>
                  <Input
                    type="text"
                    value={surname}
                    onChange={(event) => setSurname(event.target.value)}
                    placeholder="Smith"
                  />
                </TextField>
              </div>
              <TextField>
                <Label>Email</Label>
                <Input
                  type="email"
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  placeholder="name@domain.com"
                  required
                />
              </TextField>
              <TextField>
                <Label>Phone</Label>
                <Input
                  type="text"
                  value={phone}
                  onChange={(event) => setPhone(event.target.value)}
                  placeholder="+1 234 567 89"
                  required
                />
              </TextField>
              <Switch isSelected={active} onChange={setActive}>
                Active
              </Switch>
              {error && <p className="text-sm text-red-500">{error}</p>}
            </SheetBody>
            <SheetFooter>
              <Button
                intent="primary"
                type="button"
                isDisabled={saving}
                onPress={() => handleSave(close)}
              >
                {saving ? "Saving..." : "Save"}
              </Button>
              <SheetClose>Cancel</SheetClose>
            </SheetFooter>
          </>
        )}
      </SheetContent>
    </Sheet>
  );
}
