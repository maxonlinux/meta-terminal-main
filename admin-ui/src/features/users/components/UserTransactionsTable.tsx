"use client";

import { Plus, RotateCw } from "lucide-react";
import { useState } from "react";
import useSWR from "swr";
import { toast } from "sonner";
import {
  createUserTransaction,
  getUserTransactions,
  type UserTransactionPayload,
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
import { Label } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
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
import {
  Table,
  TableBody,
  TableCell,
  TableColumn,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { TextField } from "@/components/ui/text-field";
import { safeString } from "@/lib/utils";

const typeOptions = [
  { id: "DEPOSIT", title: "DEPOSIT" },
  { id: "WITHDRAWAL", title: "WITHDRAWAL" },
];

type FormState = UserTransactionPayload;

const initialForm: FormState = {
  type: "DEPOSIT",
  asset: "",
  amount: "",
  destination: "",
  message: "",
};

export function UserTransactionsTable({ id }: { id: string }) {
  const { data, isLoading, error, mutate } = useSWR(
    ["admin:user:transactions", id],
    () => getUserTransactions(id),
  );
  const [form, setForm] = useState<FormState>(initialForm);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const submit = async () => {
    if (!form.asset.trim() || !form.amount.trim() || !form.destination.trim()) {
      toast.error("Asset, amount and destination are required");
      return;
    }

    try {
      setIsSubmitting(true);
      await createUserTransaction(id, {
        type: form.type,
        asset: form.asset.trim(),
        amount: form.amount.trim(),
        destination: form.destination.trim(),
        message: form.message?.trim() || undefined,
      });
      toast.success("Transaction created");
      setForm(initialForm);
      await mutate();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to create transaction");
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>User Transactions</CardTitle>
        <CardDescription>Create and review transactions for this user.</CardDescription>
        <CardAction>
          <ButtonGroup>
            <Sheet>
              <Button intent="primary">
                <Plus data-slot="icon" />
                New transaction
              </Button>
              <SheetContent>
                <SheetHeader>
                  <SheetTitle>Create transaction</SheetTitle>
                </SheetHeader>
                <SheetBody className="space-y-4">
                  <Select
                    value={form.type}
                    onChange={(value) =>
                      setForm((prev) => ({
                        ...prev,
                        type: (value as FormState["type"] | null) ?? "DEPOSIT",
                      }))
                    }
                    placeholder="Select type"
                  >
                    <Label>Type</Label>
                    <SelectTrigger />
                    <SelectContent items={typeOptions}>
                      {(item) => (
                        <SelectItem id={item.id} textValue={item.title}>
                          {item.title}
                        </SelectItem>
                      )}
                    </SelectContent>
                  </Select>

                  <div className="grid gap-4 sm:grid-cols-2">
                    <TextField>
                      <Label>Asset</Label>
                      <Input
                        value={form.asset}
                        onChange={(event) =>
                          setForm((prev) => ({ ...prev, asset: event.target.value }))
                        }
                        placeholder="USDT"
                        required
                      />
                    </TextField>
                    <TextField>
                      <Label>Amount</Label>
                      <Input
                        value={form.amount}
                        onChange={(event) =>
                          setForm((prev) => ({ ...prev, amount: event.target.value }))
                        }
                        placeholder="100"
                        required
                      />
                    </TextField>
                  </div>

                  <TextField>
                    <Label>Destination</Label>
                    <Input
                      value={form.destination}
                      onChange={(event) =>
                        setForm((prev) => ({ ...prev, destination: event.target.value }))
                      }
                      placeholder="Wallet address or reference"
                      required
                    />
                  </TextField>

                  <TextField>
                    <Label>Message</Label>
                    <Input
                      value={form.message}
                      onChange={(event) =>
                        setForm((prev) => ({ ...prev, message: event.target.value }))
                      }
                      placeholder="Optional note"
                    />
                  </TextField>
                </SheetBody>
                <SheetFooter>
                  <Button intent="primary" isDisabled={isSubmitting} onPress={submit}>
                    {isSubmitting ? "Saving..." : "Create"}
                  </Button>
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

      <CardContent>
        {data ? (
          <Table allowResize className="mt-2" aria-label="User transactions">
            <TableHeader>
              <TableColumn isResizable className="min-w-16">
                ID
              </TableColumn>
              <TableColumn>Type</TableColumn>
              <TableColumn>Amount</TableColumn>
              <TableColumn>Status</TableColumn>
              <TableColumn>Creator</TableColumn>
              <TableColumn isResizable>Destination</TableColumn>
              <TableColumn isResizable>Message</TableColumn>
            </TableHeader>
            <TableBody items={data}>
              {(item) => (
                <TableRow id={item.id} key={item.id}>
                  <TableCell>{item.id}</TableCell>
                  <TableCell textValue={safeString(item.type)}>{safeString(item.type)}</TableCell>
                  <TableCell textValue={safeString(item.amount)}>{safeString(item.amount)}</TableCell>
                  <TableCell textValue={safeString(item.status)}>{safeString(item.status)}</TableCell>
                  <TableCell textValue={safeString(item.createdBy)}>{safeString(item.createdBy)}</TableCell>
                  <TableCell textValue={safeString(item.destination)}>{safeString(item.destination)}</TableCell>
                  <TableCell textValue={safeString(item.message)}>{safeString(item.message)}</TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        ) : null}

        {isLoading ? (
          <div className="flex items-center justify-center py-6">
            <Loader variant="spin" />
          </div>
        ) : null}

        {error ? (
          <div className="flex items-center justify-center py-4">
            <p className="text-danger">{error.message}</p>
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}
