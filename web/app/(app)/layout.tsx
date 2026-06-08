import { Sidebar } from "@/components/shell/Sidebar";
import { Tweaks } from "@/components/shell/Tweaks";
import { AuthProvider, AuthGate } from "@/lib/auth";
import { NotificationsProvider } from "@/lib/notifications";

// Shell autenticado: guard de sessão + sidebar fixa + área principal rolável.
export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <AuthProvider>
      <AuthGate>
        <NotificationsProvider>
          <div className="app-shell">
            <Sidebar />
            <main className="main">{children}</main>
            <Tweaks />
          </div>
        </NotificationsProvider>
      </AuthGate>
    </AuthProvider>
  );
}
