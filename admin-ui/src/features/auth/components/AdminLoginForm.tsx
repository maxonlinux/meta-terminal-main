"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { buttonStyles } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Label } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { TextField } from "@/components/ui/text-field";

export function AdminLoginForm() {
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setIsSubmitting(true);
    setError(null);

    const formData = new FormData(event.currentTarget);
    const password = formData.get("password");

    const res = await fetch("api/admin/auth/login", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ password }),
    });

    if (!res.ok) {
      let body: { error?: string } | null = null;
      try {
        body = (await res.json()) as { error?: string } | null;
      } catch {
        body = null;
      }
      setError(body?.error ?? "LOGIN_FAILED");
      setIsSubmitting(false);
      return;
    }

    router.replace("/");
  };

  return (
    <div className="flex min-h-[70vh] items-center justify-center px-4">
      <Card className="w-full max-w-md">
        <CardHeader>
          <CardTitle>Admin login</CardTitle>
          <CardDescription>Use your admin password to continue.</CardDescription>
        </CardHeader>
        <CardContent>
          <form className="space-y-4" onSubmit={handleSubmit}>
            <TextField>
              <Label>Password</Label>
              <Input
                type="password"
                name="password"
                placeholder="Enter password"
                required
              />
            </TextField>
            {error && <p className="text-sm text-red-500">{error}</p>}
            <button
              type="submit"
              className={buttonStyles({ intent: "primary" })}
              disabled={isSubmitting}
            >
              {isSubmitting ? "Signing in..." : "Sign in"}
            </button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
