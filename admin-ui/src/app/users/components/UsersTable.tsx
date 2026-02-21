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
import { User } from "../../../types";
import { Loader } from "@/components/ui/loader";
import { safeString } from "@/lib/utils";

function AutocompleteHighlight({ children }: { children: string }) {
  const state = use(AutocompleteStateContext)!;
  const index = useMemo(() => {
    // TODO: use a better case-insensitive matching algorithm
    return children?.toLowerCase().indexOf(state.inputValue.toLowerCase());
  }, [children, state.inputValue]);

  if (index >= 0) {
    return (
      <>
        {children.slice(0, index)}
        <mark className="bg-primary text-primary-fg">
          {children.slice(index, index + state.inputValue.length)}
        </mark>
        {children.slice(index + state.inputValue.length)}
      </>
    );
  }

  return children;
}

export function UsersTable() {
  const { data, isLoading, error } = useSWR(
    `/api/proxy/main/admin/users`,
    async (url) => {
      const { data } = await axios.get<User[]>(url);
      return data;
    }
  );

  const { contains } = useFilter({
    sensitivity: "base",
  });

  return (
    <div className="rounded-lg border p-4">
      <Autocomplete filter={contains}>
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
        <div className="flex justify-end mt-4">
          <SearchField aria-label="Search">
            <SearchInput />
          </SearchField>
        </div>
        {data && (
          <Table className="mt-4" aria-label="Users">
            <TableHeader>
              <TableColumn className="w-0">#</TableColumn>
              <TableColumn isRowHeader>Name</TableColumn>
              <TableColumn>Email</TableColumn>
              <TableColumn>Username</TableColumn>
              <TableColumn>Phone</TableColumn>
              <TableColumn>Plan</TableColumn>
              <TableColumn>Active</TableColumn>
            </TableHeader>
            <TableBody items={data}>
              {(item) => (
                <TableRow
                  id={item.id}
                  className="cursor-pointer"
                  href={`/users/${item.id}`}
                >
                  <TableCell>{item.id}</TableCell>
                  <TableCell textValue={`${item.name} ${item.surname}`}>
                    <AutocompleteHighlight>{`${item.name} ${item.surname}`}</AutocompleteHighlight>
                  </TableCell>
                  <TableCell textValue={item.email}>
                    <AutocompleteHighlight>{item.email}</AutocompleteHighlight>
                  </TableCell>
                  <TableCell textValue={item.username}>
                    <AutocompleteHighlight>
                      {item.username}
                    </AutocompleteHighlight>
                  </TableCell>
                  <TableCell textValue={item.phone}>
                    <AutocompleteHighlight>{item.phone}</AutocompleteHighlight>
                  </TableCell>
                  <TableCell textValue={safeString(item.Plan?.plan)}>
                    <AutocompleteHighlight>
                      {safeString(item.Plan?.plan)}
                    </AutocompleteHighlight>
                  </TableCell>
                  <TableCell>{item.isActive ? "Yes" : "No"}</TableCell>
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
      </Autocomplete>
    </div>
  );
}
