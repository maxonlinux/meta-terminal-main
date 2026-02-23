"use client";

import { Check, RotateCw, X } from "lucide-react";
import useSWR from "swr";
import { getKycFileUrl, getKycRequests, updateKycRequest } from "@/api/admin";
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
import {
  DescriptionDetails,
  DescriptionList,
  DescriptionTerm,
} from "@/components/ui/description-list";
import { Link } from "@/components/ui/link";
import { Loader } from "@/components/ui/loader";
import { safeString } from "@/lib/utils";
import type { KycListItem } from "@/types";

export function UserKyc({ id }: { id: number }) {
  const { data, isLoading, error, mutate, isValidating } = useSWR(
    ["admin:user:kyc", id],
    () => getKycRequests({ userId: id }),
  );

  const item = data?.[0];

  const handleApprove = async (kycId: number) => {
    await updateKycRequest(kycId, { status: "APPROVED" });
    await mutate();
  };

  const handleReject = async (kycId: number) => {
    const reason = window.prompt("Reject reason");
    if (!reason) return;
    await updateKycRequest(kycId, { status: "REJECTED", rejectReason: reason });
    await mutate();
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>KYC</CardTitle>
        <CardDescription>Latest KYC request for this user.</CardDescription>
        <CardAction>
          <Button intent="outline" onClick={() => mutate()}>
            {isValidating ? (
              <Loader variant="spin" />
            ) : (
              <RotateCw data-slot="icon" />
            )}
            Refresh
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent>
        {item ? (
          <>
            <DescriptionList>
              <DescriptionTerm>Document</DescriptionTerm>
              <DescriptionDetails>{safeString(item.docType)}</DescriptionDetails>
              <DescriptionTerm>Country</DescriptionTerm>
              <DescriptionDetails>{safeString(item.country)}</DescriptionDetails>
              <DescriptionTerm>Status</DescriptionTerm>
              <DescriptionDetails>{safeString(item.status)}</DescriptionDetails>
              <DescriptionTerm>Reject reason</DescriptionTerm>
              <DescriptionDetails>
                {safeString(item.rejectReason)}
              </DescriptionDetails>
              <DescriptionTerm>Files</DescriptionTerm>
              <DescriptionDetails>
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
              </DescriptionDetails>
            </DescriptionList>

            {item.status === "PENDING" && (
              <div className="mt-4">
                <ButtonGroup>
                  <Button intent="outline" onClick={() => handleApprove(item.id)}>
                    <Check data-slot="icon" />
                    Approve
                  </Button>
                  <Button intent="outline" onClick={() => handleReject(item.id)}>
                    <X data-slot="icon" />
                    Reject
                  </Button>
                </ButtonGroup>
              </div>
            )}
          </>
        ) : (
          <p className="text-sm text-muted-fg">No KYC requests found.</p>
        )}

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
