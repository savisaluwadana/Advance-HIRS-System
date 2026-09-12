import AcceptClient from "./AcceptClient";

export const metadata = {
  title: "Accept invitation · Advance HRIS",
};

type Props = {searchParams: Promise<{token?: string | string[]}>};

export default async function AcceptPage({searchParams}: Props) {
  const params = await searchParams;
  const raw = Array.isArray(params.token) ? params.token[0] : params.token;
  return <AcceptClient token={raw || ""} />;
}
