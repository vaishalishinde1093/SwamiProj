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
  const baseUrl = requiredEnv("BRIDGE_BASE_URL").replace(/\/$/, "");
  const apiKey = requiredEnv("BRIDGE_API_KEY");

  const pathname = ctx.params.path.join("/");
  const url = new URL(`${baseUrl}/${pathname}`);

  const search = req.nextUrl.searchParams;
  search.forEach((v, k) => url.searchParams.set(k, v));

  const headers: Record<string, string> = {
    Authorization: `Bearer ${apiKey}`
  };

  const contentType = req.headers.get("content-type");
  if (contentType) {
    headers["Content-Type"] = contentType;
  }

  const method = req.method.toUpperCase();
  const body = method === "GET" || method === "HEAD" ? undefined : await req.text();

  const upstream = await fetch(url, {
    method,
    headers,
    body
  });

  const upstreamBody = await upstream.arrayBuffer();

  return new Response(upstreamBody, {
    status: upstream.status,
    headers: {
      "Content-Type": upstream.headers.get("content-type") ?? "application/json"
    }
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
