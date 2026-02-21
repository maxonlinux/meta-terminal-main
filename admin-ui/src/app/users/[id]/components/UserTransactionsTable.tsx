"use client";

import { use, useMemo } from "react";
import {
  Autocomplete,
  AutocompleteStateContext,
  useFilter,
} from "react-aria-components";
import {
  CardAction,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { SearchField, SearchInput } from "@/components/ui/search-field";
import {
  Table,
  TableBody,
  TableCell,
  TableColumn,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { RotateCw } from "lucide-react";
import useSWR from "swr";
import axios from "axios";
import { Loader } from "@/components/ui/loader";
import { UserTransaction } from "../../../../types";
import { safeString } from "@/lib/utils";

export function UserTransactionsTable({ id }: { id: number }) {
  const { data, isLoading, error } = useSWR(
    `/api/proxy/main/admin/users/${id}/transactions`,
    async (url) => {
      const { data } = await axios.get<UserTransaction[]>(url);
      return data;
    }
  );

  return (
    <div className="rounded-lg border p-4">
      <CardHeader>
        <CardTitle>Users</CardTitle>
        <CardDescription>A list of users.</CardDescription>
        <CardAction>
          <Button intent="outline">
            <RotateCw className="size-3" />
            Refresh
          </Button>
        </CardAction>
      </CardHeader>
      {data && (
        <Table allowResize className="mt-4" aria-label="Users">
          <TableHeader>
            <TableColumn isResizable className="w-0">
              #
            </TableColumn>
            <TableColumn isResizable isRowHeader>
              Type
            </TableColumn>
            <TableColumn isResizable isRowHeader>
              Amount
            </TableColumn>
            <TableColumn isResizable>Status</TableColumn>
            <TableColumn isResizable>Creator</TableColumn>
            <TableColumn isResizable>Destination</TableColumn>
            <TableColumn>Message</TableColumn>
          </TableHeader>
          <TableBody items={data}>
            {(item) => (
              <TableRow id={item.id}>
                <TableCell>{item.id}</TableCell>
                <TableCell textValue={safeString(item.type)}>
                  {safeString(item.type)}
                </TableCell>
                <TableCell textValue={safeString(item.amount)}>
                  {safeString(item.amount)}
                </TableCell>
                <TableCell textValue={safeString(item.status)}>
                  {safeString(item.status)}
                </TableCell>
                <TableCell textValue={safeString(item.createdBy)}>
                  {safeString(item.createdBy)}
                </TableCell>
                <TableCell textValue={safeString(item.destination)}>
                  {safeString(item.destination)}
                </TableCell>
                <TableCell textValue={safeString(item.message)}>
                  {safeString(item.message)}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      )}
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
