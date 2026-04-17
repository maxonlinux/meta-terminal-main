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

function redirectTo(request: NextRequest, path: string) {
  const url = request.nextUrl.clone();
  url.pathname = path.startsWith("/") ? path : `/${path}`;
  url.search = "";
  return NextResponse.redirect(url);
}

export async function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;

  if (pathname === "/setup") {
    if (checkToken(request)) {
      return redirectTo(request, "/");
    }

    const init = await checkSetup();
    if (init) {
      return redirectTo(request, "/login");
    }

    return NextResponse.next();
  }

  if (pathname === "/login") {
    if (checkToken(request)) {
      return redirectTo(request, "/");
    }

    const init = await checkSetup();
    if (init) {
      return NextResponse.next();
    }

    return redirectTo(request, "/setup");
  }

  if (checkToken(request)) {
    return NextResponse.next();
  }

  const init = await checkSetup();
  if (init) {
    return redirectTo(request, "/login");
  }

  return redirectTo(request, "/setup");
}

export const config = {
  matcher: ["/((?!api|trpc|_next|_vercel|.*\\..*).*)"],
};
