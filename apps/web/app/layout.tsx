import type { Metadata } from "next";
import "./globals.css";
import "./portal.css";

export const metadata: Metadata = {
  title: "Advance HRIS",
  description: "Modern people operations, without the operational drag.",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
