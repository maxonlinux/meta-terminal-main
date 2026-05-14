"use client";

import { Check, Plus, RotateCw, X } from "lucide-react";
import { use, useMemo, useState } from "react";
import {
  Autocomplete,
  AutocompleteStateContext,
  type Key,
  useFilter,
} from "react-aria-components";
import useSWR from "swr";
import { toast } from "sonner";
import {
  approveFunding,
  cancelFunding,
  createUserTransaction,
  getUserTransactions,
  type UserTransactionPayload,
} from "@/api/admin";
import { Button } from "@/components/ui/button";
import { ButtonGroup } from "@/components/ui/button-group";
import {
  CardAction,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Label } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Loader } from "@/components/ui/loader";
import { SearchField, SearchInput } from "@/components/ui/search-field";
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
import { Tag, TagGroup, TagList } from "@/components/ui/tag-group";
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

function AutocompleteHighlight({ children }: { children: string }) {
  const state = use(AutocompleteStateContext)!;
  const childrenStr = safeString(children);

  const index = useMemo(() => {
    return childrenStr.toLowerCase().indexOf(state.inputValue.toLowerCase());
  }, [childrenStr, state.inputValue]);

  if (index >= 0) {
    return (
      <>
        {childrenStr.slice(0, index)}
        <mark className="bg-primary text-primary-fg">
          {childrenStr.slice(index, index + state.inputValue.length)}
        </mark>
        {childrenStr.slice(index + state.inputValue.length)}
      </>
    );
  }

  return children;
}

export function UserTransactionsTable({ id }: { id: string }) {
  const { data, isLoading, error, mutate } = useSWR(
    ["admin:user:transactions", id],
    () => getUserTransactions(id),
  );

  const { contains } = useFilter({ sensitivity: "base" });
  const [selectedStatuses, setSelectedStatuses] = useState<Iterable<Key>>("all");

  const [form, setForm] = useState<FormState>(initialForm);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const statusTags = useMemo(() => {
    const unique = new Set<string>();
    for (const item of data ?? []) {
      unique.add(safeString(item.status));
    }
    return Array.from(unique)
      .sort()
      .map((name) => ({ id: name, name }));
  }, [data]);

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
      toast.success("Transaction created as PENDING");
      setForm(initialForm);
      await mutate();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to create transaction");
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleApprove = async (fundingId: string) => {
    try {
      await approveFunding(fundingId);
      toast.success(`Transaction ${fundingId} approved`);
      await mutate();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to approve");
      await mutate();
    }
  };

  const handleCancel = async (fundingId: string) => {
    try {
      await cancelFunding(fundingId);
      toast.success(`Transaction ${fundingId} cancelled`);
      await mutate();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to cancel");
      await mutate();
    }
  };

  const rows = (data ?? [])
    .filter((item) =>
      selectedStatuses === "all" ? true : new Set(selectedStatuses).has(item.status),
    )
    .map((item) => (
      <TableRow id={item.id} key={item.id}>
        <TableCell>{item.id}</TableCell>
        <TableCell textValue={safeString(item.type)}>
          <AutocompleteHighlight>{safeString(item.type)}</AutocompleteHighlight>
        </TableCell>
        <TableCell textValue={safeString(item.amount)}>
          <AutocompleteHighlight>{safeString(item.amount)}</AutocompleteHighlight>
        </TableCell>
        <TableCell textValue={safeString(item.status)}>
          <AutocompleteHighlight>{safeString(item.status)}</AutocompleteHighlight>
        </TableCell>
        <TableCell textValue={safeString(item.createdBy)}>
          <AutocompleteHighlight>{safeString(item.createdBy)}</AutocompleteHighlight>
        </TableCell>
        <TableCell textValue={safeString(item.destination)}>
          <AutocompleteHighlight>{safeString(item.destination)}</AutocompleteHighlight>
        </TableCell>
        <TableCell textValue={safeString(item.message)}>
          <AutocompleteHighlight>{safeString(item.message)}</AutocompleteHighlight>
        </TableCell>
        <TableCell>
          {item.status === "PENDING" ? (
            <ButtonGroup>
              <Button intent="outline" size="sm" onClick={() => handleApprove(item.id)}>
                <Check data-slot="icon" />
              </Button>
              <Button intent="outline" size="sm" onClick={() => handleCancel(item.id)}>
                <X data-slot="icon" />
              </Button>
            </ButtonGroup>
          ) : null}
        </TableCell>
      </TableRow>
    ));

  return (
    <div className="rounded-lg border p-4">
      <Autocomplete filter={contains}>
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

        <div className="mt-4">
          <TagGroup
            aria-label="Status selection"
            selectionMode="multiple"
            selectedKeys={selectedStatuses}
            onSelectionChange={setSelectedStatuses}
          >
            <TagList items={statusTags}>{(item) => <Tag>{item.name}</Tag>}</TagList>
          </TagGroup>
        </div>

        <div className="mt-4 flex justify-end">
          <SearchField aria-label="Search transactions">
            <SearchInput />
          </SearchField>
        </div>

        <Table allowResize className="mt-4" aria-label="User transactions">
          <TableHeader>
            <TableColumn isRowHeader className="min-w-16">
              ID
            </TableColumn>
            <TableColumn>Type</TableColumn>
            <TableColumn>Amount</TableColumn>
            <TableColumn>Status</TableColumn>
            <TableColumn>Creator</TableColumn>
            <TableColumn isResizable>Destination</TableColumn>
            <TableColumn isResizable>Message</TableColumn>
            <TableColumn>Actions</TableColumn>
          </TableHeader>
          <TableBody>{rows}</TableBody>
        </Table>

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
      </Autocomplete>
    </div>
  );
}
