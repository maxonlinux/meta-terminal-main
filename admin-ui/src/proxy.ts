import { type NextRequest, NextResponse } from "next/server";
import { getEnv } from "../env.config";

const env = await getEnv();

const ADMIN_COOKIE_NAME = "admin_token";

async function checkSetup() {
  const res = await fetch(`${env.CORE_URL}/api/v1/admin/auth/status`, {
    cache: "no-store",
  });

  if (!res.ok) {
    return true;
  }

  const data = (await res.json()) as { initialized?: boolean } | null;
  return data?.initialized !== false;
}

function checkToken(request: NextRequest) {
  const token = request.cookies.get(ADMIN_COOKIE_NAME);
  return Boolean(token?.value);
}

export async function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;

  if (pathname === "/setup") {
    if (checkToken(request)) {
      return NextResponse.redirect(new URL(`/`, request.url));
    }

    const init = await checkSetup();
    if (init) {
      return NextResponse.redirect(new URL(`/login`, request.url));
    }

    return NextResponse.next();
  }

  if (pathname === "/login") {
    if (checkToken(request)) {
      return NextResponse.redirect(new URL(`/`, request.url));
    }

    const init = await checkSetup();
    if (init) {
      return NextResponse.next();
    }

    return NextResponse.redirect(new URL(`/setup`, request.url));
  }

  if (checkToken(request)) {
    return NextResponse.next();
  }

  const init = await checkSetup();
  if (init) {
    return NextResponse.redirect(new URL(`/login`, request.url));
  }

  return NextResponse.redirect(new URL(`/setup`, request.url));
}

export const config = {
  matcher: ["/((?!api|trpc|_next|_vercel|.*\\..*).*)"],
};
