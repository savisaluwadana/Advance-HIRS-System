import { cookies } from "next/headers";
import { NextResponse } from "next/server";
import { apiBaseURL, SESSION_COOKIE } from "../../../../lib/api";

export async function POST(request: Request) {
  let payload: unknown;
  try {
    payload = await request.json();
  } catch {
    return NextResponse.json({error: {code: "invalid_request", message: "Invalid JSON body"}}, {status: 400});
  }

  const upstream = await fetch(`${apiBaseURL()}/api/v1/auth/login`, {
    method: "POST",
    headers: {"Content-Type": "application/json"},
    body: JSON.stringify(payload),
    cache: "no-store",
  });
  const body = await upstream.json().catch(() => ({error: {code: "upstream_error", message: "Authentication service returned an invalid response"}}));
  if (!upstream.ok) {
    return NextResponse.json(body, {status: upstream.status});
  }

  const token = typeof body.access_token === "string" ? body.access_token : "";
  if (!token) {
    return NextResponse.json({error: {code: "session_error", message: "Authentication service returned no access token"}}, {status: 502});
  }

  const cookieStore = await cookies();
  const expires = body.expires_at ? new Date(body.expires_at) : new Date(Date.now() + 8 * 60 * 60 * 1000);
  cookieStore.set(SESSION_COOKIE, token, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    expires,
  });

  return NextResponse.json({authenticated: true, expires_at: body.expires_at, user: body.user});
}
