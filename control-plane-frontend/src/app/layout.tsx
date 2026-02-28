import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Saki Control Plane",
  description: "Frontend dashboard for app deployment and runtime operations.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
