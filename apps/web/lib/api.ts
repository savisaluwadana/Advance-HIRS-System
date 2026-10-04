export const SESSION_COOKIE = "advance_hris_session";

export function apiBaseURL() {
  return (process.env.API_INTERNAL_URL || process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080").replace(/\/$/, "");
}

export function sessionCookieSecure() {
  const configured = process.env.WEB_COOKIE_SECURE?.trim().toLowerCase();
  if (configured === "true") return true;
  if (configured === "false") return false;
  return process.env.NODE_ENV === "production";
}
