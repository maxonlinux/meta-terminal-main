"use client";

import { Check, RotateCcw, RotateCw } from "lucide-react";
import { useEffect, useState } from "react";
import useSWR from "swr";
import {
  getUser,
  getPlans,
  getUserPlan,
  resetUserPlan,
  updateUserPlan,
} from "@/api/admin";
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
import { Loader } from "@/components/ui/loader";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
} from "@/components/ui/select";

const PlanSelect = ({
  selectedPlan,
  setSelectedPlan,
}: {
  selectedPlan: string | null | undefined;
  setSelectedPlan: (value: string | null) => void;
}) => {
  const { data } = useSWR("admin:plans", getPlans);

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

  const {
    data: planData,
    isLoading,
    isValidating,
    error,
    mutate: mutatePlan,
  } = useSWR(
    ["admin:user:plan", id],
    () => getUserPlan(id),
  );
  const { data: userData, mutate: mutateUser } = useSWR(
    ["admin:user", id],
    () => getUser(id),
  );

  const submit = async () => {
    await updateUserPlan(id, selectedPlan ?? "");
    await Promise.all([mutatePlan(), mutateUser()]);
  };

  const reset = async () => {
    await resetUserPlan(id);
    setSelectedPlan(undefined);
    await Promise.all([mutatePlan(), mutateUser()]);
  };

  useEffect(() => {
    if (selectedPlan !== undefined) {
      return;
    }

    if (userData?.Plan?.plan) {
      setSelectedPlan(userData.Plan.plan);
      return;
    }

    if (typeof planData?.current === "string") {
      setSelectedPlan(planData.current);
    }
  }, [planData?.current, selectedPlan, userData?.Plan?.plan]);

  return (
    <Card>
      <CardHeader>
        <CardTitle>User Plan</CardTitle>
        <CardDescription>Manually change user plan.</CardDescription>
        <CardAction>
          <ButtonGroup>
            <Button intent="outline" onClick={() => mutatePlan()}>
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
          {userData?.Plan?.isManual && (
            <div className="flex items-center gap-4">
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Plan is set manually.
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
