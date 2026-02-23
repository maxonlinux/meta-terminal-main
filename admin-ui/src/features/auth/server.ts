import type { NextResponse } from "next/server";

type ErrorResponse = { error?: string };

export function readErrorMessage(data: unknown, fallback: string): string {
  if (!data || typeof data !== "object") {
    return fallback;
  }
  const maybe = data as ErrorResponse;
  return typeof maybe.error === "string" ? maybe.error : fallback;
}

export function parseCookie(setCookie: string) {
  const parts = setCookie.split(";").map((part) => part.trim());
  const [nameValue, ...attrs] = parts;
  const [name, ...valueParts] = nameValue.split("=");
  const value = valueParts.join("=");

  const options: {
    path?: string;
    maxAge?: number;
    expires?: Date;
    httpOnly?: boolean;
    secure?: boolean;
    sameSite?: "lax" | "strict" | "none";
  } = {};

  for (const attr of attrs) {
    const [rawKey, ...rawValue] = attr.split("=");
    const key = rawKey.toLowerCase();
    const valuePart = rawValue.join("=");

    if (key === "path") options.path = valuePart || "/";
    if (key === "max-age") options.maxAge = Number(valuePart);
    if (key === "expires") options.expires = new Date(valuePart);
    if (key === "httponly") options.httpOnly = true;
    if (key === "secure") options.secure = true;
    if (key === "samesite") {
      const normalized = valuePart.toLowerCase();
      if (
        normalized === "lax" ||
        normalized === "strict" ||
        normalized === "none"
      ) {
        options.sameSite = normalized;
      }
    }
  }

  return { name, value, options };
}

export function getCoreUrl() {
  const coreUrl = process.env.CORE_URL;
  if (!coreUrl) {
    throw new Error("CORE_URL is not configured");
  }
  return coreUrl.replace(/\/$/, "");
}

export function setResponseCookie(
  response: NextResponse,
  setCookie: string,
) {
  const cookie = parseCookie(setCookie);
  response.cookies.set(cookie.name, cookie.value, cookie.options);
}
