"use client";

import { Button } from "@/components/ui/button";
import {
  DescriptionDetails,
  DescriptionList,
  DescriptionTerm,
} from "@/components/ui/description-list";
import useSWR from "swr";
import axios from "axios";
import { Loader } from "@/components/ui/loader";
import { ButtonGroup } from "@/components/ui/button-group";
import { UserAddress } from "../../../../types";
import { RotateCw } from "lucide-react";
import { EditUserAddressDetails } from "./EditUserAddressDetails";
import { safeString } from "@/lib/utils";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardAction,
} from "@/components/ui/card";

export function UserAddressDetails({ id }: { id: number }) {
  const { data, isLoading, isValidating, error, mutate } = useSWR(
    `/api/proxy/main/admin/users/${id}/address`,
    async (url) => {
      const { data } = await axios.get<UserAddress>(url);
      return data;
    }
  );

  return (
    <Card>
      <CardHeader>
        <CardTitle>User address</CardTitle>
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
            <EditUserAddressDetails />
          </ButtonGroup>
        </CardAction>
      </CardHeader>
      <CardContent>
        {data && (
          <DescriptionList>
            <DescriptionTerm>Country</DescriptionTerm>
            <DescriptionDetails>{safeString(data.country)}</DescriptionDetails>
            <DescriptionTerm>City</DescriptionTerm>
            <DescriptionDetails>{safeString(data.city)}</DescriptionDetails>
            <DescriptionTerm>Address</DescriptionTerm>
            <DescriptionDetails>{safeString(data.address)}</DescriptionDetails>
            <DescriptionTerm>Zip</DescriptionTerm>
            <DescriptionDetails>{safeString(data.zip)}</DescriptionDetails>
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
