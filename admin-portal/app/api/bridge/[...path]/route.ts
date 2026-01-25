import { NextRequest } from "next/server";

export const runtime = "nodejs";

function requiredEnv(name: string): string {
  const v = process.env[name];
  if (!v) {
    throw new Error(`${name} is not set`);
  }
  return v;
}

async function handler(req: NextRequest, ctx: { params: { path: string[] } }) {
  let baseUrl: string;
  let apiKey: string;
  try {
    baseUrl = requiredEnv("BRIDGE_BASE_URL").replace(/\/$/, "");
    apiKey = requiredEnv("BRIDGE_API_KEY");
  } catch (e) {
    return new Response(
      JSON.stringify({
        error: "misconfigured",
        message: e instanceof Error ? e.message : "Missing required env vars"
      }),
      { status: 500, headers: { "Content-Type": "application/json" } }
    );
  }

  const pathname = ctx.params.path.join("/");
  let url: URL;
  try {
    url = new URL(`${baseUrl}/${pathname}`);
  } catch {
    return new Response(
      JSON.stringify({
        error: "misconfigured",
        message: "BRIDGE_BASE_URL is not a valid URL"
      }),
      { status: 500, headers: { "Content-Type": "application/json" } }
    );
  }

  const search = req.nextUrl.searchParams;
  search.forEach((v, k) => url.searchParams.set(k, v));

  const headers: Record<string, string> = {};

  // Forward browser cookies (admin session).
  const cookie = req.headers.get("cookie");
  if (cookie) {
    headers.Cookie = cookie;
  }

  // Only attach API key for non-admin endpoints.
  // Otherwise, the admin portal would be implicitly authorized without login.
  if (!pathname.startsWith("api/admin/v1/")) {
    headers.Authorization = `Bearer ${apiKey}`;
  }

  const contentType = req.headers.get("content-type");
  if (contentType) {
    headers["Content-Type"] = contentType;
  }

  const method = req.method.toUpperCase();
  const body = method === "GET" || method === "HEAD" ? undefined : await req.text();

  const ac = new AbortController();
  const timeout = setTimeout(() => ac.abort(), 10_000);

  let upstream: Response;
  try {
    upstream = await fetch(url, {
      method,
      headers,
      body,
      signal: ac.signal
    });
  } catch (e) {
    const message = e instanceof Error ? e.message : "Upstream fetch failed";
    return new Response(
      JSON.stringify({
        error: "bad_gateway",
        message,
        upstream: url.toString()
      }),
      { status: 502, headers: { "Content-Type": "application/json" } }
    );
  } finally {
    clearTimeout(timeout);
  }

  const upstreamBody = await upstream.arrayBuffer();

  const outHeaders: Record<string, string> = {
    "Content-Type": upstream.headers.get("content-type") ?? "application/json"
  };

  const setCookie = upstream.headers.get("set-cookie");
  if (setCookie) {
    outHeaders["Set-Cookie"] = setCookie;
  }

  return new Response(upstreamBody, {
    status: upstream.status,
    headers: outHeaders
  });
}

export async function GET(req: NextRequest, ctx: { params: { path: string[] } }) {
  return handler(req, ctx);
}

export async function POST(req: NextRequest, ctx: { params: { path: string[] } }) {
  return handler(req, ctx);
}

export async function PUT(req: NextRequest, ctx: { params: { path: string[] } }) {
  return handler(req, ctx);
}

export async function PATCH(req: NextRequest, ctx: { params: { path: string[] } }) {
  return handler(req, ctx);
}

export async function DELETE(req: NextRequest, ctx: { params: { path: string[] } }) {
  return handler(req, ctx);
}
