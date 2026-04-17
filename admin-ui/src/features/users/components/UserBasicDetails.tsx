"use client";

import { RotateCw } from "lucide-react";
import { useState } from "react";
import useSWR from "swr";
import { getUser, getUserActiveOtp, setUserActive } from "@/api/admin";
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
import { Loader } from "@/components/ui/loader";
import { safeString } from "@/lib/utils";
import { User } from "@/types";
import { EditUserBasicDetails } from "./EditUserBasicDetails";

export function UserBasicDetails({ id }: { id: string }) {
  const [togglingActive, setTogglingActive] = useState(false);
  const { data, isLoading, isValidating, error, mutate } = useSWR(
    ["admin:user", id],
    () => getUser(id),
  );
  const { data: activeOtp, mutate: mutateOtp } = useSWR(["admin:user-otp", id], () =>
    getUserActiveOtp(id),
  );

  const handleToggleActive = async () => {
    if (!data) return;
    setTogglingActive(true);
    try {
      await setUserActive(id, !data.isActive);
      await mutate();
    } finally {
      setTogglingActive(false);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>User details</CardTitle>
        <CardDescription>
          The product details card is a great way to display information about a
          product.
        </CardDescription>
        <CardAction>
          <ButtonGroup>
            <Button
              intent="outline"
              onClick={() => Promise.all([mutate(), mutateOtp()])}
            >
              {isValidating ? (
                <Loader variant="spin" />
              ) : (
                <RotateCw data-slot="icon" />
              )}
              Refresh
            </Button>
            {data && (
              <Button
                intent={data.isActive ? "outline" : "primary"}
                isDisabled={togglingActive}
                onPress={handleToggleActive}
              >
                {togglingActive
                  ? "Saving..."
                  : data.isActive
                    ? "Deactivate"
                    : "Activate"}
              </Button>
            )}
            {data && (
              <EditUserBasicDetails
                user={data}
                onSaved={() => Promise.all([mutate(), mutateOtp()]).then(() => undefined)}
              />
            )}
          </ButtonGroup>
        </CardAction>
      </CardHeader>
      <CardContent>
        {data && (
          <DescriptionList>
            <DescriptionTerm>Name</DescriptionTerm>
            <DescriptionDetails>
              {safeString(data.name)} {safeString(data.surname)}
            </DescriptionDetails>
            <DescriptionTerm>E-mail</DescriptionTerm>
            <DescriptionDetails>{data.email}</DescriptionDetails>
            <DescriptionTerm>Username</DescriptionTerm>
            <DescriptionDetails>{data.username}</DescriptionDetails>
            <DescriptionTerm>Phone</DescriptionTerm>
            <DescriptionDetails>{data.phone}</DescriptionDetails>
            <DescriptionTerm>Active</DescriptionTerm>
            <DescriptionDetails>
              {data.isActive ? "Yes" : "No"}
            </DescriptionDetails>
            <DescriptionTerm>Last login</DescriptionTerm>
            <DescriptionDetails>
              {data.lastLogin > 0
                ? new Date(data.lastLogin).toLocaleString()
                : ""}
            </DescriptionDetails>
            <DescriptionTerm>Active OTP code</DescriptionTerm>
            <DescriptionDetails>{activeOtp?.code || ""}</DescriptionDetails>
            <DescriptionTerm>OTP expires at</DescriptionTerm>
            <DescriptionDetails>
              {activeOtp?.expiresAt
                ? new Date(activeOtp.expiresAt).toLocaleString()
                : ""}
            </DescriptionDetails>
          </DescriptionList>
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
