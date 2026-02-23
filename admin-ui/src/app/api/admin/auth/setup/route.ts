import { NextResponse } from "next/server";
import { getCoreUrl, readErrorMessage } from "@/features/auth/server";

export async function POST(request: Request) {
  let payload: { password?: string } = {};
  try {
    payload = (await request.json()) as { password?: string };
  } catch {
    payload = {};
  }

  if (!payload.password) {
    return NextResponse.json({ error: "Password is required" }, { status: 400 });
  }

  const res = await fetch(`${getCoreUrl()}/api/v1/admin/auth/setup`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ password: payload.password }),
  });

  if (!res.ok) {
    let body: unknown = null;
    try {
      body = await res.json();
    } catch {
      body = null;
    }
    return NextResponse.json(
      { error: readErrorMessage(body, "SETUP_FAILED") },
      { status: res.status },
    );
  }

  return NextResponse.json({ success: true });
}
