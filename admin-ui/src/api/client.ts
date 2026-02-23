type JsonResult<T> = {
  res: Response;
  body: T | null;
};

export const API_BASE = "/proxy/main";

export async function requestJson<T>(
  url: string,
  init?: RequestInit,
): Promise<JsonResult<T>> {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), 12_000);

  let res: Response;
  try {
    res = await fetch(url, {
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        ...(init?.headers ?? {}),
      },
      signal: controller.signal,
      ...init,
    });
  } catch (err) {
    if (err instanceof Error && err.name === "AbortError") {
      throw new Error("Request timed out");
    }
    throw err;
  } finally {
    clearTimeout(timeoutId);
  }

  let body: T | null = null;
  try {
    body = (await res.json()) as T;
  } catch {
    body = null;
  }

  return { res, body };
}

export async function getJson<T>(url: string): Promise<T> {
  const { res, body } = await requestJson<T>(url);
  if (!res.ok) {
    const message =
      body && typeof (body as { error?: string }).error === "string"
        ? (body as { error?: string }).error
        : `Request failed: ${res.status}`;
    throw new Error(message);
  }
  return body as T;
}

export async function patchJson<T>(url: string, data?: unknown): Promise<T> {
  const { res, body } = await requestJson<T>(url, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: data ? JSON.stringify(data) : undefined,
  });
  if (!res.ok) {
    const message =
      body && typeof (body as { error?: string }).error === "string"
        ? (body as { error?: string }).error
        : `Request failed: ${res.status}`;
    throw new Error(message);
  }
  return body as T;
}

export async function postJson<T>(url: string, data?: unknown): Promise<T> {
  const { res, body } = await requestJson<T>(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: data ? JSON.stringify(data) : undefined,
  });
  if (!res.ok) {
    const message =
      body && typeof (body as { error?: string }).error === "string"
        ? (body as { error?: string }).error
        : `Request failed: ${res.status}`;
    throw new Error(message);
  }
  return body as T;
}
