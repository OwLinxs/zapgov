import type { Metadata } from "next";
import "./globals.css";
import "./zg.css";
import { ThemeProvider, ThemeScript } from "@/lib/theme";
import { ToastProvider } from "@/components/ui";

export const metadata: Metadata = {
  title: "ZapGov — Atendimento ao cidadão",
  description: "Painel de atendimento ao cidadão via WhatsApp Business (Meta Cloud API)",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="pt-BR" suppressHydrationWarning>
      <head>
        <ThemeScript />
      </head>
      <body>
        <ThemeProvider>
          <ToastProvider>{children}</ToastProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
