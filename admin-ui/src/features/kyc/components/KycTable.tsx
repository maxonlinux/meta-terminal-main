"use client";

import { Check, RotateCw, X } from "lucide-react";
import { useMemo, useState } from "react";
import useSWR from "swr";
import { toast } from "sonner";
import { getKycFileUrl, getKycRequests, updateKycRequest } from "@/api/admin";
import { Button } from "@/components/ui/button";
import { ButtonGroup } from "@/components/ui/button-group";
import {
  CardAction,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Link } from "@/components/ui/link";
import { Loader } from "@/components/ui/loader";
import { SearchField, SearchInput } from "@/components/ui/search-field";
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
import type { KycListItem } from "@/types";

const statusOptions = [
  { id: "", title: "All" },
  { id: "PENDING", title: "Pending" },
  { id: "APPROVED", title: "Approved" },
  { id: "REJECTED", title: "Rejected" },
];

export function KycTable() {
  const [status, setStatus] = useState<string | null>("");
  const [query, setQuery] = useState("");

  const swrKey = useMemo(
    () => ["admin:kyc", status ?? "", query],
    [status, query],
  );

  const { data, isLoading, error, mutate } = useSWR(swrKey, () =>
    getKycRequests({ status: status || undefined, q: query || undefined }),
  );

  const handleApprove = async (id: string) => {
    try {
      await updateKycRequest(id, { status: "APPROVED" });
      await mutate();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to approve");
      await mutate();
    }
  };

  const handleReject = async (id: string) => {
    const reason = window.prompt("Reject reason");
    if (!reason) return;
    try {
      await updateKycRequest(id, { status: "REJECTED", rejectReason: reason });
      await mutate();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to reject");
      await mutate();
    }
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
        <CardTitle>KYC</CardTitle>
        <CardDescription>Review KYC requests and files.</CardDescription>
        <CardAction>
          <Button intent="outline" onClick={() => mutate()}>
            <RotateCw className="size-3" />
            Refresh
          </Button>
        </CardAction>
      </CardHeader>

      <div className="mt-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <Select value={status} onChange={setStatus} placeholder="All statuses">
          <SelectTrigger className="w-full md:w-56" />
          <SelectContent items={statusOptions}>
            {(item) => (
              <SelectItem id={item.id} textValue={item.title}>
                {item.title}
              </SelectItem>
            )}
          </SelectContent>
        </Select>
        <SearchField aria-label="Search" value={query} onChange={setQuery}>
          <SearchInput placeholder="Search by doc type or country" />
        </SearchField>
      </div>

      <Table allowResize className="mt-4" aria-label="KYC requests">
        <TableHeader>
          <TableColumn isRowHeader className="min-w-16">
            ID
          </TableColumn>
          <TableColumn>User</TableColumn>
          <TableColumn>Doc type</TableColumn>
          <TableColumn>Country</TableColumn>
          <TableColumn>Status</TableColumn>
          <TableColumn>Files</TableColumn>
          <TableColumn>Actions</TableColumn>
        </TableHeader>
        <TableBody items={data}>
          {(item: KycListItem) => (
            <TableRow id={item.id} key={item.id}>
              <TableCell>{item.id}</TableCell>
              <TableCell textValue={safeString(item.user.username)}>
                <Link href={`/users/${item.userId}`}>{item.user.username}</Link>
              </TableCell>
              <TableCell textValue={safeString(item.docType)}>
                {safeString(item.docType)}
              </TableCell>
              <TableCell textValue={safeString(item.country)}>
                {safeString(item.country)}
              </TableCell>
              <TableCell textValue={safeString(item.status)}>
                {safeString(item.status)}
              </TableCell>
              <TableCell>
                <div className="flex flex-col gap-1">
                  {item.files.map((file) => (
                    <Link
                      key={file.id}
                      href={getKycFileUrl(item.id, file.id)}
                      target="_blank"
                    >
                      {file.kind}: {file.filename}
                    </Link>
                  ))}
                </div>
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
                      onClick={() => handleReject(item.id)}
                    >
                      <X data-slot="icon" />
                    </Button>
                  </ButtonGroup>
                )}
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
