"use client";

import { Check, RotateCw } from "lucide-react";
import { useMemo, useState } from "react";
import useSWR from "swr";
import { assignUserWallet, getUserWallets, getWallets } from "@/api/admin";
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
import { Loader } from "@/components/ui/loader";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableColumn,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { safeString } from "@/lib/utils";
import type { UserWallet, Wallet } from "@/types";

export function UserWallets({ id }: { id: string }) {
  const [selectedWallet, setSelectedWallet] = useState<string | null>(null);

  const {
    data: userWallets,
    isLoading,
    error,
    mutate,
  } = useSWR(["admin:user:wallets", id], () => getUserWallets(id));

  const { data: wallets } = useSWR("admin:wallets", getWallets);

  const walletOptions = useMemo(() => {
    return (wallets ?? []).map((wallet: Wallet) => ({
      id: String(wallet.id),
      title: `${wallet.name} (${wallet.currency})`,
    }));
  }, [wallets]);

  const handleAssign = async () => {
    if (!selectedWallet) return;
    await assignUserWallet(id, Number(selectedWallet));
    await mutate();
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Wallets</CardTitle>
        <CardDescription>Assign and review user wallets.</CardDescription>
        <CardAction>
          <ButtonGroup>
            <Button intent="outline" onClick={() => mutate()}>
              <RotateCw className="size-3" />
              Refresh
            </Button>
          </ButtonGroup>
        </CardAction>
      </CardHeader>
      <CardContent>
        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-2 md:flex-row md:items-center">
            <Select
              value={selectedWallet}
              onChange={setSelectedWallet}
              placeholder="Select wallet"
            >
              <SelectTrigger className="w-full md:w-72" />
              <SelectContent items={walletOptions}>
                {(item) => (
                  <SelectItem id={item.id} textValue={item.title}>
                    {item.title}
                  </SelectItem>
                )}
              </SelectContent>
            </Select>
            <Button intent="outline" onClick={handleAssign}>
              <Check data-slot="icon" />
              Assign
            </Button>
          </div>

          {userWallets && (
            <Table allowResize aria-label="User wallets">
              <TableHeader>
                <TableColumn isRowHeader className="min-w-16">
                  ID
                </TableColumn>
                <TableColumn>Name</TableColumn>
                <TableColumn>Currency</TableColumn>
                <TableColumn>Network</TableColumn>
                <TableColumn isResizable>Address</TableColumn>
                <TableColumn>Assigned by</TableColumn>
                <TableColumn>Active</TableColumn>
                <TableColumn>Custom</TableColumn>
              </TableHeader>
              <TableBody items={userWallets}>
                {(item: UserWallet) => (
                  <TableRow id={item.id} key={item.id}>
                    <TableCell>{item.id}</TableCell>
                    <TableCell textValue={safeString(item.name)}>
                      {safeString(item.name)}
                    </TableCell>
                    <TableCell textValue={safeString(item.currency)}>
                      {safeString(item.currency)}
                    </TableCell>
                    <TableCell textValue={safeString(item.network)}>
                      {safeString(item.network)}
                    </TableCell>
                    <TableCell textValue={safeString(item.address)}>
                      {safeString(item.address)}
                    </TableCell>
                    <TableCell textValue={safeString(item.by)}>
                      {safeString(item.by)}
                    </TableCell>
                    <TableCell>{item.active ? "Yes" : "No"}</TableCell>
                    <TableCell>{item.custom ? "Yes" : "No"}</TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          )}
        </div>

        {isLoading && (
          <div className="flex items-center justify-center w-full">
            <Loader variant="spin" />
          </div>
        )}
        {error && <div>Error: {error.message}</div>}
      </CardContent>
    </Card>
  );
}
