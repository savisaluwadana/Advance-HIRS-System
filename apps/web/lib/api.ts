export const SESSION_COOKIE = "advance_hris_session";

export function apiBaseURL() {
  return (process.env.API_INTERNAL_URL || process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080").replace(/\/$/, "");
}
