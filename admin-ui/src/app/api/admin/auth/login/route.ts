import { NextResponse } from "next/server";
import { getCoreUrl, readErrorMessage, setResponseCookie } from "@/features/auth/server";

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

  const res = await fetch(`${getCoreUrl()}/api/v1/admin/auth/login`, {
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
      { error: readErrorMessage(body, "LOGIN_FAILED") },
      { status: res.status },
    );
  }

  const setCookie = res.headers.get("set-cookie");
  if (!setCookie) {
    return NextResponse.json({ error: "No set-cookie header" }, { status: 500 });
  }

  const response = NextResponse.json({ success: true });
  setResponseCookie(response, setCookie);
  return response;
}
