"use client";

import { Button } from "@/components/ui/button";
import useSWR from "swr";
import { Loader } from "@/components/ui/loader";
import { ButtonGroup } from "@/components/ui/button-group";
import { UserPlan } from "../../../../types";
import { Check, RotateCcw, RotateCw } from "lucide-react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
} from "@/components/ui/select";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardAction,
} from "@/components/ui/card";
import { useEffect, useState } from "react";
import { api } from "@/axios/api";

const PlanSelect = ({
  selectedPlan,
  setSelectedPlan,
}: {
  selectedPlan: string | null | undefined;
  setSelectedPlan: (value: string | null) => void;
}) => {
  const { data } = useSWR(
    `/api/proxy/main/admin/existing-plans`,
    async (url) => {
      const { data } = await api.get<string[]>(url);
      return data;
    }
  );

  const plans = data?.map((plan) => ({ id: plan, title: plan })) ?? [];

  return (
    <Select
      value={selectedPlan}
      onChange={setSelectedPlan}
      placeholder="Select a plan"
    >
      <SelectTrigger />
      <SelectContent items={plans}>
        {(item) => (
          <SelectItem id={item.id} textValue={item.title}>
            {item.title}
          </SelectItem>
        )}
      </SelectContent>
    </Select>
  );
};

export function UserPlanSelector({ id }: { id: number }) {
  const [selectedPlan, setSelectedPlan] = useState<string>();

  const { data, isLoading, isValidating, error, mutate } = useSWR(
    `/api/proxy/main/admin/users/${id}/plan`,
    async (url) => {
      const { data } = await api.get<UserPlan | null>(url);
      return data;
    }
  );

  const submit = async () => {
    await api.patch(`/api/proxy/main/admin/users/${id}/plan`, {
      plan: selectedPlan,
    });

    await mutate();
  };

  const reset = async () => {
    await api.patch(`/api/proxy/main/admin/users/${id}/reset-plan`);

    await mutate();
  };

  useEffect(() => {
    if (data) {
      setSelectedPlan(data.plan);
    }
  }, [data]);

  return (
    <Card>
      <CardHeader>
        <CardTitle>User Plan</CardTitle>
        <CardDescription>Manually change user plan.</CardDescription>
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
          </ButtonGroup>
        </CardAction>
      </CardHeader>
      <CardContent>
        <div className="flex flex-col gap-4">
          {data?.isManual && (
            <div className="flex items-center gap-4">
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Plan is set manually!!
              </p>
              <Button intent="outline" onClick={reset}>
                {isValidating ? (
                  <Loader variant="spin" />
                ) : (
                  <RotateCcw data-slot="icon" />
                )}
                Reset to auto
              </Button>
            </div>
          )}

          <div className="flex items-center gap-2">
            <PlanSelect
              setSelectedPlan={setSelectedPlan}
              selectedPlan={selectedPlan}
            />
            <Button intent="outline" onClick={submit}>
              {isValidating ? (
                <Loader variant="spin" />
              ) : (
                <Check data-slot="icon" />
              )}
              Save
            </Button>
          </div>
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
