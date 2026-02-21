"use client";

import { use, useMemo, useState } from "react";
import {
  Autocomplete,
  AutocompleteStateContext,
  Key,
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
import { Check, RotateCw, X } from "lucide-react";
import useSWR, { mutate } from "swr";
import axios from "axios";
import { Loader } from "@/components/ui/loader";
import { safeString } from "@/lib/utils";
import { Transaction } from "@/types";
import { Link } from "@/components/ui/link";
import { ButtonGroup } from "@/components/ui/button-group";
import { Tag, TagGroup, TagList } from "@/components/ui/tag-group";
import { api } from "@/axios/api";

function AutocompleteHighlight({ children }: { children: string }) {
  const state = use(AutocompleteStateContext)!;
  const childrenStr = safeString(children);

  const index = useMemo(() => {
    // TODO: use a better case-insensitive matching algorithm
    return childrenStr.toLowerCase().indexOf(state.inputValue.toLowerCase());
  }, [children, state.inputValue]);

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

export function TransactionsTable() {
  const { data, isLoading, error, mutate } = useSWR(
    `/api/proxy/main/admin/funding`,
    async (url) => {
      const { data } = await axios.get<Transaction[]>(url);
      return data;
    }
  );

  const { contains } = useFilter({ sensitivity: "base" });

  const [selectedStatuses, setSelectedStatuses] =
    useState<Iterable<Key>>("all");

  const tags = useMemo(() => {
    const s = new Set<string>();
    (data ?? []).forEach((t) => s.add(safeString(t.status)));
    return Array.from(s)
      .sort()
      .map((name) => ({ id: name, name }));
  }, [data]);

  const handleApprove = async (id: number) => {
    await api.patch(`/api/proxy/main/admin/funding/${id}/approve`);
    await mutate();
  };

  const handleCancel = async (id: number) => {
    await api.patch(`/api/proxy/main/admin/funding/${id}/cancel`);
    await mutate();
  };

  if (!data) {
    return (
      <div className="flex items-center justify-center w-full">
        <Loader variant="spin" />
      </div>
    );
  }

  const rows = data
    .filter((item) =>
      selectedStatuses === "all"
        ? true
        : new Set(selectedStatuses).has(item.status)
    )
    .map((item) => (
      <TableRow id={item.id} key={item.id}>
        <TableCell>{item.id}</TableCell>
        <TableCell textValue={safeString(item.User.username)}>
          <Link href={"/users/" + item.userId}>
            <AutocompleteHighlight>
              {safeString(item.User.username)}
            </AutocompleteHighlight>
          </Link>
        </TableCell>
        <TableCell textValue={safeString(item.type)}>
          <AutocompleteHighlight>{safeString(item.type)}</AutocompleteHighlight>
        </TableCell>
        <TableCell textValue={safeString(item.amount)}>
          <AutocompleteHighlight>
            {safeString(item.amount)}
          </AutocompleteHighlight>
        </TableCell>
        <TableCell textValue={safeString(item.status)}>
          <AutocompleteHighlight>
            {safeString(item.status)}
          </AutocompleteHighlight>
        </TableCell>
        <TableCell textValue={safeString(item.destination)}>
          <AutocompleteHighlight>
            {safeString(item.destination)}
          </AutocompleteHighlight>
        </TableCell>
        <TableCell textValue={safeString(item.message)}>
          <AutocompleteHighlight>
            {safeString(item.message)}
          </AutocompleteHighlight>
        </TableCell>
        <TableCell>
          {item.status === "PENDING" && (
            <ButtonGroup>
              <Button
                intent="outline"
                size="sm"
                onClick={() => handleApprove(item.id)}
              >
                <Check data-slot="icon" />
              </Button>
              <Button
                intent="outline"
                size="sm"
                onClick={() => handleCancel(item.id)}
              >
                <X data-slot="icon" />
              </Button>
            </ButtonGroup>
          )}
        </TableCell>
      </TableRow>
    ));

  return (
    <div className="rounded-lg border p-4">
      <CardHeader>
        <CardTitle>Transactions</CardTitle>
        <CardDescription>A list of all transactions.</CardDescription>
        <CardAction>
          <Button intent="outline">
            <RotateCw className="size-3" />
            Refresh
          </Button>
        </CardAction>
      </CardHeader>

      <div className="mt-4">
        <TagGroup
          aria-label="Selection"
          selectionMode="multiple"
          selectedKeys={selectedStatuses}
          onSelectionChange={setSelectedStatuses}
        >
          <TagList items={tags}>{(item) => <Tag>{item.name}</Tag>}</TagList>
        </TagGroup>
      </div>

      <Autocomplete filter={contains}>
        <div className="flex justify-end mt-4">
          <SearchField aria-label="Search">
            <SearchInput />
          </SearchField>
        </div>

        {data && (
          <Table allowResize className="mt-4" aria-label="Users">
            <TableHeader>
              <TableColumn isRowHeader className="max-w-10">
                ID
              </TableColumn>
              <TableColumn isResizable>User</TableColumn>
              <TableColumn>Type</TableColumn>
              <TableColumn>Amount</TableColumn>
              <TableColumn>Status</TableColumn>
              <TableColumn isResizable>Destination</TableColumn>
              <TableColumn isResizable>Message</TableColumn>
              <TableColumn>Actions</TableColumn>
            </TableHeader>
            <TableBody items={data}>{rows}</TableBody>
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
      </Autocomplete>
    </div>
  );
}
