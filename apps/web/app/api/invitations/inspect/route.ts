import { NextResponse } from "next/server";
import { apiBaseURL } from "../../../../lib/api";

export async function POST(request: Request) {
  let payload: unknown;
  try {
    payload = await request.json();
  } catch {
    return NextResponse.json({error: {code: "invalid_request", message: "Invalid JSON body"}}, {status: 400});
  }

  const upstream = await fetch(`${apiBaseURL()}/api/v1/access/invitations/inspect`, {
    method: "POST",
    headers: {"Content-Type": "application/json"},
    body: JSON.stringify(payload),
    cache: "no-store",
  });
  const body = await upstream.json().catch(() => ({error: {code: "upstream_error", message: "Invitation service returned an invalid response"}}));
  return NextResponse.json(body, {status: upstream.status, headers: {"Cache-Control": "no-store"}});
}
