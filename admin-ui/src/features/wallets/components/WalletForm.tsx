"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { TextField } from "@/components/ui/text-field";
import type { WalletPayload } from "@/api/admin";

type WalletFormProps = {
  initial?: WalletPayload;
  onSubmit: (payload: WalletPayload) => Promise<void>;
  submitLabel: string;
};

export function WalletForm({ initial, onSubmit, submitLabel }: WalletFormProps) {
  const [name, setName] = useState(initial?.name ?? "");
  const [address, setAddress] = useState(initial?.address ?? "");
  const [network, setNetwork] = useState(initial?.network ?? "");
  const [currency, setCurrency] = useState(initial?.currency ?? "");
  const [custom, setCustom] = useState(initial?.custom ?? false);
  const [active, setActive] = useState(initial?.active ?? true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSaving(true);
    setError(null);

    try {
      await onSubmit({
        name,
        address,
        network,
        currency,
        custom,
        active,
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : "SAVE_FAILED");
    } finally {
      setSaving(false);
    }
  };

  return (
    <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
      <div className="grid gap-4 md:grid-cols-2">
        <TextField>
          <Label>Name</Label>
          <Input
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder="USDT ERC20"
            required
          />
        </TextField>
        <TextField>
          <Label>Currency</Label>
          <Input
            value={currency}
            onChange={(event) => setCurrency(event.target.value)}
            placeholder="USDT"
            required
          />
        </TextField>
        <TextField>
          <Label>Network</Label>
          <Input
            value={network}
            onChange={(event) => setNetwork(event.target.value)}
            placeholder="ERC20"
            required
          />
        </TextField>
        <TextField>
          <Label>Address</Label>
          <Input
            value={address}
            onChange={(event) => setAddress(event.target.value)}
            placeholder="0x..."
            required
          />
        </TextField>
      </div>
      <div className="flex items-center gap-6">
        <Switch isSelected={active} onChange={setActive}>
          Active
        </Switch>
        <Switch isSelected={custom} onChange={setCustom}>
          Custom
        </Switch>
      </div>
      {error && <p className="text-sm text-red-500">{error}</p>}
      <div className="flex justify-end">
        <Button type="submit" intent="primary" disabled={saving}>
          {saving ? "Saving..." : submitLabel}
        </Button>
      </div>
    </form>
  );
}
