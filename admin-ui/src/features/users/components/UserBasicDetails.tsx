"use client";

import { RotateCw } from "lucide-react";
import useSWR from "swr";
import { getUser } from "@/api/admin";
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
  const { data, isLoading, isValidating, error, mutate } = useSWR(
    ["admin:user", id],
    () => getUser(id),
  );

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
            <Button intent="outline" onClick={() => mutate()}>
              {isValidating ? (
                <Loader variant="spin" />
              ) : (
                <RotateCw data-slot="icon" />
              )}
              Refresh
            </Button>
            {data && (
              <EditUserBasicDetails user={data} onSaved={() => mutate()} />
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
