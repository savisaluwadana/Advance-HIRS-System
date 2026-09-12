import { cookies } from "next/headers";
import { NextResponse } from "next/server";
import { apiBaseURL, SESSION_COOKIE } from "../../../../lib/api";

type Context = {params: Promise<{path: string[]}>};

async function proxy(request: Request, context: Context) {
  const cookieStore = await cookies();
  const token = cookieStore.get(SESSION_COOKIE)?.value;
  if (!token) {
    return NextResponse.json({error: {code: "unauthorized", message: "Sign in is required"}}, {status: 401});
  }

  const {path} = await context.params;
  const incomingURL = new URL(request.url);
  const encodedPath = path.map((segment) => encodeURIComponent(segment)).join("/");
  const target = new URL(`${apiBaseURL()}/api/v1/${encodedPath}`);
  target.search = incomingURL.search;

  const headers = new Headers({Authorization: `Bearer ${token}`});
  const contentType = request.headers.get("content-type");
  if (contentType) headers.set("Content-Type", contentType);

  const method = request.method.toUpperCase();
  const body = method === "GET" || method === "HEAD" ? undefined : await request.arrayBuffer();
  const upstream = await fetch(target, {
    method,
    headers,
    body,
    cache: "no-store",
    redirect: "manual",
  });

  if (upstream.status === 401) cookieStore.delete(SESSION_COOKIE);
  const responseHeaders = new Headers();
  const upstreamType = upstream.headers.get("content-type");
  if (upstreamType) responseHeaders.set("Content-Type", upstreamType);
  responseHeaders.set("Cache-Control", "no-store");
  return new NextResponse(await upstream.arrayBuffer(), {status: upstream.status, headers: responseHeaders});
}

export async function GET(request: Request, context: Context) { return proxy(request, context); }
export async function POST(request: Request, context: Context) { return proxy(request, context); }
export async function PUT(request: Request, context: Context) { return proxy(request, context); }
export async function PATCH(request: Request, context: Context) { return proxy(request, context); }
export async function DELETE(request: Request, context: Context) { return proxy(request, context); }
