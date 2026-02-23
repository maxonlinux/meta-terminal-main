"use client";

import { Edit3 } from "lucide-react";
import { useState } from "react";
import { updateUserAddress } from "@/api/admin";
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
import { TextField } from "@/components/ui/text-field";
import type { UserAddress } from "@/types";

export function EditUserAddressDetails({
  userId,
  address,
  onSaved,
}: {
  userId: number;
  address: UserAddress;
  onSaved: () => Promise<void>;
}) {
  const [country, setCountry] = useState(address.country ?? "");
  const [city, setCity] = useState(address.city ?? "");
  const [street, setStreet] = useState(address.address ?? "");
  const [zip, setZip] = useState(address.zip ?? "");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSave = async (close: () => void) => {
    setSaving(true);
    setError(null);
    try {
      await updateUserAddress(userId, {
        id: address.id,
        country: country || undefined,
        city: city || undefined,
        address: street || undefined,
        zip: zip || undefined,
      });
      await onSaved();
      close();
    } catch (err) {
      setError(err instanceof Error ? err.message : "SAVE_FAILED");
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
              <SheetTitle>Update User Address</SheetTitle>
              <SheetDescription>Adjust user's address here.</SheetDescription>
            </SheetHeader>
            <SheetBody className="space-y-4">
              <TextField>
                <Label>Country</Label>
                <Input
                  type="text"
                  placeholder="USA"
                  value={country}
                  onChange={(event) => setCountry(event.target.value)}
                />
              </TextField>
              <TextField>
                <Label>City</Label>
                <Input
                  type="text"
                  placeholder="New York"
                  value={city}
                  onChange={(event) => setCity(event.target.value)}
                />
              </TextField>
              <TextField>
                <Label>Address</Label>
                <Input
                  type="text"
                  placeholder="123 Main St"
                  value={street}
                  onChange={(event) => setStreet(event.target.value)}
                />
              </TextField>
              <TextField>
                <Label>Zip Code</Label>
                <Input
                  type="text"
                  placeholder="10001"
                  value={zip}
                  onChange={(event) => setZip(event.target.value)}
                />
              </TextField>
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
