"use client";

import { Plus, RotateCw, Settings2 } from "lucide-react";
import useSWR from "swr";
import { createWallet, getWallets, updateWallet } from "@/api/admin";
import { Button } from "@/components/ui/button";
import { ButtonGroup } from "@/components/ui/button-group";
import {
  CardAction,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Sheet,
  SheetBody,
  SheetClose,
  SheetContent,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Loader } from "@/components/ui/loader";
import {
  Table,
  TableBody,
  TableCell,
  TableColumn,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { safeString } from "@/lib/utils";
import type { Wallet } from "@/types";
import { WalletForm } from "./WalletForm";

export function WalletsTable() {
  const { data, isLoading, error, mutate } = useSWR("admin:wallets", getWallets);

  const handleCreate = async (payload: {
    name: string;
    address: string;
    network: string;
    currency: string;
    custom: boolean;
    active: boolean;
  }) => {
    await createWallet(payload);
    await mutate();
  };

  const handleUpdate = async (
    id: number,
    payload: {
      name: string;
      address: string;
      network: string;
      currency: string;
      custom: boolean;
      active: boolean;
    },
  ) => {
    await updateWallet(id, payload);
    await mutate();
  };

  if (!data) {
    return (
      <div className="flex items-center justify-center w-full">
        <Loader variant="spin" />
      </div>
    );
  }

  return (
    <div className="rounded-lg border p-4">
      <CardHeader>
        <CardTitle>Wallets</CardTitle>
        <CardDescription>Manage deposit wallets and availability.</CardDescription>
        <CardAction>
          <ButtonGroup>
            <Sheet>
              <Button intent="primary">
                <Plus className="size-3" />
                New wallet
              </Button>
              <SheetContent>
                <SheetHeader>
                  <SheetTitle>Create wallet</SheetTitle>
                </SheetHeader>
                <SheetBody>
                  <WalletForm submitLabel="Create" onSubmit={handleCreate} />
                </SheetBody>
                <SheetFooter>
                  <SheetClose>Close</SheetClose>
                </SheetFooter>
              </SheetContent>
            </Sheet>
            <Button intent="outline" onClick={() => mutate()}>
              <RotateCw className="size-3" />
              Refresh
            </Button>
          </ButtonGroup>
        </CardAction>
      </CardHeader>

      <Table allowResize className="mt-4" aria-label="Wallets">
        <TableHeader>
          <TableColumn isRowHeader className="min-w-16">
            ID
          </TableColumn>
          <TableColumn>Name</TableColumn>
          <TableColumn>Currency</TableColumn>
          <TableColumn>Network</TableColumn>
          <TableColumn isResizable>Address</TableColumn>
          <TableColumn>Active</TableColumn>
          <TableColumn>Custom</TableColumn>
          <TableColumn>Actions</TableColumn>
        </TableHeader>
        <TableBody items={data}>
          {(item: Wallet) => (
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
              <TableCell>{item.active ? "Yes" : "No"}</TableCell>
              <TableCell>{item.custom ? "Yes" : "No"}</TableCell>
              <TableCell>
                <Sheet>
                  <Button intent="outline" size="sm">
                    <Settings2 className="size-3" />
                    Edit
                  </Button>
                  <SheetContent>
                    <SheetHeader>
                      <SheetTitle>Edit wallet</SheetTitle>
                    </SheetHeader>
                    <SheetBody>
                      <WalletForm
                        initial={item}
                        submitLabel="Save"
                        onSubmit={(payload) => handleUpdate(item.id, payload)}
                      />
                    </SheetBody>
                    <SheetFooter>
                      <SheetClose>Close</SheetClose>
                    </SheetFooter>
                  </SheetContent>
                </Sheet>
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>

      {isLoading && (
        <div className="flex items-center justify-center w-full">
          <Loader variant="spin" />
        </div>
      )}
      {error && (
        <div className="flex items-center justify-center w-full">
          <p className="text-red-500">Error: {error.message}</p>
        </div>
      )}
    </div>
  );
}
